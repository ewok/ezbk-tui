/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"reflect"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ezbk-tui/internal/domain"
)

var (
	accWallet = domain.Account{ID: "10", Name: "Wallet", Category: domain.AccountCash, Currency: "USD", Balance: 12500, IsAsset: true}
	accEUR    = domain.Account{ID: "21", Name: "EUR", ParentName: "Bank", ParentID: "20", Category: domain.AccountChecking, Currency: "EUR", Balance: 250000, IsAsset: true}
	accUSD2   = domain.Account{ID: "22", Name: "Savings", Category: domain.AccountSavings, Currency: "USD", Balance: 100, IsAsset: true}
	accVisa   = domain.Account{ID: "30", Name: "Visa", Category: domain.AccountCreditCard, Currency: "USD", Balance: -45000, IsLiability: true}

	catFood      = domain.Category{ID: "200", Name: "Food", Type: domain.CategoryExpense}
	catGroceries = domain.Category{ID: "201", Name: "Groceries", ParentID: "200", ParentName: "Food", Type: domain.CategoryExpense}
	catFuel      = domain.Category{ID: "211", Name: "Fuel", ParentID: "210", ParentName: "Transport", Type: domain.CategoryExpense}
	catTransport = domain.Category{ID: "210", Name: "Transport", Type: domain.CategoryExpense}
	catSalary    = domain.Category{ID: "100", Name: "Salary", Type: domain.CategoryIncome}
	catMainJob   = domain.Category{ID: "101", Name: "Main job", ParentID: "100", ParentName: "Salary", Type: domain.CategoryIncome}
	catGeneral   = domain.Category{ID: "300", Name: "General", Type: domain.CategoryTransfer}
	catMove      = domain.Category{ID: "301", Name: "Move", ParentID: "300", ParentName: "General", Type: domain.CategoryTransfer}

	tagTrip = domain.Tag{ID: "7", Name: "trip"}
	tagWork = domain.Tag{ID: "8", Name: "work"}
)

type mockAPI struct {
	loc        *time.Location
	start      time.Time
	accounts   []domain.Account
	categories []domain.Category
	tags       []domain.Tag
	templates  []domain.Template
	txs        []domain.Transaction
	totals     map[string]domain.Amounts
	income     domain.Amounts
	expense    domain.Amounts

	created      []domain.TransactionRequest
	updated      []domain.TransactionRequest
	deleted      []string
	newAccounts  []string
	newCats      []string
	newTags      []string
	lastQuery    string
	createErr    error
	statsUpdates int
}

func newMockAPI() *mockAPI {
	loc := time.FixedZone("CEST", 2*3600)
	return &mockAPI{
		loc:        loc,
		start:      time.Date(2026, 10, 1, 0, 0, 0, 0, loc),
		accounts:   []domain.Account{accWallet, accEUR, accUSD2, accVisa},
		categories: []domain.Category{catFood, catGroceries, catTransport, catFuel, catSalary, catMainJob, catGeneral, catMove},
		tags:       []domain.Tag{tagTrip, tagWork},
		txs: []domain.Transaction{
			{ID: "1", Type: domain.TxExpense, Time: time.Date(2026, 10, 3, 9, 15, 0, 0, loc), Category: catGroceries, Source: accWallet, SourceAmount: 1550, Tags: []domain.Tag{tagTrip}, Comment: "milk", Editable: true},
			{ID: "2", Type: domain.TxIncome, Time: time.Date(2026, 10, 2, 8, 0, 0, 0, loc), Category: catMainJob, Source: accEUR, SourceAmount: 300000, Editable: true},
			{ID: "3", Type: domain.TxTransfer, Time: time.Date(2026, 10, 1, 8, 0, 0, 0, loc), Category: catMove, Source: accWallet, Destination: accEUR, SourceAmount: 10000, DestinationAmount: 9100, Comment: "fx"},
			{ID: "4", Type: domain.TxExpense, Time: time.Date(2026, 10, 4, 18, 0, 0, 0, loc), Category: catFuel, Source: accVisa, SourceAmount: 6000, Tags: []domain.Tag{tagWork}, Comment: "gas", Editable: true},
		},
		totals: map[string]domain.Amounts{
			"200": {"USD": 1550}, "201": {"USD": 1550},
			"210": {"USD": 6000}, "211": {"USD": 6000},
			"100": {"EUR": 300000}, "101": {"EUR": 300000},
		},
		income:  domain.Amounts{"EUR": 300000},
		expense: domain.Amounts{"USD": 7550},
	}
}

