/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/notify"
	"ezbk-tui/internal/ui/prompt"
)

type (
	RefreshAccountsMsg struct{}
	AccountsUpdatedMsg struct{}
	NewAccountMsg      struct {
		Name     string
		Category domain.AccountCategory
		Currency string
	}
	AccountCreatedMsg struct{ Name string }
)

type accountItem struct {
	account domain.Account
}

func (i accountItem) Title() string { return i.account.DisplayName() }
func (i accountItem) Description() string {
	return fmt.Sprintf("%s · %s", i.account.Balance.Format(i.account.Currency), i.account.Category)
}
func (i accountItem) FilterValue() string { return i.account.DisplayName() }

// accountSort is the ordering of the accounts panel, cycled with the Sort key.
type accountSort int

const (
	sortDefault accountSort = iota
	sortName
	sortBalance
	sortCurrency
	sortCategory
	sortModeCount
)

const accountsTitle = "Accounts"

func (s accountSort) next() accountSort { return (s + 1) % sortModeCount }

func (s accountSort) label() string {
	switch s {
	case sortName:
		return "by name"
	case sortBalance:
		return "by balance"
	case sortCurrency:
		return "by currency"
	case sortCategory:
		return "by category"
	default:
		return ""
	}
}

// helpLabel describes the mode the Sort key switches to next.
func (s accountSort) helpLabel() string {
	if l := s.next().label(); l != "" {
		return "sort: " + l
	}
	return "sort: default"
}

type modelAccounts struct {
	listPanel
	api      AccountsAPI
	sortMode accountSort
}

func newModelAccounts(api AccountsAPI) modelAccounts {
	p := newListPanel(accountsTitle, "account")
	p.withSummary = true
	m := modelAccounts{listPanel: p, api: api}
	m.applySortMode(sortDefault)
	return m
}

// applySortMode sets the mode and updates the title and help text.
func (m *modelAccounts) applySortMode(mode accountSort) {
	m.sortMode = mode
	m.list.Title = accountsTitle
	if l := mode.label(); l != "" {
		m.list.Title += " · " + l
	}
	m.keymap.Sort.SetHelp("s", mode.helpLabel())
}

func (m modelAccounts) Init() tea.Cmd { return nil }

func (m modelAccounts) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshAccountsMsg:
		return m, m.refreshCmd()
	case AccountsUpdatedMsg:
		return m, tea.Batch(
			m.list.SetItems(accountItems(m.api.Accounts(), m.sortMode, m.api.DefaultCurrency())),
			Cmd(DataLoadCompletedMsg{DataType: "accounts"}),
			Cmd(SummaryUpdateMsg{}),
		)
	case NewAccountMsg:
		api := m.api
		return m, func() tea.Msg {
			opID := startLoading("Creating account...")
			defer stopLoading(opID)
			if err := api.CreateAccount(msg.Name, msg.Category, msg.Currency); err != nil {
				return notify.NotifyWarn(err.Error())()
			}
			return AccountCreatedMsg{Name: msg.Name}
		}
	case AccountCreatedMsg:
		return m, tea.Batch(
			Cmd(RefreshAccountsMsg{}),
			notify.NotifyLog(fmt.Sprintf("Account '%s' created", msg.Name)),
		)
	case UpdatePositions:
		m.resize(msg.layout)
		return m, nil
	}

	if !m.focus {
		return m, nil
	}
	// The sort mode lives on modelAccounts, so the Sort key is handled here
	// instead of through listActions.resort (which closes over a copy of m).
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, m.keymap.Sort) && !m.filterActive() {
		m.applySortMode(m.sortMode.next())
		return m, Cmd(AccountsUpdatedMsg{})
	}
	cmd := m.update(msg, listActions{
		refresh: func() tea.Cmd { return Cmd(RefreshAccountsMsg{}) },
		resort:  func() tea.Cmd { return Cmd(AccountsUpdatedMsg{}) },
		create: func(list.Item) tea.Cmd {
			return CmdPromptNewAccount(m.api.DefaultCurrency(), SetView(accountsView))
		},
		filterBy: func(selected list.Item) tea.Cmd {
			if i, ok := selected.(accountItem); ok {
				return Cmd(FilterMsg{Account: i.account})
			}
			return nil
		},
	})
	return m, cmd
}

