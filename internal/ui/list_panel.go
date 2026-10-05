/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// listPanel is the common part of the left-side list views.
type listPanel struct {
	list        list.Model
	focus       bool
	sorted      bool
	withSummary bool
	keymap      ListKeyMap
	styles      Styles
}

// listActions are the item specific callbacks of a panel.
type listActions struct {
	refresh  func() tea.Cmd
	resort   func() tea.Cmd
	create   func(selected list.Item) tea.Cmd
	filterBy func(selected list.Item) tea.Cmd
}

func newListPanel(title, item string) listPanel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.FilterInput.Blur()
	l.FilterInput.Width = 20
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return listPanel{
		list:   l,
		keymap: DefaultListKeyMap(item),
		styles: DefaultStyles(),
	}
}

func (p *listPanel) resize(layout *LayoutConfig) {
	if layout == nil {
		return
	}
	h, v := p.styles.Base.GetFrameSize()
	height := layout.Height - v - layout.TopSize - layout.TabBarSize
	if p.withSummary {
		height -= layout.SummarySize
	}
	p.list.SetSize(max(1, layout.Width-h), max(1, height))
	p.list.FilterInput.Width = 20
}

// update handles keys common to all panels. It must only be called when focused.
func (p *listPanel) update(msg tea.Msg, actions listActions) tea.Cmd {
	var cmd tea.Cmd
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, p.keymap.Filter) && !p.list.FilterInput.Focused():
			p.list.FilterInput.Focus()
		case key.Matches(km, p.keymap.Quit):
			if !p.list.FilterInput.Focused() {
				return SetView(transactionsView)
			}
			p.list.FilterInput.Blur()
		}
	}
	if p.filterActive() {
		p.list, cmd = p.list.Update(msg)
		return cmd
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		if cmd := p.keymap.viewSwitch(km); cmd != nil {
			return cmd
		}
		switch {
		case key.Matches(km, p.keymap.Refresh):
			return actions.refresh()
		case key.Matches(km, p.keymap.ResetFilter):
			return Cmd(FilterMsg{Reset: true})
		case key.Matches(km, p.keymap.Sort):
			p.sorted = !p.sorted
			return actions.resort()
		case key.Matches(km, p.keymap.New):
			return actions.create(p.list.SelectedItem())
		case key.Matches(km, p.keymap.FilterBy):
			return actions.filterBy(p.list.SelectedItem())
		case key.Matches(km, p.keymap.Select):
			cmd := actions.filterBy(p.list.SelectedItem())
			if cmd == nil {
				return nil
			}
			return tea.Sequence(cmd, SetView(transactionsView))
		}
	}
	p.list, cmd = p.list.Update(msg)
	return cmd
}

func (p listPanel) View() string {
	return p.styles.LeftPanel.Render(p.list.View())
}

func (p *listPanel) Focus() { p.focus = true }
func (p *listPanel) Blur()  { p.focus = false }

// filterActive reports whether keys currently go to the list filter.
func (p *listPanel) filterActive() bool {
	return p.list.FilterInput.Focused() || p.list.SettingFilter()
}

// filterInputFocused reports whether the list's filter input is active.
func (p *listPanel) filterInputFocused() bool {
	return p.list.FilterInput.Focused()
}

// simpleItem is a list item with fixed strings, used for total rows.
type simpleItem struct {
	title, desc string
}

func (i simpleItem) Title() string       { return i.title }
func (i simpleItem) Description() string { return i.desc }
func (i simpleItem) FilterValue() string { return "" }
