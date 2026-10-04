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

type modelAccounts struct {
	listPanel
	api AccountsAPI
}

func newModelAccounts(api AccountsAPI) modelAccounts {
	p := newListPanel("Accounts", "account")
	p.withSummary = true
	p.keymap.Sort.SetHelp("s", "group by category")
	return modelAccounts{listPanel: p, api: api}
}

func (m modelAccounts) Init() tea.Cmd { return nil }

func (m modelAccounts) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshAccountsMsg:
		return m, m.refreshCmd()
	case AccountsUpdatedMsg:
		return m, tea.Batch(
			m.list.SetItems(accountItems(m.api.Accounts(), m.sorted)),
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

func accountItems(accounts []domain.Account, grouped bool) []list.Item {
	if grouped {
		accounts = slices.Clone(accounts)
		slices.SortStableFunc(accounts, func(a, b domain.Account) int {
			if a.IsLiability != b.IsLiability {
				if a.IsLiability {
					return 1
				}
				return -1
			}
			return cmp.Compare(categoryOrder(a.Category), categoryOrder(b.Category))
		})
	}
	items := make([]list.Item, 0, len(accounts))
	for _, acc := range accounts {
		items = append(items, accountItem{account: acc})
	}
	return items
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
