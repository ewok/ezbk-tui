/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"cmp"
	"errors"
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
	RefreshCategoriesMsg struct{}
	CategoriesUpdatedMsg struct{}
	NewCategoryMsg       struct {
		Type   domain.CategoryType
		Parent string
		Name   string
	}
	CategoryCreatedMsg struct{ Name string }
)

type categoryItem struct {
	category domain.Category
	total    domain.Amounts
	view     currencyView
	label    string
	flat     bool
}

func (i categoryItem) Title() string {
	if i.flat {
		return i.category.DisplayName()
	}
	if i.category.IsPrimary() {
		return i.category.Name
	}
	return "  · " + i.category.Name
}

func (i categoryItem) Description() string {
	prefix := ""
	if !i.flat && !i.category.IsPrimary() {
		prefix = "    "
	}
	if i.total.IsZero() {
		return prefix + "No transactions"
	}
	return prefix + i.label + ": " + formatTotal(i.total, i.view, i.view.convert)
}

func (i categoryItem) FilterValue() string { return i.category.DisplayName() }

type modelCategories struct {
	listPanel
	api       CategoriesAPI
	catType   domain.CategoryType
	view      state
	isLoader  bool
	converted bool
}

func newModelCategories(api CategoriesAPI, t domain.CategoryType, view state, isLoader bool) modelCategories {
	title := "Expense categories"
	if t == domain.CategoryIncome {
		title = "Income categories"
	}
	p := newListPanel(title, "category")
	return modelCategories{listPanel: p, api: api, catType: t, view: view, isLoader: isLoader, converted: true}
}

func (m modelCategories) Init() tea.Cmd { return nil }

func (m modelCategories) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshCategoriesMsg:
		if !m.isLoader {
			return m, nil
		}
		return m, m.refreshCmd()
	case CategoriesUpdatedMsg, StatsUpdatedMsg:
		cmds := []tea.Cmd{m.updateItemsCmd()}
		if _, ok := msg.(CategoriesUpdatedMsg); ok && m.isLoader {
			cmds = append(cmds, Cmd(DataLoadCompletedMsg{DataType: "categories"}))
		}
		return m, tea.Batch(cmds...)
	case CurrencyModeMsg:
		m.converted = msg.Converted
		return m, m.updateItemsCmd()
	case NewCategoryMsg:
		if msg.Type != m.catType {
			return m, nil
		}
		return m, m.createCmd(msg)
	case CategoryCreatedMsg:
		if !m.isLoader {
			return m, nil
		}
		return m, tea.Batch(
			Cmd(RefreshCategoriesMsg{}),
			notify.NotifyLog(fmt.Sprintf("Category '%s' created", msg.Name)),
		)
	case UpdatePositions:
		m.resize(msg.layout)
		return m, nil
	}

	if !m.focus {
		return m, nil
	}
	cmd := m.update(msg, listActions{
		refresh: func() tea.Cmd { return Cmd(RefreshCategoriesMsg{}) },
		resort:  func() tea.Cmd { return m.updateItemsCmd() },
		create: func(selected list.Item) tea.Cmd {
			parent := ""
			if i, ok := selected.(categoryItem); ok {
				parent = i.category.ParentName
				if i.category.IsPrimary() {
					parent = i.category.Name
				}
			}
			return CmdPromptNewCategory(m.catType, parent, SetView(m.view))
		},
		filterBy: func(selected list.Item) tea.Cmd {
			if i, ok := selected.(categoryItem); ok {
				return Cmd(FilterMsg{Category: i.category})
			}
			return nil
		},
	})
	return m, cmd
}

func (m modelCategories) refreshCmd() tea.Cmd {
	api := m.api
	return func() tea.Msg {
		opID := startLoading("Loading categories...")
		defer stopLoading(opID)
		if err := api.UpdateCategories(); err != nil {
			return tea.Batch(
				notify.NotifyWarn(err.Error()),
				Cmd(DataLoadCompletedMsg{DataType: "categories"}),
			)()
		}
		return CategoriesUpdatedMsg{}
	}
}

