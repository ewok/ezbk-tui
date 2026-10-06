/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/notify"
	"ezbk-tui/internal/ui/period"
	"ezbk-tui/internal/ui/prompt"
)

type state uint

const (
	transactionsView state = iota
	formView
	accountsView
	expenseView
	incomeView
	tagsView
	templatesView
)

type (
	ViewFullTransactionViewMsg struct{}
	SetFocusedViewMsg          struct{ state state }
	DataLoadCompletedMsg       struct{ DataType string }
	LazyLoadMsg                struct{ c int }
	RefreshAllMsg              struct{}
	UpdatePositions            struct{ layout *LayoutConfig }
)

var baseDataTypes = []string{"accounts", "categories", "tags"}

type modelUI struct {
	state        state
	api          UIAPI
	transactions modelTransactions
	form         modelTransaction
	accounts     modelAccounts
	expense      modelCategories
	income       modelCategories
	tags         modelTags
	templates    modelTemplates
	summary      modelSummary
	prompt       prompt.Model
	periodPicker period.Model
	notify       notify.Model
	spinner      spinner.Model

	keymap UIKeyMap
	help   help.Model
	styles Styles

	Width  int
	layout *LayoutConfig

	loadStatus    map[string]bool
	convertTotals bool
}

// Show runs the TUI until the user quits.
func Show(api UIAPI) {
	if _, err := tea.NewProgram(NewModelUI(api)).Run(); err != nil {
		zap.L().Error("error running program", zap.Error(err))
		fmt.Println("Error running program:", err)
	}
}

func NewModelUI(api UIAPI) modelUI {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := modelUI{
		api:          api,
		transactions: NewModelTransactions(api),
		form:         newModelTransaction(api),
		accounts:     newModelAccounts(api),
		expense:      newModelCategories(api, domain.CategoryExpense, expenseView, true),
		income:       newModelCategories(api, domain.CategoryIncome, incomeView, false),
		tags:         newModelTags(api),
		templates:    newModelTemplates(api),
		summary:      newModelSummary(api),
		prompt:       prompt.New(),
		periodPicker: period.New(),
		notify:       notify.New(),
		spinner:      sp,
		keymap:       DefaultUIKeyMap(),
		help:         help.New(),
		styles:       DefaultStyles(),
		Width:        80,
		layout:       NewDefaultLayout().WithFullTransactionView(viper.GetBool("ui.full_view")),
		loadStatus:   newLoadStatus(),
	}
	m.setConvertTotals(convertTotalsSetting())
	m.help.Styles.FullKey = m.styles.HelpFullKey
	m.help.Styles.ShortKey = m.styles.HelpShortKey
	return m
}

func newLoadStatus() map[string]bool {
	s := map[string]bool{}
	for _, t := range baseDataTypes {
		s[t] = false
	}
	return s
}

func (m modelUI) Init() tea.Cmd {
	return tea.Batch(Cmd(RefreshAllMsg{}), m.spinner.Tick)
}

func updateModel[T tea.Model](current T, msg tea.Msg) (T, tea.Cmd) {
	model, cmd := current.Update(msg)
	if converted, ok := model.(T); ok {
		return converted, cmd
	}
	zap.S().Errorf("Failed to update model: type assertion failed for %T", current)
	return current, cmd
}

func (m modelUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	handled, model, cmd := m.handleGlobal(msg)
	if handled {
		return model, cmd
	}
	next, ok := model.(modelUI)
	if !ok {
		return m, nil
	}
	return next.dispatch(msg)
}

// handleGlobal processes messages owned by the root model.
func (m modelUI) handleGlobal(msg tea.Msg) (bool, tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleGlobalKey(msg)
	case period.SelectedMsg:
		m.transactions.currentSearch = ""
		m.api.SetPeriod(msg.Year, msg.Month)
		return true, m, tea.Batch(Cmd(RefreshTransactionsMsg{}), Cmd(RefreshStatsMsg{}))
	case UpdatePositions:
		m.updatePositions(msg)
	case tea.WindowSizeMsg:
		return true, m, Cmd(UpdatePositions{layout: m.layout.WithSize(msg.Width, msg.Height)})
	case SetFocusedViewMsg:
		m.setFocus(msg.state)
		return true, m, Cmd(UpdatePositions{layout: m.layout})
	case ViewFullTransactionViewMsg:
		viper.Set("ui.full_view", m.layout.ToggleFullTransactionView())
		return true, m, Cmd(UpdatePositions{layout: m.layout})
	case DataLoadCompletedMsg:
		m.loadStatus[msg.DataType] = true
	case LazyLoadMsg:
		return true, m, m.lazyLoad(msg)
	case RefreshAllMsg:
		m.loadStatus = newLoadStatus()
		return true, m, tea.Batch(
			SetView(transactionsView),
			tea.WindowSize(),
			Cmd(RefreshAccountsMsg{}),
			Cmd(RefreshCategoriesMsg{}),
			Cmd(RefreshTagsMsg{}),
			Cmd(RefreshTemplatesMsg{}),
			tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
				return LazyLoadMsg{c: m.api.TimeoutSeconds() * 10}
			}),
		)
	}
	return false, m, nil
}

func (m modelUI) handleGlobalKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Quit):
		return true, m, tea.Quit
	case key.Matches(msg, m.keymap.ShowShortHelp) && !m.isAnyInputFocused():
		m.help.ShowAll = !m.help.ShowAll
		return true, m, tea.WindowSize()
	case key.Matches(msg, m.keymap.PeriodPicker) && !m.isAnyInputFocused():
		start := m.api.PeriodStart()
		return true, m, period.Open(start.Year(), start.Month())
	case key.Matches(msg, m.keymap.ToggleCurrency) && !m.isAnyInputFocused() && !m.periodPicker.Focused():
		m.setConvertTotals(!m.convertTotals)
		viper.Set(convertTotalsKey, m.convertTotals)
		return true, m, Cmd(CurrencyModeMsg{Converted: m.convertTotals})
	}
	return false, m, nil
}

