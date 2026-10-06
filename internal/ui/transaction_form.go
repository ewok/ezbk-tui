/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"go.uber.org/zap"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/notify"
	"ezbk-tui/internal/ui/prompt"
)

const (
	maxCommentLength = 255
	maxTags          = 10 // ezBookkeeping limit per transaction
)

type (
	RedrawFormMsg          struct{}
	ContinueTransactionMsg struct{}
	NewTransactionMsg      struct{ Transaction domain.Transaction }
	EditTransactionMsg     struct{ Transaction domain.Transaction }
	LoadTransactionMsg     struct {
		Transaction domain.Transaction
		New         bool
	}
	ResetTransactionMsg      struct{}
	TransactionSaveResultMsg struct {
		ID      string
		Err     error
		Updated bool
	}
)

// formData holds the values bound to the huh form fields.
type formData struct {
	id          string
	txType      domain.TransactionType
	sourceID    string
	destID      string
	categoryID  string
	amount      string
	destAmount  string
	tagIDs      []string
	keptTagIDs  []string // tags not offered by the form (e.g. hidden); preserved on save
	comment     string
	year        string
	month       string
	day         string
	timeOfDay   time.Duration
	originalDay string
}

type modelTransaction struct {
	form   *huh.Form
	api    TransactionFormAPI
	keymap TransactionFormKeyMap
	focus  bool

	new      bool
	created  bool
	lastDate string

	data *formData
	opts *formOptions
}

func newModelTransaction(api TransactionFormAPI) modelTransaction {
	return modelTransaction{
		api:    api,
		keymap: DefaultTransactionFormKeyMap(),
		data:   &formData{txType: domain.TxExpense},
		opts:   &formOptions{accountByID: map[string]domain.Account{}},
		form:   huh.NewForm(huh.NewGroup(huh.NewNote().Title("Loading..."))),
	}
}

func (m modelTransaction) Init() tea.Cmd { return nil }

func (m modelTransaction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case NewTransactionMsg:
		return m, m.confirmLoad(msg.Transaction, true)
	case EditTransactionMsg:
		return m, m.confirmLoad(msg.Transaction, false)
	case LoadTransactionMsg:
		m.SetTransaction(msg.Transaction, msg.New)
		m.created = true
		return m, tea.Batch(RedrawForm(), SetView(formView))
	case ContinueTransactionMsg:
		if m.created {
			return m, tea.Batch(RedrawForm(), SetView(formView))
		}
		return m, nil
	case ResetTransactionMsg:
		m.SetTransaction(domain.Transaction{}, true)
		m.created = true
		return m, RedrawForm()
	case RedrawFormMsg:
		m.UpdateForm()
		return m, tea.WindowSize()
	case TransactionSaveResultMsg:
		if msg.Err != nil {
			return m, notify.NotifyError(msg.Err.Error())
		}
		m.created = false
		action := "updated"
		if !msg.Updated {
			action = "created"
			m.lastDate = m.data.dateString()
		}
		return m, tea.Batch(
			SetView(transactionsView),
			notify.NotifyLog("Transaction "+action),
			refreshAfterWrite(msg.ID),
		)
	}

	if !m.focus {
		return m, nil
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, m.keymap.Cancel):
			if m.created {
				return m, tea.Batch(SetView(transactionsView),
					notify.NotifyLog("Form kept. Press esc in the list to continue editing."))
			}
			return m, SetView(transactionsView)
		case key.Matches(km, m.keymap.Reset):
			return m, Cmd(ResetTransactionMsg{})
		case key.Matches(km, m.keymap.EditFormAgain):
			return m, RedrawForm()
		case key.Matches(km, m.keymap.ToggleLastDate):
			return m, m.toggleLastDate()
		case key.Matches(km, m.keymap.Submit):
			if m.form.State == huh.StateCompleted {
				return m, m.saveCmd()
			}
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	return m, cmd
}

func (m modelTransaction) confirmLoad(tx domain.Transaction, isNew bool) tea.Cmd {
	load := Cmd(LoadTransactionMsg{Transaction: tx, New: isNew})
	if !m.created {
		return load
	}
	return prompt.Ask(
		"Unsaved form data will be lost. Discard? (y - yes / any key - no): ",
		"",
		func(value string) tea.Cmd {
			if value == "y" {
				return load
			}
			return SetView(transactionsView)
		},
	)
}

func (m modelTransaction) View() string {
	if m.form.State == huh.StateCompleted {
		return "Press ctrl+s to save, ctrl+e to edit the form again, ctrl+n to reset, or esc to go back."
	}
	return m.form.View()
}

func (m *modelTransaction) Focus()        { m.focus = true }
func (m *modelTransaction) Blur()         { m.focus = false }
func (m *modelTransaction) Focused() bool { return m.focus }

