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
	list      list.Model
	api       SummaryAPI
	styles    Styles
	converted bool
	warned    string // last missing-rate set we warned about
}

func newModelSummary(api SummaryAPI) modelSummary {
	m := modelSummary{
		list:      list.New([]list.Item{}, summaryDelegate{}, 0, 0),
		api:       api,
		styles:    DefaultStyles(),
		converted: true,
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
	case CurrencyModeMsg:
		m.converted = msg.Converted
		return m.rebuild()
	case StatsUpdatedMsg, SummaryUpdateMsg:
		return m.rebuild()
	case UpdatePositions:
		if msg.layout != nil {
			_, v := m.styles.Base.GetFrameSize()
			m.list.SetHeight(len(m.list.Items()) + v + 1)
			msg.layout.SummarySize = m.list.Height()
		}
	}
	return m, nil
}

// rebuild recomputes the rows and warns once per new set of currencies
// that have no exchange rate. No warning is shown while rates are not
// loaded yet (startup, or the rates request failed).
func (m modelSummary) rebuild() (tea.Model, tea.Cmd) {
	items, missing := summaryItems(m.api, m.styles, m.converted)
	m.list.SetWidth(summaryWidth(items))
	cmds := []tea.Cmd{m.list.SetItems(items), tea.WindowSize()}
	if m.api.ExchangeRates().IsEmpty() {
		return m, tea.Sequence(cmds...)
	}
	if key := strings.Join(missing, ","); key != m.warned {
		m.warned = key
		if key != "" {
			cmds = append(cmds, notify.NotifyWarn(missingRatesWarning(missing)))
		}
	}
	return m, tea.Sequence(cmds...)
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

// summaryItems builds the summary rows. In converted mode every total is shown
// once in the default currency; amounts without an exchange rate follow as
// per-currency rows and are returned as missing.
func summaryItems(api SummaryAPI, styles Styles, convert bool) ([]list.Item, []string) {
	primary := api.DefaultCurrency()
	assets, liabilities := api.NetWorth()
	netWorth := domain.Amounts{}
	for _, part := range []domain.Amounts{assets, liabilities} {
		for c, v := range part {
			netWorth.Add(c, v)
		}
	}
	income, expense := api.PeriodIncome(), api.PeriodExpense()
	r := summaryRows{styles: styles, primary: primary}

	if !convert {
		items := r.netWorth(netWorth)
		return append(items, r.period(income, expense, true)...), nil
	}

	nw := convertAmounts(netWorth, api)
	in, out := convertAmounts(income, api), convertAmounts(expense, api)
	balance := converted{total: in.total - out.total, left: missingAmounts(in, out), currency: in.currency}

	items := []list.Item{summaryItem{title: "Net worth", value: nw.value(nw.total), style: r.signed(nw.total)}}
	items = append(items, r.netWorth(nw.left)...)
	items = append(items,
		summaryItem{title: "Income", value: in.value(in.total), style: styles.Income},
		summaryItem{title: "Expense", value: out.value(-out.total), style: styles.Expense},
		summaryItem{title: "Balance", value: balance.value(balance.total), style: r.signed(balance.total)},
	)
	items = append(items, r.period(in.left, out.left, false)...)
	return items, missingCurrencies(nw.left, in.left, out.left)
}

// missingAmounts merges the unconverted parts, used only to mark a total as partial.
func missingAmounts(parts ...converted) domain.Amounts {
	out := domain.Amounts{}
	for _, p := range parts {
		for c, v := range p.left {
			out.Add(c, v)
		}
	}
	return out
}

// summaryRows renders per-currency rows.
type summaryRows struct {
	styles  Styles
	primary string
}

func (r summaryRows) signed(v domain.Money) lipgloss.Style {
	switch {
	case v < 0:
		return r.styles.Expense
	case v > 0:
		return r.styles.Income
	}
	return r.styles.Normal
}

func (r summaryRows) netWorth(a domain.Amounts) []list.Item {
	items := []list.Item{}
	for _, c := range a.Currencies(r.primary) {
		v := a[c]
		items = append(items, summaryItem{title: "Net worth " + c, value: v.String(), style: r.signed(v)})
	}
	return items
}

// period renders Income/Expense/Balance per currency; showPrimary adds an
// empty primary-currency block when there are no amounts at all.
func (r summaryRows) period(income, expense domain.Amounts, showPrimary bool) []list.Item {
	currencies := domain.Amounts{}
	for _, part := range []domain.Amounts{income, expense} {
		for c := range part {
			currencies[c] = 0
		}
	}
	if showPrimary && len(currencies) == 0 && r.primary != "" {
		currencies[r.primary] = 0
	}
	items := []list.Item{}
	for _, c := range currencies.Currencies(r.primary) {
		in, out := income[c], expense[c]
		items = append(items,
			summaryItem{title: "Income " + c, value: in.String(), style: r.styles.Income},
			summaryItem{title: "Expense " + c, value: (-out).String(), style: r.styles.Expense},
			summaryItem{title: "Balance " + c, value: (in - out).String(), style: r.signed(in - out)},
		)
	}
	return items
}
