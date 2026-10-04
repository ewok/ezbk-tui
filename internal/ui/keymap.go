/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type UIKeyMap struct {
	Quit          key.Binding
	ShowShortHelp key.Binding
	PeriodPicker  key.Binding
}

// ViewKeyMap holds the tab switching keys shared by all panels.
type ViewKeyMap struct {
	ViewTransactions key.Binding
	ViewAccounts     key.Binding
	ViewExpense      key.Binding
	ViewIncome       key.Binding
	ViewTags         key.Binding
}

// ListKeyMap is used by the left-side list panels (accounts, categories, tags).
type ListKeyMap struct {
	ViewKeyMap
	ShowFullHelp key.Binding
	Quit         key.Binding
	Refresh      key.Binding
	Filter       key.Binding
	FilterBy     key.Binding
	ResetFilter  key.Binding
	Sort         key.Binding
	New          key.Binding
	Select       key.Binding
}

type TransactionsKeyMap struct {
	ViewKeyMap
	ShowFullHelp       key.Binding
	Quit               key.Binding
	Refresh            key.Binding
	Filter             key.Binding
	ResetFilter        key.Binding
	Search             key.Binding
	NewView            key.Binding
	NewTransactionFrom key.Binding
	FromTemplate       key.Binding
	ContinueEditing    key.Binding
	Select             key.Binding
	Delete             key.Binding
	ToggleFullView     key.Binding
}

type TransactionFormKeyMap struct {
	Reset          key.Binding
	Cancel         key.Binding
	Submit         key.Binding
	EditFormAgain  key.Binding
	ToggleLastDate key.Binding
}

type TemplatesKeyMap struct {
	Quit    key.Binding
	Select  key.Binding
	Refresh key.Binding
	Filter  key.Binding
}

func binding(keys, help string, k ...string) key.Binding {
	return key.NewBinding(key.WithKeys(append([]string{keys}, k...)...), key.WithHelp(keys, help))
}

func DefaultUIKeyMap() UIKeyMap {
	return UIKeyMap{
		Quit:          binding("ctrl+c", "quit"),
		ShowShortHelp: binding("?", "toggle help"),
		PeriodPicker:  binding("p", "period picker"),
	}
}

func DefaultViewKeyMap() ViewKeyMap {
	return ViewKeyMap{
		ViewTransactions: binding("t", "transactions"),
		ViewAccounts:     binding("a", "accounts"),
		ViewExpense:      binding("e", "expense categories"),
		ViewIncome:       binding("i", "income categories"),
		ViewTags:         binding("o", "tags"),
	}
}

func DefaultListKeyMap(item string) ListKeyMap {
	return ListKeyMap{
		ViewKeyMap:   DefaultViewKeyMap(),
		ShowFullHelp: binding("?", "toggle help"),
		Quit:         binding("esc", "go back"),
		Refresh:      binding("r", "refresh"),
		Filter:       binding("/", "filter list"),
		FilterBy:     binding("f", "filter transactions by "+item+" (twice: exclusive)"),
		ResetFilter:  binding("ctrl+a", "reset filter"),
		Sort:         binding("s", "sort / hide empty"),
		New:          binding("n", "new "+item),
		Select:       binding("enter", "show transactions"),
	}
}

func DefaultTransactionsKeyMap() TransactionsKeyMap {
	vk := DefaultViewKeyMap()
	vk.ViewTransactions = binding("t", "toggle full view")
	vk.ViewTransactions.SetEnabled(false)
	return TransactionsKeyMap{
		ViewKeyMap:         vk,
		ShowFullHelp:       binding("?", "toggle help"),
		Quit:               binding("ctrl+c", "quit"),
		Refresh:            binding("r", "refresh all"),
		Filter:             binding("/", "filter (twice: exclusive)"),
		ResetFilter:        binding("ctrl+a", "reset filter"),
		Search:             binding("s", "search (all time)"),
		NewView:            binding("n", "new transaction"),
		NewTransactionFrom: binding("N", "new from selected"),
		FromTemplate:       binding("T", "new from template"),
		ContinueEditing:    binding("esc", "continue editing"),
		Select:             binding("enter", "edit transaction"),
		Delete:             binding("D", "delete transaction"),
		ToggleFullView:     binding("t", "toggle full view"),
	}
}

func DefaultTransactionFormKeyMap() TransactionFormKeyMap {
	return TransactionFormKeyMap{
		Reset:          binding("ctrl+n", "reset form"),
		Cancel:         binding("esc", "back (keeps form)"),
		Submit:         binding("ctrl+s", "save"),
		EditFormAgain:  binding("ctrl+e", "edit form again"),
		ToggleLastDate: binding("ctrl+t", "toggle last saved date"),
	}
}

func DefaultTemplatesKeyMap() TemplatesKeyMap {
	return TemplatesKeyMap{
		Quit:    binding("esc", "back"),
		Select:  binding("enter", "use template"),
		Refresh: binding("r", "refresh templates"),
		Filter:  binding("/", "filter"),
	}
}

func (k UIKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.ShowShortHelp, k.Quit, k.PeriodPicker}
}

func (k UIKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.PeriodPicker}}
}

func (k ViewKeyMap) bindings() []key.Binding {
	return []key.Binding{k.ViewTransactions, k.ViewAccounts, k.ViewExpense, k.ViewIncome, k.ViewTags}
}

func (k ListKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.ShowFullHelp, k.Quit, k.Filter, k.FilterBy, k.ResetFilter,
		k.Sort, k.Select, k.New, k.Refresh,
	}
}

func (k ListKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp(), k.bindings()}
}

func (k TransactionsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.ShowFullHelp, k.Quit, k.ToggleFullView, k.Search, k.Filter, k.ResetFilter,
		k.NewView, k.NewTransactionFrom, k.FromTemplate, k.ContinueEditing,
		k.Select, k.Delete, k.Refresh,
	}
}

func (k TransactionsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp(), k.bindings()}
}

func (k TransactionFormKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Cancel, k.Reset, k.EditFormAgain, k.ToggleLastDate}
}

func (k TransactionFormKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

func (k TemplatesKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Filter, k.Quit, k.Refresh}
}

func (k TemplatesKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// viewSwitch returns the command for a tab switching key, if any.
func (k ViewKeyMap) viewSwitch(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(km, k.ViewTransactions):
		return SetView(transactionsView)
	case key.Matches(km, k.ViewAccounts):
		return SetView(accountsView)
	case key.Matches(km, k.ViewExpense):
		return SetView(expenseView)
	case key.Matches(km, k.ViewIncome):
		return SetView(incomeView)
	case key.Matches(km, k.ViewTags):
		return SetView(tagsView)
	}
	return nil
}
