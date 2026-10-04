/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.uber.org/zap"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/notify"
)

type (
	RefreshStatsMsg  struct{}
	StatsUpdatedMsg  struct{}
	SummaryUpdateMsg struct{}
)

type summaryItem struct {
	title, value string
	style        lipgloss.Style
}

func (i summaryItem) FilterValue() string { return i.title }

type summaryDelegate struct{}

func (d summaryDelegate) Height() int                             { return 1 }
func (d summaryDelegate) Spacing() int                            { return 0 }
func (d summaryDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d summaryDelegate) Render(w io.Writer, m list.Model, _ int, listItem list.Item) {
	i, ok := listItem.(summaryItem)
	if !ok {
		return
	}
	valueLen := utf8.RuneCountInString(i.value)
	titleLen := utf8.RuneCountInString(i.title)
	spacing := max(m.Width()+4-titleLen-valueLen, 1)
	str := fmt.Sprintf(" %s%s%s", i.title, strings.Repeat(" ", spacing), i.style.Render(i.value))
	if _, err := fmt.Fprint(w, str); err != nil {
		zap.L().Error("failed to render summary item", zap.Error(err))
	}
}

type modelSummary struct {
	list   list.Model
	api    SummaryAPI
	styles Styles
}

func newModelSummary(api SummaryAPI) modelSummary {
	m := modelSummary{
		list:   list.New([]list.Item{}, summaryDelegate{}, 0, 0),
		api:    api,
		styles: DefaultStyles(),
	}
	m.list.Title = "Summary"
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(false)
	m.list.SetShowHelp(false)
	m.list.DisableQuitKeybindings()
	m.list.SetShowPagination(false)
	m.list.SetWidth(28)
	return m
}

func (m modelSummary) Init() tea.Cmd { return nil }

func (m modelSummary) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshStatsMsg:
		api := m.api
		return m, func() tea.Msg {
			opID := startLoading("Loading period totals...")
			defer stopLoading(opID)
			if err := api.UpdatePeriodStats(); err != nil {
				return notify.NotifyWarn(err.Error())()
			}
			return StatsUpdatedMsg{}
		}
	case StatsUpdatedMsg, SummaryUpdateMsg:
		items := summaryItems(m.api, m.styles)
		m.list.SetWidth(summaryWidth(items))
		return m, tea.Sequence(m.list.SetItems(items), tea.WindowSize())
	case UpdatePositions:
		if msg.layout != nil {
			_, v := m.styles.Base.GetFrameSize()
			m.list.SetHeight(len(m.list.Items()) + v + 1)
			msg.layout.SummarySize = m.list.Height()
		}
	}
	return m, nil
}

func (m modelSummary) View() string {
	return m.styles.LeftPanel.Render(m.list.View())
}

func summaryWidth(items []list.Item) int {
	width := 24
	for _, it := range items {
		si := it.(summaryItem)
		width = max(width, utf8.RuneCountInString(si.title)+utf8.RuneCountInString(si.value)+2)
	}
	return width
}

func summaryItems(api SummaryAPI, styles Styles) []list.Item {
	primary := api.DefaultCurrency()
	assets, liabilities := api.NetWorth()
	income, expense := api.PeriodIncome(), api.PeriodExpense()

	styled := func(v domain.Money) lipgloss.Style {
		switch {
		case v < 0:
			return styles.Expense
		case v > 0:
			return styles.Income
		}
		return styles.Normal
	}

	items := []list.Item{}
	netWorth := domain.Amounts{}
	for c, v := range assets {
		netWorth.Add(c, v)
	}
	for c, v := range liabilities {
		netWorth.Add(c, v)
	}
	for _, c := range netWorth.Currencies(primary) {
		v := netWorth[c]
		items = append(items, summaryItem{title: "Net worth " + c, value: v.String(), style: styled(v)})
	}

	currencies := domain.Amounts{}
	for c := range income {
		currencies[c] = 0
	}
	for c := range expense {
		currencies[c] = 0
	}
	if len(currencies) == 0 && primary != "" {
		currencies[primary] = 0
	}
	for _, c := range currencies.Currencies(primary) {
		in, out := income[c], expense[c]
		items = append(items,
			summaryItem{title: "Income " + c, value: in.String(), style: styles.Income},
			summaryItem{title: "Expense " + c, value: (-out).String(), style: styles.Expense},
			summaryItem{title: "Balance " + c, value: (in - out).String(), style: styled(in - out)},
		)
	}
	return items
}
