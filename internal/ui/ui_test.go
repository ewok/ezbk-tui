/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ezbk-tui/internal/ui/period"
)

func newReadyUI(t *testing.T, api *mockAPI) modelUI {
	t.Helper()
	m := NewModelUI(api)
	steps := []tea.Msg{
		AccountsUpdatedMsg{}, CategoriesUpdatedMsg{}, TagsUpdatedMsg{}, StatsUpdatedMsg{},
		TransactionsUpdateMsg{Transactions: api.txs},
		SetFocusedViewMsg{state: transactionsView},
		UpdatePositions{layout: m.layout.WithSize(160, 40)},
	}
	for _, msg := range steps {
		m = mustUpdate(t, m, msg)
	}
	return m
}

func mustUpdate(t *testing.T, m modelUI, msg tea.Msg) modelUI {
	t.Helper()
	next, _ := m.Update(msg)
	mm, ok := next.(modelUI)
	if !ok {
		t.Fatalf("Update returned %T", next)
	}
	return mm
}

func TestUI_ViewsRender(t *testing.T) {
	api := newMockAPI()
	m := newReadyUI(t, api)
	tests := []struct {
		state state
		want  []string
	}{
		{transactionsView, []string{"p October 2026", "Summary", "Accounts", "milk", "a Accounts"}},
		{accountsView, []string{"Wallet", "Bank / EUR"}},
		{expenseView, []string{"Expense categories", "Groceries"}},
		{incomeView, []string{"Income categories", "Main job"}},
		{tagsView, []string{"#trip", "#work"}},
		{templatesView, []string{"Templates"}},
		{formView, []string{"New transaction"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.want[:1], ""), func(t *testing.T) {
			mm := mustUpdate(t, m, SetFocusedViewMsg{state: tt.state})
			if tt.state == formView {
				mm.form.SetTransaction(api.txs[0], true)
				mm.form.UpdateForm()
			}
			view := mm.View()
			for _, w := range tt.want {
				if !strings.Contains(view, w) {
					t.Errorf("view for state %d misses %q", tt.state, w)
				}
			}
		})
	}
}

func TestUI_TabKeysSwitchView(t *testing.T) {
	m := newReadyUI(t, newMockAPI())
	tests := []struct {
		key  string
		want state
	}{
		{"a", accountsView},
		{"e", expenseView},
		{"i", incomeView},
		{"g", tagsView},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			_, cmd := m.Update(keyMsg(tt.key))
			v, ok := findMsg[SetFocusedViewMsg](runCmd(cmd))
			if !ok || v.state != tt.want {
				t.Errorf("key %s -> %+v", tt.key, v)
			}
		})
	}
}

func TestUI_PeriodSelected(t *testing.T) {
	api := newMockAPI()
	m := newReadyUI(t, api)
	m.transactions.currentSearch = "x"
	next, cmd := m.Update(period.SelectedMsg{Year: 2026, Month: time.March})
	mm := next.(modelUI)
	if api.start.Month() != time.March || mm.transactions.currentSearch != "" {
		t.Errorf("period = %v search = %q", api.start, mm.transactions.currentSearch)
	}
	msgs := runCmd(cmd)
	if !hasMsg[RefreshTransactionsMsg](msgs) || !hasMsg[RefreshStatsMsg](msgs) {
		t.Errorf("period change should refresh: %v", msgs)
	}
}

func TestUI_LazyLoadWaitsForBaseData(t *testing.T) {
	api := newMockAPI()
	m := NewModelUI(api)
	cmd := m.lazyLoad(LazyLoadMsg{c: 0})
	if !hasMsg[RefreshTransactionsMsg](runCmd(cmd)) {
		t.Error("expired lazy load should still load transactions")
	}
	for _, dt := range baseDataTypes {
		m = mustUpdate(t, m, DataLoadCompletedMsg{DataType: dt})
	}
	msgs := runCmd(m.lazyLoad(LazyLoadMsg{c: 5}))
	if !hasMsg[RefreshTransactionsMsg](msgs) || !hasMsg[RefreshStatsMsg](msgs) {
		t.Errorf("loaded base data should trigger transactions+stats: %v", msgs)
	}
}

func TestUI_HeaderShowsFilters(t *testing.T) {
	m := newReadyUI(t, newMockAPI())
	m = mustUpdate(t, m, FilterMsg{Category: catGroceries})
	m = mustUpdate(t, m, FilterMsg{Tag: tagTrip})
	header := m.headerView()
	for _, want := range []string{"Category: Food / Groceries", "Tag: #trip"} {
		if !strings.Contains(header, want) {
			t.Errorf("header %q misses %q", header, want)
		}
	}
}

func TestUI_FormKeysDoNotLeak(t *testing.T) {
	api := newMockAPI()
	m := newReadyUI(t, api)
	m = mustUpdate(t, m, LoadTransactionMsg{Transaction: api.txs[0], New: true})
	m = mustUpdate(t, m, RedrawFormMsg{})
	m = mustUpdate(t, m, SetFocusedViewMsg{state: formView})
	_, cmd := m.Update(keyMsg("a"))
	if v, ok := findMsg[SetFocusedViewMsg](runCmd(cmd)); ok && v.state == accountsView {
		t.Error("typing in the form must not switch tabs")
	}
}