func (m modelAccounts) refreshCmd() tea.Cmd {
	api := m.api
	return func() tea.Msg {
		opID := startLoading("Loading accounts...")
		defer stopLoading(opID)
		if err := api.UpdateAccounts(); err != nil {
			return tea.Batch(
				notify.NotifyWarn(err.Error()),
				Cmd(DataLoadCompletedMsg{DataType: "accounts"}),
			)()
		}
		return AccountsUpdatedMsg{}
	}
}

// accountItems builds list items ordered by mode. primaryCurrency goes first
// in the currency mode.
func accountItems(accounts []domain.Account, mode accountSort, primaryCurrency string) []list.Item {
	if less := accountComparator(mode, primaryCurrency); less != nil {
		accounts = slices.Clone(accounts)
		slices.SortStableFunc(accounts, less)
	}
	items := make([]list.Item, 0, len(accounts))
	for _, acc := range accounts {
		items = append(items, accountItem{account: acc})
	}
	return items
}

// accountComparator returns the ordering for mode, or nil for server order.
func accountComparator(mode accountSort, primaryCurrency string) func(a, b domain.Account) int {
	switch mode {
	case sortName:
		return compareAccountNames
	case sortBalance:
		return func(a, b domain.Account) int {
			return cmp.Or(cmp.Compare(b.Balance, a.Balance), compareAccountNames(a, b))
		}
	case sortCurrency:
		return func(a, b domain.Account) int {
			return cmp.Or(
				compareCurrencies(a.Currency, b.Currency, primaryCurrency),
				compareAccountNames(a, b),
			)
		}
	case sortCategory:
		return func(a, b domain.Account) int {
			return cmp.Or(
				compareLiability(a, b),
				cmp.Compare(categoryOrder(a.Category), categoryOrder(b.Category)),
				compareAccountNames(a, b),
			)
		}
	default:
		return nil
	}
}

func compareAccountNames(a, b domain.Account) int {
	return cmp.Compare(strings.ToLower(a.DisplayName()), strings.ToLower(b.DisplayName()))
}

// compareCurrencies orders primary first, then the rest alphabetically.
func compareCurrencies(a, b, primary string) int {
	if a == b {
		return 0
	}
	if a == primary {
		return -1
	}
	if b == primary {
		return 1
	}
	return cmp.Compare(a, b)
}

// compareLiability orders assets before liabilities.
func compareLiability(a, b domain.Account) int {
	switch {
	case a.IsLiability == b.IsLiability:
		return 0
	case a.IsLiability:
		return 1
	default:
		return -1
	}
}

func categoryOrder(c domain.AccountCategory) int {
	if i := slices.Index(domain.AllAccountCategories, c); i >= 0 {
		return i
	}
	return len(domain.AllAccountCategories)
}

// CmdPromptNewAccount asks for "<name>[,<currency>[,<category>]]".
func CmdPromptNewAccount(defaultCurrency string, backCmd tea.Cmd) tea.Cmd {
	return prompt.Ask(
		fmt.Sprintf("New account <name>[,<currency=%s>[,<category=checking>]] (cash|checking|savings|credit|debt|virtual|investment|receivables|deposit): ", defaultCurrency),
		"",
		func(value string) tea.Cmd {
			if value == "None" {
				return backCmd
			}
			msg, err := parseNewAccount(value, defaultCurrency)
			if err != nil {
				return tea.Sequence(notify.NotifyWarn(err.Error()), backCmd)
			}
			return tea.Sequence(Cmd(msg), backCmd)
		},
	)
}

func parseNewAccount(value, defaultCurrency string) (NewAccountMsg, error) {
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if parts[0] == "" {
		return NewAccountMsg{}, fmt.Errorf("account name is required")
	}
	currency := defaultCurrency
	if len(parts) > 1 && parts[1] != "" {
		currency = parts[1]
	}
	if len(currency) != 3 {
		return NewAccountMsg{}, fmt.Errorf("invalid currency %q, expected a 3-letter code", currency)
	}
	category := domain.AccountChecking
	if len(parts) > 2 && parts[2] != "" {
		c, err := domain.ParseAccountCategory(parts[2])
		if err != nil {
			return NewAccountMsg{}, err
		}
		category = c
	}
	return NewAccountMsg{Name: parts[0], Currency: strings.ToUpper(currency), Category: category}, nil
}
