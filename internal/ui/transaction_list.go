/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/notify"
	"ezbk-tui/internal/ui/prompt"
)

type (
	FilterMsg struct {
		TrxID    string
		Account  domain.Account
		Category domain.Category
		Tag      domain.Tag
		Query    string
		Reset    bool
	}
	SearchMsg              struct{ Query string }
	RefreshTransactionsMsg struct{ TrxID string }
	TransactionsUpdateMsg  struct {
		TrxID        string
		Transactions []domain.Transaction
	}
	DeleteTransactionMsg       struct{ Transaction domain.Transaction }
	TransactionDeleteResultMsg struct{ Err error }
)

// txFilter is the active client-side filter of the transaction list.
type txFilter struct {
	account  domain.Account
	category domain.Category
	tag      domain.Tag
	query    string
}

func (f txFilter) match(tx domain.Transaction) bool {
	if !f.account.IsEmpty() && !tx.InvolvesAccount(f.account.ID) {
		return false
	}
	if !f.category.IsEmpty() && tx.Category.ID != f.category.ID && tx.Category.ParentID != f.category.ID {
		return false
	}
	if !f.tag.IsEmpty() && !tx.HasTag(f.tag.ID) {
		return false
	}
	if f.query != "" {
		haystack := strings.Join([]string{
			tx.Comment, tx.Source.DisplayName(), tx.Destination.DisplayName(),
			tx.Category.DisplayName(), tx.Source.Currency, tx.TagNames(),
			tx.SourceAmount.String(), tx.Type.String(),
		}, "\x00")
		if !CaseInsensitiveContains(haystack, f.query) {
			return false
		}
	}
	return true
}

type modelTransactions struct {
	table         table.Model
	transactions  []domain.Transaction
	api           TransactionAPI
	filter        txFilter
	savedFilter   txFilter
	currentSearch string
	focus         bool
	keymap        TransactionsKeyMap
	styles        Styles
}

func NewModelTransactions(api TransactionAPI) modelTransactions {
	rows, columns := getRows(nil)
	t := table.New(table.WithColumns(columns), table.WithRows(rows), table.WithFocused(true))

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return modelTransactions{
		table:  t,
		api:    api,
		keymap: DefaultTransactionsKeyMap(),
		styles: DefaultStyles(),
	}
}

func (m modelTransactions) Init() tea.Cmd { return nil }

func (m modelTransactions) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SearchMsg:
		m.applySearch(msg.Query)
		return m, Cmd(RefreshTransactionsMsg{})
	case FilterMsg:
		m.applyFilter(msg)
		m.renderRows(msg.TrxID)
		return m, nil
	case RefreshTransactionsMsg:
		return m, m.refreshCmd(msg.TrxID)
	case TransactionsUpdateMsg:
		m.transactions = msg.Transactions
		m.renderRows(msg.TrxID)
		return m, nil
	case DeleteTransactionMsg:
		api := m.api
		id := msg.Transaction.ID
		return m, func() tea.Msg {
			opID := startLoading("Deleting transaction...")
			defer stopLoading(opID)
			return TransactionDeleteResultMsg{Err: api.DeleteTransaction(id)}
		}
	case TransactionDeleteResultMsg:
		if msg.Err != nil {
			return m, notify.NotifyError("Error deleting transaction: " + msg.Err.Error())
		}
		return m, tea.Batch(notify.NotifyLog("Transaction deleted."), refreshAfterWrite(""))
	case UpdatePositions:
		if msg.layout != nil {
			h, v := m.styles.Base.GetFrameSize()
			m.table.SetWidth(max(1, msg.layout.Width-msg.layout.LeftSize-h))
			m.table.SetHeight(max(2, msg.layout.Height-msg.layout.TopSize-v))
		}
		return m, nil
	}

	if !m.focus {
		return m, nil
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		if cmd, handled := m.handleKey(km); handled {
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *modelTransactions) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if cmd := m.keymap.viewSwitch(msg); cmd != nil {
		return cmd, true
	}
	switch {
	case key.Matches(msg, m.keymap.Quit):
		return tea.Quit, true
	case key.Matches(msg, m.keymap.Refresh):
		return Cmd(RefreshAllMsg{}), true
	case key.Matches(msg, m.keymap.Filter):
		return prompt.Ask("Filter (ESC to reset): ", m.filter.query, func(value string) tea.Cmd {
			return tea.Sequence(Cmd(FilterMsg{Query: value}), SetView(transactionsView))
		}), true
	case key.Matches(msg, m.keymap.Search):
		return prompt.Ask("Search all transactions by keyword (ESC to exit search): ", m.currentSearch, func(value string) tea.Cmd {
			return tea.Sequence(Cmd(SearchMsg{Query: value}), SetView(transactionsView))
		}), true
	case key.Matches(msg, m.keymap.NewView):
		return Cmd(NewTransactionMsg{Transaction: m.prefilledTransaction()}), true
	case key.Matches(msg, m.keymap.ContinueEditing):
		return Cmd(ContinueTransactionMsg{}), true
	case key.Matches(msg, m.keymap.FromTemplate):
		return SetView(templatesView), true
	case key.Matches(msg, m.keymap.NewTransactionFrom):
		trx, err := m.GetCurrentTransaction()
		if err != nil {
			return notify.NotifyWarn(err.Error()), true
		}
		return Cmd(NewTransactionMsg{Transaction: copyForNew(trx)}), true
	case key.Matches(msg, m.keymap.Select):
		trx, err := m.GetCurrentTransaction()
		if err != nil {
			return notify.NotifyWarn(err.Error()), true
		}
		if !trx.Editable {
			return notify.NotifyWarn("This transaction is read-only (not editable in ezBookkeeping)."), true
		}
		return Cmd(EditTransactionMsg{Transaction: trx}), true
	case key.Matches(msg, m.keymap.Delete):
		return m.confirmDelete(), true
	case key.Matches(msg, m.keymap.ResetFilter):
		return Cmd(FilterMsg{Reset: true}), true
	case key.Matches(msg, m.keymap.ToggleFullView):
		return Cmd(ViewFullTransactionViewMsg{}), true
	}
	return nil, false
}

