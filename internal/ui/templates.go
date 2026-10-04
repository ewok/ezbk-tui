/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/notify"
)

type (
	RefreshTemplatesMsg struct{}
	TemplatesUpdatedMsg struct{}
)

type templateItem struct {
	template domain.Template
}

func (i templateItem) Title() string { return i.template.Name }
func (i templateItem) Description() string {
	tx := i.template.Transaction
	desc := fmt.Sprintf("%s %s · %s", tx.Type.Icon(), tx.SourceAmount.Format(tx.Source.Currency), tx.Source.DisplayName())
	if tx.Type == domain.TxTransfer {
		desc += " -> " + tx.Destination.DisplayName()
	}
	if !tx.Category.IsEmpty() {
		desc += " · " + tx.Category.DisplayName()
	}
	return desc
}
func (i templateItem) FilterValue() string { return i.template.Name }

type modelTemplates struct {
	list   list.Model
	api    TemplatesAPI
	focus  bool
	keymap TemplatesKeyMap
	styles Styles
}

func newModelTemplates(api TemplatesAPI) modelTemplates {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Templates"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.SetStatusBarItemName("template", "templates")
	return modelTemplates{list: l, api: api, keymap: DefaultTemplatesKeyMap(), styles: DefaultStyles()}
}

func (m modelTemplates) Init() tea.Cmd { return nil }

func (m modelTemplates) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshTemplatesMsg:
		api := m.api
		return m, func() tea.Msg {
			opID := startLoading("Loading templates...")
			defer stopLoading(opID)
			if err := api.UpdateTemplates(); err != nil {
				return notify.NotifyWarn(err.Error())()
			}
			return TemplatesUpdatedMsg{}
		}
	case TemplatesUpdatedMsg, AccountsUpdatedMsg, CategoriesUpdatedMsg:
		items := []list.Item{}
		for _, t := range m.api.Templates() {
			items = append(items, templateItem{template: t})
		}
		return m, m.list.SetItems(items)
	case UpdatePositions:
		if msg.layout != nil {
			h, v := m.styles.Base.GetFrameSize()
			m.list.SetSize(max(1, msg.layout.Width-h), max(1, msg.layout.Height-v-msg.layout.TopSize))
		}
		return m, nil
	}

	if !m.focus {
		return m, nil
	}

	var cmd tea.Cmd
	if m.list.SettingFilter() {
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, m.keymap.Quit):
			if m.list.IsFiltered() {
				m.list.ResetFilter()
				return m, nil
			}
			return m, SetView(transactionsView)
		case key.Matches(km, m.keymap.Refresh):
			return m, Cmd(RefreshTemplatesMsg{})
		case key.Matches(km, m.keymap.Select):
			i, ok := m.list.SelectedItem().(templateItem)
			if !ok {
				return m, notify.NotifyWarn("No template selected (create templates in ezBookkeeping)")
			}
			return m, Cmd(NewTransactionMsg{Transaction: copyForNew(i.template.Transaction)})
		}
	}
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m modelTemplates) View() string {
	return m.styles.LeftPanel.Render(m.list.View())
}

func (m *modelTemplates) Focus() { m.focus = true }
func (m *modelTemplates) Blur()  { m.focus = false }
