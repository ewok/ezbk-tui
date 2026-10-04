/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
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
	RefreshTagsMsg struct{}
	TagsUpdatedMsg struct{}
	NewTagMsg      struct{ Name string }
	TagCreatedMsg  struct{ Name string }
)

type tagItem struct {
	tag      domain.Tag
	spent    domain.Amounts
	earned   domain.Amounts
	currency string
}

func (i tagItem) Title() string { return "#" + i.tag.Name }
func (i tagItem) Description() string {
	parts := []string{}
	if !i.spent.IsZero() {
		parts = append(parts, "Spent: "+i.spent.Format(i.currency))
	}
	if !i.earned.IsZero() {
		parts = append(parts, "Earned: "+i.earned.Format(i.currency))
	}
	if len(parts) == 0 {
		return "No transactions"
	}
	return strings.Join(parts, " | ")
}
func (i tagItem) FilterValue() string { return i.tag.Name }

type modelTags struct {
	listPanel
	api TagsAPI
}

func newModelTags(api TagsAPI) modelTags {
	return modelTags{listPanel: newListPanel("Tags", "tag"), api: api}
}

func (m modelTags) Init() tea.Cmd { return nil }

func (m modelTags) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshTagsMsg:
		return m, m.refreshCmd()
	case TagsUpdatedMsg:
		return m, tea.Batch(
			m.list.SetItems(tagItems(m.api, m.sorted)),
			Cmd(DataLoadCompletedMsg{DataType: "tags"}),
		)
	case StatsUpdatedMsg:
		return m, m.list.SetItems(tagItems(m.api, m.sorted))
	case NewTagMsg:
		api := m.api
		return m, func() tea.Msg {
			opID := startLoading("Creating tag...")
			defer stopLoading(opID)
			if err := api.CreateTag(msg.Name); err != nil {
				return notify.NotifyWarn(err.Error())()
			}
			return TagCreatedMsg(msg)
		}
	case TagCreatedMsg:
		return m, tea.Batch(
			Cmd(RefreshTagsMsg{}),
			notify.NotifyLog(fmt.Sprintf("Tag '%s' created", msg.Name)),
		)
	case UpdatePositions:
		m.resize(msg.layout)
		return m, nil
	}

	if !m.focus {
		return m, nil
	}
	cmd := m.update(msg, listActions{
		refresh: func() tea.Cmd { return Cmd(RefreshTagsMsg{}) },
		resort:  func() tea.Cmd { return Cmd(TagsUpdatedMsg{}) },
		create:  func(list.Item) tea.Cmd { return CmdPromptNewTag(SetView(tagsView)) },
		filterBy: func(selected list.Item) tea.Cmd {
			if i, ok := selected.(tagItem); ok {
				return Cmd(FilterMsg{Tag: i.tag})
			}
			return nil
		},
	})
	return m, cmd
}

func (m modelTags) refreshCmd() tea.Cmd {
	api := m.api
	return func() tea.Msg {
		opID := startLoading("Loading tags...")
		defer stopLoading(opID)
		if err := api.UpdateTags(); err != nil {
			return tea.Batch(
				notify.NotifyWarn(err.Error()),
				Cmd(DataLoadCompletedMsg{DataType: "tags"}),
			)()
		}
		return TagsUpdatedMsg{}
	}
}

func tagItems(api TagsAPI, sorted bool) []list.Item {
	currency := api.DefaultCurrency()
	items := []list.Item{}
	for _, tag := range api.Tags() {
		spent, earned := api.TagTotals(tag.ID)
		if sorted && spent.IsZero() && earned.IsZero() {
			continue
		}
		items = append(items, tagItem{tag: tag, spent: spent, earned: earned, currency: currency})
	}
	if sorted {
		slices.SortStableFunc(items, func(a, b list.Item) int {
			return int(b.(tagItem).spent.Get(currency) - a.(tagItem).spent.Get(currency))
		})
	}
	return items
}

// CmdPromptNewTag asks for a tag name.
func CmdPromptNewTag(backCmd tea.Cmd) tea.Cmd {
	return prompt.Ask("New tag (<name>): ", "", func(value string) tea.Cmd {
		if value == "None" {
			return backCmd
		}
		return tea.Sequence(Cmd(NewTagMsg{Name: strings.TrimPrefix(value, "#")}), backCmd)
	})
}