// SetTransaction loads a transaction (or a prefilled template) into the form.
func (m *modelTransaction) SetTransaction(tx domain.Transaction, isNew bool) {
	zap.L().Debug("load transaction into form", zap.String("id", tx.ID), zap.Bool("new", isNew))
	loc := m.api.Location()
	now := time.Now().In(loc)

	d := &formData{
		txType:     tx.Type,
		sourceID:   tx.Source.ID,
		destID:     tx.Destination.ID,
		categoryID: tx.Category.ID,
		comment:    tx.Comment,
		tagIDs:     tx.TagIDs(),
	}
	if d.txType != domain.TxIncome && d.txType != domain.TxTransfer {
		d.txType = domain.TxExpense
	}
	if tx.SourceAmount != 0 {
		d.amount = tx.SourceAmount.String()
	}
	if tx.Type == domain.TxTransfer && tx.DestinationAmount != 0 && tx.Destination.Currency != tx.Source.Currency {
		d.destAmount = tx.DestinationAmount.String()
	}

	when := now
	if !isNew && !tx.Time.IsZero() {
		when = tx.Time.In(loc)
		d.id = tx.ID
	}
	d.setDate(when)
	d.timeOfDay = timeOfDay(when)
	d.originalDay = d.dateString()

	if d.sourceID == "" {
		d.sourceID = m.api.DefaultAccountID()
	}
	m.new = isNew || d.id == ""
	m.data = d
}

func (d *formData) setDate(t time.Time) {
	d.year = strconv.Itoa(t.Year())
	d.month = fmt.Sprintf("%02d", t.Month())
	d.day = fmt.Sprintf("%02d", t.Day())
}

func (d *formData) dateString() string {
	return d.year + "-" + d.month + "-" + d.day
}

// allTagIDs returns the tags selected in the form plus the preserved ones.
func (d *formData) allTagIDs() []string {
	out := make([]string, 0, len(d.tagIDs)+len(d.keptTagIDs))
	out = append(out, d.keptTagIDs...)
	for _, id := range d.tagIDs {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// splitTagIDs moves tags that the form cannot show into keptTagIDs, so that
// the multi-select (which only reports offered options) does not drop them.
func (d *formData) splitTagIDs(offered map[string]bool) {
	var shown, kept []string
	for _, id := range d.allTagIDs() {
		if offered[id] {
			shown = append(shown, id)
		} else {
			kept = append(kept, id)
		}
	}
	d.tagIDs, d.keptTagIDs = shown, kept
}

func timeOfDay(t time.Time) time.Duration {
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second
}

func (m *modelTransaction) toggleLastDate() tea.Cmd {
	if !m.new {
		return nil
	}
	if m.lastDate == "" {
		return notify.NotifyWarn("No saved date yet")
	}
	now := time.Now().In(m.api.Location())
	if m.data.dateString() == m.lastDate {
		m.data.setDate(now)
	} else if t, err := time.Parse("2006-01-02", m.lastDate); err == nil {
		m.data.setDate(t)
	}
	return RedrawForm()
}

// buildRequest converts the form data into an API request.
func (m *modelTransaction) buildRequest() (domain.TransactionRequest, error) {
	d := m.data
	amount, err := domain.ParseMoney(d.amount)
	if err != nil {
		return domain.TransactionRequest{}, fmt.Errorf("invalid amount: %w", err)
	}
	t, err := m.transactionTime()
	if err != nil {
		return domain.TransactionRequest{}, err
	}
	req := domain.TransactionRequest{
		ID:              d.id,
		Type:            d.txType,
		Time:            t,
		CategoryID:      d.categoryID,
		SourceAccountID: d.sourceID,
		SourceAmount:    amount,
		TagIDs:          d.allTagIDs(),
		Comment:         d.comment,
	}
	if d.txType == domain.TxTransfer {
		req.DestinationAccountID = d.destID
		req.DestinationAmount = amount
		if m.needsDestAmount() {
			dest, err := domain.ParseMoney(d.destAmount)
			if err != nil {
				return domain.TransactionRequest{}, fmt.Errorf("invalid destination amount: %w", err)
			}
			req.DestinationAmount = dest
		}
	}
	return req, nil
}

func (m *modelTransaction) transactionTime() (time.Time, error) {
	d := m.data
	day, err := time.ParseInLocation("2006-01-02", d.dateString(), m.api.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %s: %w", d.dateString(), err)
	}
	clock := d.timeOfDay
	if m.new && d.dateString() != d.originalDay {
		clock = timeOfDay(time.Now().In(m.api.Location()))
	}
	return day.Add(clock), nil
}

func (m *modelTransaction) saveCmd() tea.Cmd {
	req, err := m.buildRequest()
	if err != nil {
		return notify.NotifyError(err.Error())
	}
	api, isNew := m.api, m.new
	return func() tea.Msg {
		opID := startLoading("Saving transaction...")
		defer stopLoading(opID)
		if isNew {
			id, err := api.CreateTransaction(req)
			return TransactionSaveResultMsg{ID: id, Err: err}
		}
		id, err := api.UpdateTransaction(req)
		return TransactionSaveResultMsg{ID: id, Err: err, Updated: true}
	}
}

// RedrawForm rebuilds the form from current data.
func RedrawForm() tea.Cmd {
	return Cmd(RedrawFormMsg{})
}

func (m *modelTransaction) sourceAccount() domain.Account {
	return m.opts.accountByID[m.data.sourceID]
}

func (m *modelTransaction) destAccount() domain.Account {
	return m.opts.accountByID[m.data.destID]
}

func (m *modelTransaction) needsDestAmount() bool {
	if m.data.txType != domain.TxTransfer {
		return false
	}
	src, dst := m.sourceAccount(), m.destAccount()
	return !dst.IsEmpty() && src.Currency != dst.Currency
}

func validateAmount(s string) error {
	v, err := domain.ParseMoney(s)
	if err != nil || v <= 0 {
		return errors.New("enter a positive amount, e.g. 12.34")
	}
	return nil
}