func (m *modelTransactions) confirmDelete() tea.Cmd {
	trx, err := m.GetCurrentTransaction()
	if err != nil {
		return notify.NotifyWarn(err.Error())
	}
	if !trx.Editable {
		return notify.NotifyWarn("This transaction is read-only (not editable in ezBookkeeping).")
	}
	return prompt.Ask(
		fmt.Sprintf("Delete transaction %s %s %s? Type 'yes!' to confirm: ",
			trx.Time.Format("2006-01-02"), trx.SourceAmount.Format(trx.Source.Currency), trx.Description()),
		"",
		func(value string) tea.Cmd {
			if value == "yes!" {
				return tea.Sequence(SetView(transactionsView), Cmd(DeleteTransactionMsg{Transaction: trx}))
			}
			return SetView(transactionsView)
		},
	)
}

func (m modelTransactions) refreshCmd(trxID string) tea.Cmd {
	api, query := m.api, m.currentSearch
	return func() tea.Msg {
		opID := startLoading("Loading transactions...")
		defer stopLoading(opID)
		txs, err := api.ListTransactions(query)
		if err != nil {
			return notify.NotifyWarn(err.Error())()
		}
		return TransactionsUpdateMsg{TrxID: trxID, Transactions: txs}
	}
}

func (m *modelTransactions) applySearch(query string) {
	if query == "None" {
		if m.currentSearch == "" {
			return
		}
		m.currentSearch = ""
		m.filter = m.savedFilter
		return
	}
	if m.currentSearch == "" {
		m.savedFilter = m.filter
		m.filter = txFilter{}
	}
	m.currentSearch = query
}

// applyFilter merges a filter message. Applying the same filter twice makes it exclusive.
func (m *modelTransactions) applyFilter(msg FilterMsg) {
	if msg.Reset {
		m.filter = txFilter{}
	}
	if msg.Query == "None" {
		m.filter.query = ""
	}
	if !msg.Account.IsEmpty() {
		if msg.Account.ID == m.filter.account.ID {
			m.filter = txFilter{}
		}
		m.filter.account = msg.Account
	}
	if !msg.Category.IsEmpty() {
		if msg.Category.ID == m.filter.category.ID {
			m.filter = txFilter{}
		}
		m.filter.category = msg.Category
	}
	if !msg.Tag.IsEmpty() {
		if msg.Tag.ID == m.filter.tag.ID {
			m.filter = txFilter{}
		}
		m.filter.tag = msg.Tag
	}
	if msg.Query != "" && msg.Query != "None" {
		if msg.Query == m.filter.query {
			m.filter = txFilter{}
		}
		m.filter.query = msg.Query
	}
}

func (m *modelTransactions) visible() []domain.Transaction {
	out := []domain.Transaction{}
	for _, tx := range m.transactions {
		if m.filter.match(tx) {
			out = append(out, tx)
		}
	}
	return out
}