// setConvertTotals sets the initial mode on the root and the panels; later
// changes reach the panels through CurrencyModeMsg.
func (m *modelUI) setConvertTotals(v bool) {
	m.convertTotals = v
	m.summary.converted = v
	m.expense.converted = v
	m.income.converted = v
}

// lazyLoad waits for base data (needed to resolve names) before loading transactions.
func (m modelUI) lazyLoad(msg LazyLoadMsg) tea.Cmd {
	for _, loaded := range m.loadStatus {
		if loaded {
			continue
		}
		if msg.c <= 0 {
			return tea.Batch(
				notify.NotifyWarn("Could not load all resources"),
				Cmd(RefreshTransactionsMsg{}),
				Cmd(RefreshStatsMsg{}),
			)
		}
		return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
			return LazyLoadMsg{c: msg.c - 1}
		})
	}
	return tea.Batch(Cmd(RefreshTransactionsMsg{}), Cmd(RefreshStatsMsg{}))
}

func (m *modelUI) dispatch(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.prompt, cmd = updateModel(m.prompt, msg)
	cmds = append(cmds, cmd)
	if m.prompt.Focused() && !isDataMsg(msg) {
		return *m, tea.Batch(cmds...)
	}

	pickerFocused := m.periodPicker.Focused()
	m.periodPicker, cmd = updateModel(m.periodPicker, msg)
	cmds = append(cmds, cmd)
	if pickerFocused && !isDataMsg(msg) {
		return *m, tea.Batch(cmds...)
	}

	if _, isKey := msg.(tea.KeyMsg); isKey && m.form.Focused() {
		m.form, cmd = updateModel(m.form, msg)
		return *m, tea.Batch(append(cmds, cmd)...)
	}

	m.notify, cmd = updateModel(m.notify, msg)
	cmds = append(cmds, cmd)
	m.summary, cmd = updateModel(m.summary, msg)
	cmds = append(cmds, cmd)
	m.transactions, cmd = updateModel(m.transactions, msg)
	cmds = append(cmds, cmd)
	m.accounts, cmd = updateModel(m.accounts, msg)
	cmds = append(cmds, cmd)
	m.expense, cmd = updateModel(m.expense, msg)
	cmds = append(cmds, cmd)
	m.income, cmd = updateModel(m.income, msg)
	cmds = append(cmds, cmd)
	m.tags, cmd = updateModel(m.tags, msg)
	cmds = append(cmds, cmd)
	m.templates, cmd = updateModel(m.templates, msg)
	cmds = append(cmds, cmd)
	m.form, cmd = updateModel(m.form, msg)
	cmds = append(cmds, cmd)
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return *m, tea.Batch(cmds...)
}

func (m *modelUI) setFocus(s state) {
	m.transactions.Blur()
	m.accounts.Blur()
	m.expense.Blur()
	m.income.Blur()
	m.tags.Blur()
	m.templates.Blur()
	m.form.Blur()
	switch s {
	case transactionsView:
		m.transactions.Focus()
	case accountsView:
		m.accounts.Focus()
	case expenseView:
		m.expense.Focus()
	case incomeView:
		m.income.Focus()
	case tagsView:
		m.tags.Focus()
	case templatesView:
		m.templates.Focus()
	case formView:
		m.form.Focus()
	}
	m.state = s
}

func (m *modelUI) updatePositions(msg UpdatePositions) {
	h, _ := m.styles.Base.GetFrameSize()
	width := m.layout.Width
	if msg.layout != nil && msg.layout.Width != 0 {
		width = msg.layout.Width
	}
	m.Width = width - h

	topSize := 5
	if m.help.ShowAll {
		topSize += lipgloss.Height(m.HelpView())
	}

	tabBarSize := 2
	tabBarWidth := lipgloss.Width(m.tabBar())
	leftSize := 0
	switch m.state {
	case transactionsView, accountsView:
		if m.layout.GetFullTransactionView() && m.state == transactionsView {
			tabBarSize = 0
		} else {
			leftSize = max(lipgloss.Width(m.accounts.View()), lipgloss.Width(m.summary.View()), tabBarWidth) + h
		}
	case expenseView:
		leftSize = max(lipgloss.Width(m.expense.View()), tabBarWidth) + h
	case incomeView:
		leftSize = max(lipgloss.Width(m.income.View()), tabBarWidth) + h
	case tagsView:
		leftSize = max(lipgloss.Width(m.tags.View()), tabBarWidth) + h
	case templatesView:
		leftSize = lipgloss.Width(m.templates.View()) + h
	case formView:
		leftSize = max(lipgloss.Width(m.accounts.View()), lipgloss.Width(m.summary.View())) + h
	}
	m.layout = m.layout.WithTopSize(topSize).WithLeftSize(leftSize)
	m.layout.TabBarSize = tabBarSize
}

func (m *modelUI) isAnyInputFocused() bool {
	return m.prompt.Focused() ||
		m.form.Focused() ||
		m.accounts.filterInputFocused() ||
		m.expense.filterInputFocused() ||
		m.income.filterInputFocused() ||
		m.tags.filterInputFocused() ||
		m.templates.list.SettingFilter()
}

// SetView focuses a view.
func SetView(s state) tea.Cmd {
	return Cmd(SetFocusedViewMsg{state: s})
}