func (m modelCategories) createCmd(msg NewCategoryMsg) tea.Cmd {
	api := m.api
	return func() tea.Msg {
		opID := startLoading("Creating category...")
		defer stopLoading(opID)
		if err := createCategoryPath(api, msg); err != nil {
			return notify.NotifyWarn(err.Error())()
		}
		return CategoryCreatedMsg{Name: msg.Name}
	}
}

// createCategoryPath creates "<name>" as a primary category, or "<parent>/<name>"
// as a secondary category, creating the parent first when it does not exist.
func createCategoryPath(api CategoriesAPI, msg NewCategoryMsg) error {
	if msg.Parent == "" {
		_, err := api.CreateCategory(msg.Name, msg.Type, "")
		return err
	}
	parentID := ""
	for _, c := range api.Categories(msg.Type) {
		if c.IsPrimary() && strings.EqualFold(c.Name, msg.Parent) {
			parentID = c.ID
			break
		}
	}
	if parentID == "" {
		id, err := api.CreateCategory(msg.Parent, msg.Type, "")
		if err != nil {
			return err
		}
		parentID = id
	}
	_, err := api.CreateCategory(msg.Name, msg.Type, parentID)
	return err
}

func (m *modelCategories) updateItemsCmd() tea.Cmd {
	items := categoryItems(m.api, m.catType, m.sorted, m.converted)
	label, total := m.totalLabel()
	items = slices.Insert(items, 0, list.Item(simpleItem{
		title: "Total",
		desc:  label + ": " + formatTotal(total, m.api, m.converted),
	}))
	return m.list.SetItems(items)
}

func (m *modelCategories) totalLabel() (string, domain.Amounts) {
	if m.catType == domain.CategoryIncome {
		return "Earned", m.api.PeriodIncome()
	}
	return "Spent", m.api.PeriodExpense()
}

// categoryItems builds the category rows; with convert, totals are shown and
// sorted in the default currency.
func categoryItems(api CategoriesAPI, t domain.CategoryType, sorted, convert bool) []list.Item {
	view := newCurrencyView(api, convert)
	label := "Spent"
	if t == domain.CategoryIncome {
		label = "Earned"
	}
	items := []list.Item{}
	for _, c := range api.Categories(t) {
		total := api.CategoryTotal(c.ID)
		if sorted && (c.IsPrimary() || total.IsZero()) {
			continue
		}
		items = append(items, categoryItem{category: c, total: total, view: view, label: label, flat: sorted})
	}
	if sorted {
		slices.SortStableFunc(items, func(a, b list.Item) int {
			return cmp.Compare(view.sortKey(b.(categoryItem).total), view.sortKey(a.(categoryItem).total))
		})
	}
	return items
}

// CmdPromptNewCategory asks for "<name>" or "<parent>/<name>".
func CmdPromptNewCategory(t domain.CategoryType, parent string, backCmd tea.Cmd) tea.Cmd {
	initial := ""
	if parent != "" {
		initial = parent + "/"
	}
	return prompt.Ask(
		fmt.Sprintf("New %s category (<parent>/<name> for a sub-category, <name> for a top-level one): ", strings.ToLower(t.String())),
		initial,
		func(value string) tea.Cmd {
			if value == "None" {
				return backCmd
			}
			msg, err := parseNewCategory(t, value)
			if err != nil {
				return tea.Sequence(notify.NotifyWarn(err.Error()), backCmd)
			}
			return tea.Sequence(Cmd(msg), backCmd)
		},
	)
}

func parseNewCategory(t domain.CategoryType, value string) (NewCategoryMsg, error) {
	parent, name, found := strings.Cut(value, "/")
	parent, name = strings.TrimSpace(parent), strings.TrimSpace(name)
	if !found {
		name, parent = parent, ""
	}
	if name == "" || (found && parent == "") {
		return NewCategoryMsg{}, errors.New("category name is required")
	}
	return NewCategoryMsg{Type: t, Parent: parent, Name: name}, nil
}
