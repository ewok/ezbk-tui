/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m modelUI) View() string {
	var s strings.Builder
	s.WriteString(m.headerView() + "\n")
	s.WriteString(m.bodyView() + "\n")
	s.WriteString(m.notify.WithWidth(m.layout.GetWidth()).View() + "\n")
	s.WriteString(m.help.Styles.ShortKey.Render(m.HelpView()))
	return s.String()
}

func (m modelUI) headerView() string {
	if m.prompt.Focused() {
		return m.prompt.WithWidth(m.layout.GetWidth()).View()
	}
	if m.periodPicker.Focused() {
		return m.periodPicker.WithWidth(m.layout.GetWidth()).View()
	}

	header := " ezbk-tui"
	renderer := m.styles.Prompt
	if m.state == formView {
		if m.form.new {
			header += " | New transaction"
			renderer = m.styles.PromptNewTr
		} else {
			header += " | Editing transaction " + m.form.data.id
			renderer = m.styles.PromptEditTr
		}
	} else {
		header += m.filterSummary()
	}
	if isLoading() {
		header += " | " + m.spinner.View() + buildLoadingMessage()
	}
	return renderer.Width(m.Width).Render(header)
}

func (m modelUI) filterSummary() string {
	t := m.transactions
	s := ""
	if t.currentSearch != "" {
		s += " | Search: " + t.currentSearch
	} else {
		start := m.api.PeriodStart()
		s += fmt.Sprintf(" | p %s %d", start.Month(), start.Year())
	}
	if !t.filter.account.IsEmpty() {
		s += " | Account: " + t.filter.account.DisplayName()
	}
	if !t.filter.category.IsEmpty() {
		s += " | Category: " + t.filter.category.DisplayName()
	}
	if !t.filter.tag.IsEmpty() {
		s += " | Tag: #" + t.filter.tag.Name
	}
	if t.filter.query != "" {
		s += " | Filter: " + t.filter.query
	}
	return s
}

func (m modelUI) bodyView() string {
	left := func(focused bool, parts ...string) string {
		style := m.styles.Base
		if focused {
			style = m.styles.BaseFocused
		}
		return style.Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
	}
	right := func(focused bool, content string) string {
		if focused {
			return m.styles.BaseFocused.Render(content)
		}
		return m.styles.Base.Render(content)
	}
	txs := m.transactions.View()

	switch m.state {
	case transactionsView:
		if m.layout.GetFullTransactionView() {
			return right(true, txs)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top,
			left(false, m.tabBar(), m.summary.View(), m.accounts.View()), right(true, txs))
	case accountsView:
		return lipgloss.JoinHorizontal(lipgloss.Top,
			left(true, m.tabBar(), m.summary.View(), m.accounts.View()), right(false, txs))
	case expenseView:
		return lipgloss.JoinHorizontal(lipgloss.Top, left(true, m.tabBar(), m.expense.View()), right(false, txs))
	case incomeView:
		return lipgloss.JoinHorizontal(lipgloss.Top, left(true, m.tabBar(), m.income.View()), right(false, txs))
	case tagsView:
		return lipgloss.JoinHorizontal(lipgloss.Top, left(true, m.tabBar(), m.tags.View()), right(false, txs))
	case templatesView:
		return lipgloss.JoinHorizontal(lipgloss.Top, left(true, m.templates.View()), right(false, txs))
	case formView:
		return lipgloss.JoinHorizontal(lipgloss.Top,
			left(false, m.summary.View(), m.accounts.View()), right(true, m.form.View()))
	}
	return ""
}

func (m *modelUI) HelpView() string {
	var view string
	switch m.state {
	case transactionsView:
		view = m.help.View(m.transactions.keymap)
	case accountsView:
		view = m.help.View(m.accounts.keymap)
	case expenseView:
		view = m.help.View(m.expense.keymap)
	case incomeView:
		view = m.help.View(m.income.keymap)
	case tagsView:
		view = m.help.View(m.tags.keymap)
	case templatesView:
		view = m.help.View(m.templates.keymap)
	case formView:
		view = m.help.View(m.form.keymap)
	}
	if m.help.ShowAll {
		view = lipgloss.JoinHorizontal(lipgloss.Left, view, m.help.View(m.keymap))
	}
	return view
}

func (m *modelUI) tabBar() string {
	tabs := []struct {
		key, label string
		state      state
	}{
		{"a", "Accounts", accountsView},
		{"e", "Expense", expenseView},
		{"i", "Income", incomeView},
		{"g", "Tags", tagsView},
	}
	parts := make([]string, 0, len(tabs))
	for _, t := range tabs {
		label := t.key + " " + t.label
		active := m.state == t.state || (m.state == transactionsView && t.state == accountsView)
		if active {
			parts = append(parts, m.styles.TabActive.Render(label))
		} else {
			parts = append(parts, m.styles.TabInactive.Render(label))
		}
	}
	return strings.Join(parts, m.styles.TabInactive.Render(" ")) + "\n"
}