func (m *mockAPI) PeriodStart() time.Time { return m.start }
func (m *mockAPI) PeriodEnd() time.Time   { return m.start.AddDate(0, 1, 0).Add(-time.Second) }
func (m *mockAPI) SetPeriod(y int, mo time.Month) {
	m.start = time.Date(y, mo, 1, 0, 0, 0, 0, m.loc)
}
func (m *mockAPI) DefaultCurrency() string    { return "USD" }
func (m *mockAPI) DefaultAccountID() string   { return "10" }
func (m *mockAPI) Location() *time.Location   { return m.loc }
func (m *mockAPI) TimeoutSeconds() int        { return 1 }
func (m *mockAPI) UpdateAccounts() error      { return nil }
func (m *mockAPI) Accounts() []domain.Account { return m.accounts }
func (m *mockAPI) CreateAccount(name string, _ domain.AccountCategory, _ string) error {
	m.newAccounts = append(m.newAccounts, name)
	return nil
}
func (m *mockAPI) UpdatePeriodStats() error { m.statsUpdates++; return nil }
func (m *mockAPI) UpdateCategories() error  { return nil }
func (m *mockAPI) Categories(t domain.CategoryType) []domain.Category {
	out := []domain.Category{}
	for _, c := range m.categories {
		if c.Type == t {
			out = append(out, c)
		}
	}
	return out
}
func (m *mockAPI) CategoryTotal(id string) domain.Amounts {
	if a, ok := m.totals[id]; ok {
		return a
	}
	return domain.Amounts{}
}
func (m *mockAPI) PeriodIncome() domain.Amounts  { return m.income }
func (m *mockAPI) PeriodExpense() domain.Amounts { return m.expense }
func (m *mockAPI) CreateCategory(name string, t domain.CategoryType, parentID string) (string, error) {
	id := "9" + name
	m.newCats = append(m.newCats, name+"@"+parentID)
	m.categories = append(m.categories, domain.Category{ID: id, Name: name, ParentID: parentID, Type: t})
	return id, nil
}
func (m *mockAPI) UpdateTags() error  { return nil }
func (m *mockAPI) Tags() []domain.Tag { return m.tags }
func (m *mockAPI) TagTotals(id string) (domain.Amounts, domain.Amounts) {
	if id == tagTrip.ID {
		return domain.Amounts{"USD": 1550}, domain.Amounts{}
	}
	return domain.Amounts{}, domain.Amounts{}
}
func (m *mockAPI) CreateTag(name string) error {
	m.newTags = append(m.newTags, name)
	return nil
}
func (m *mockAPI) NetWorth() (domain.Amounts, domain.Amounts) {
	assets, liabilities := domain.Amounts{}, domain.Amounts{}
	for _, a := range m.accounts {
		if a.IsLiability {
			liabilities.Add(a.Currency, a.Balance)
		} else {
			assets.Add(a.Currency, a.Balance)
		}
	}
	return assets, liabilities
}
func (m *mockAPI) ListTransactions(q string) ([]domain.Transaction, error) {
	m.lastQuery = q
	return m.txs, nil
}
func (m *mockAPI) DeleteTransaction(id string) error {
	m.deleted = append(m.deleted, id)
	return nil
}
func (m *mockAPI) CreateTransaction(req domain.TransactionRequest) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	m.created = append(m.created, req)
	return "999", nil
}
func (m *mockAPI) UpdateTransaction(req domain.TransactionRequest) (string, error) {
	m.updated = append(m.updated, req)
	return req.ID, nil
}
func (m *mockAPI) UpdateTemplates() error       { return nil }
func (m *mockAPI) Templates() []domain.Template { return m.templates }

// runCmd executes a command and flattens batches/sequences into messages.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, runCmd(c)...)
		}
		return out
	case nil:
		return nil
	}
	if isSequence(msg) {
		return flattenSequence(msg)
	}
	return []tea.Msg{msg}
}

// isSequence detects tea's unexported sequenceMsg ([]tea.Cmd).
func isSequence(msg tea.Msg) bool {
	v := reflect.ValueOf(msg)
	return v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[tea.Cmd]()
}

func flattenSequence(msg tea.Msg) []tea.Msg {
	v := reflect.ValueOf(msg)
	var out []tea.Msg
	for i := range v.Len() {
		if c, ok := v.Index(i).Interface().(tea.Cmd); ok {
			out = append(out, runCmd(c)...)
		}
	}
	return out
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func hasMsg[T any](msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(T); ok {
			return true
		}
	}
	return false
}

func findMsg[T any](msgs []tea.Msg) (T, bool) {
	for _, m := range msgs {
		if v, ok := m.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}