func (m *modelTransactions) renderRows(selectID string) {
	rows, columns := getRows(m.visible())
	m.table.SetRows(rows)
	m.table.SetColumns(columns)
	if selectID == "" {
		return
	}
	for i, row := range rows {
		if rowTxID(row) == selectID {
			m.table.SetCursor(i)
			return
		}
	}
}

func (m modelTransactions) View() string { return m.table.View() }

func (m *modelTransactions) Blur() {
	m.table.Blur()
	m.focus = false
}

func (m *modelTransactions) Focus() {
	m.table.Focus()
	m.focus = true
}

// prefilledTransaction builds a new transaction from the active filters.
func (m *modelTransactions) prefilledTransaction() domain.Transaction {
	tx := domain.Transaction{Type: domain.TxExpense, Source: m.filter.account, Category: m.filter.category}
	if !m.filter.category.IsEmpty() {
		switch m.filter.category.Type {
		case domain.CategoryIncome:
			tx.Type = domain.TxIncome
		case domain.CategoryTransfer:
			tx.Type = domain.TxTransfer
		}
	}
	if !m.filter.tag.IsEmpty() {
		tx.Tags = []domain.Tag{m.filter.tag}
	}
	return tx
}

func copyForNew(tx domain.Transaction) domain.Transaction {
	tx.ID = ""
	tx.Editable = true
	return tx
}

var errNoTransactions = errors.New("no transactions in the list")

// GetCurrentTransaction returns the transaction under the cursor.
func (m *modelTransactions) GetCurrentTransaction() (domain.Transaction, error) {
	if len(m.table.Rows()) == 0 {
		return domain.Transaction{}, errNoTransactions
	}
	row := m.table.SelectedRow()
	if row == nil {
		return domain.Transaction{}, errors.New("transaction not selected")
	}
	id := rowTxID(row)
	for _, tx := range m.transactions {
		if tx.ID == id {
			return tx, nil
		}
	}
	return domain.Transaction{}, errors.New("transaction not found")
}

func rowTxID(row table.Row) string {
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

type column struct {
	title string
	min   int
	value func(tx domain.Transaction) string
}

var transactionColumns = []column{
	{"ID", 0, func(tx domain.Transaction) string { return tx.ID }},
	{"T", 2, func(tx domain.Transaction) string {
		if !tx.Editable {
			return tx.Type.Icon() + "*"
		}
		return tx.Type.Icon()
	}},
	{"Date", 10, func(tx domain.Transaction) string { return tx.Time.Format("2006-01-02") }},
	{"Account", 7, func(tx domain.Transaction) string { return tx.Source.DisplayName() }},
	{"To", 4, func(tx domain.Transaction) string { return tx.Destination.DisplayName() }},
	{"Category", 8, func(tx domain.Transaction) string { return tx.Category.DisplayName() }},
	{"Amount", 6, func(tx domain.Transaction) string { return signedAmount(tx) }},
	{"Cur", 3, func(tx domain.Transaction) string { return tx.Source.Currency }},
	{"To amount", 9, func(tx domain.Transaction) string {
		if tx.Type != domain.TxTransfer || tx.Destination.Currency == tx.Source.Currency {
			return ""
		}
		return tx.DestinationAmount.Format(tx.Destination.Currency)
	}},
	{"Tags", 4, func(tx domain.Transaction) string { return tx.TagNames() }},
	{"Comment", 10, func(tx domain.Transaction) string { return tx.Comment }},
}

func signedAmount(tx domain.Transaction) string {
	switch tx.Type {
	case domain.TxExpense:
		return (-tx.SourceAmount).String()
	case domain.TxIncome:
		return "+" + tx.SourceAmount.String()
	}
	return tx.SourceAmount.String()
}

func getRows(transactions []domain.Transaction) ([]table.Row, []table.Column) {
	widths := make([]int, len(transactionColumns))
	for i, c := range transactionColumns {
		widths[i] = c.min
	}
	rows := make([]table.Row, 0, len(transactions))
	for _, tx := range transactions {
		row := make(table.Row, len(transactionColumns))
		for i, c := range transactionColumns {
			row[i] = c.value(tx)
			if i > 0 {
				widths[i] = max(widths[i], lipgloss.Width(row[i]))
			}
		}
		rows = append(rows, row)
	}
	columns := make([]table.Column, len(transactionColumns))
	for i, c := range transactionColumns {
		columns[i] = table.Column{Title: c.title, Width: widths[i]}
	}
	return rows, columns
}
