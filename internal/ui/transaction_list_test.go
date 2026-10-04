/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/prompt"
)

func newLoadedTransactions(t *testing.T, api *mockAPI) modelTransactions {
	t.Helper()
	m := NewModelTransactions(api)
	m.Focus()
	msgs := runCmd(m.refreshCmd(""))
	upd, ok := findMsg[TransactionsUpdateMsg](msgs)
	if !ok {
		t.Fatalf("expected TransactionsUpdateMsg, got %v", msgs)
	}
	m, _ = updateModel(m, upd)
	return m
}

func visibleIDs(m modelTransactions) []string {
	ids := []string{}
	for _, row := range m.table.Rows() {
		ids = append(ids, rowTxID(row))
	}
	return ids
}

func TestTxFilter(t *testing.T) {
	api := newMockAPI()
	tests := []struct {
		name   string
		filter txFilter
		want   []string
	}{
		{"no filter", txFilter{}, []string{"1", "2", "3", "4"}},
		{"account as source", txFilter{account: accWallet}, []string{"1", "3"}},
		{"account as transfer destination", txFilter{account: accEUR}, []string{"2", "3"}},
		{"primary category includes children", txFilter{category: catFood}, []string{"1"}},
		{"secondary category", txFilter{category: catFuel}, []string{"4"}},
		{"tag", txFilter{tag: tagWork}, []string{"4"}},
		{"query matches comment", txFilter{query: "MILK"}, []string{"1"}},
		{"query matches category", txFilter{query: "salary"}, []string{"2"}},
		{"query matches tag", txFilter{query: "trip"}, []string{"1"}},
		{"combined", txFilter{account: accWallet, query: "fx"}, []string{"3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := []string{}
			for _, tx := range api.txs {
				if tt.filter.match(tx) {
					got = append(got, tx.ID)
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestApplyFilter_ExclusiveOnRepeat(t *testing.T) {
	m := newLoadedTransactions(t, newMockAPI())

	m, _ = updateModel(m, FilterMsg{Account: accWallet})
	m, _ = updateModel(m, FilterMsg{Query: "fx"})
	if got := visibleIDs(m); len(got) != 1 || got[0] != "3" {
		t.Fatalf("account+query = %v", got)
	}

	m, _ = updateModel(m, FilterMsg{Query: "fx"})
	if !m.filter.account.IsEmpty() || m.filter.query != "fx" {
		t.Errorf("repeating query should drop other filters, got %+v", m.filter)
	}

	m, _ = updateModel(m, FilterMsg{Query: "None"})
	if m.filter.query != "" {
		t.Error("query None should clear query")
	}

	m, _ = updateModel(m, FilterMsg{Tag: tagTrip})
	m, _ = updateModel(m, FilterMsg{Reset: true})
	if len(visibleIDs(m)) != 4 {
		t.Error("reset should show all transactions")
	}
}

func TestSearch_SavesAndRestoresFilter(t *testing.T) {
	api := newMockAPI()
	m := newLoadedTransactions(t, api)
	m, _ = updateModel(m, FilterMsg{Category: catFuel})

	m, cmd := updateModel(m, SearchMsg{Query: "milk"})
	if m.currentSearch != "milk" || !m.filter.category.IsEmpty() {
		t.Fatalf("search state = %q %+v", m.currentSearch, m.filter)
	}
	if !hasMsg[RefreshTransactionsMsg](runCmd(cmd)) {
		t.Error("search should trigger refresh")
	}
	runCmd(m.refreshCmd(""))
	if api.lastQuery != "milk" {
		t.Errorf("api query = %q", api.lastQuery)
	}

	m, _ = updateModel(m, SearchMsg{Query: "None"})
	if m.currentSearch != "" || m.filter.category.ID != catFuel.ID {
		t.Errorf("filter not restored: %q %+v", m.currentSearch, m.filter)
	}
}

func TestGetRows(t *testing.T) {
	api := newMockAPI()
	rows, cols := getRows(api.txs)
	if len(cols) != len(transactionColumns) || cols[0].Width != 0 {
		t.Fatalf("columns = %+v", cols)
	}
	tests := []struct {
		idx    int
		col    int
		want   string
		reason string
	}{
		{0, 1, "←", "expense icon"},
		{0, 6, "-15.50", "expense negative"},
		{1, 6, "+3000.00", "income positive"},
		{2, 1, "⇄*", "read-only transfer marked"},
		{2, 4, "Bank / EUR", "transfer destination"},
		{2, 8, "91.00 EUR", "cross-currency destination amount"},
		{0, 8, "", "no destination amount for expense"},
		{0, 9, "trip", "tags"},
		{0, 2, "2026-10-03", "date"},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			if got := rows[tt.idx][tt.col]; got != tt.want {
				t.Errorf("row %d col %d = %q, want %q", tt.idx, tt.col, got, tt.want)
			}
		})
	}
}

func TestTransactions_EditReadOnlyRefused(t *testing.T) {
	m := newLoadedTransactions(t, newMockAPI())
	m.table.SetCursor(2)
	_, cmd := updateModel(m, keyMsg("enter"))
	msgs := runCmd(cmd)
	if hasMsg[EditTransactionMsg](msgs) {
		t.Fatal("read-only transaction must not open the editor")
	}

	m.table.SetCursor(0)
	_, cmd = updateModel(m, keyMsg("enter"))
	edit, ok := findMsg[EditTransactionMsg](runCmd(cmd))
	if !ok || edit.Transaction.ID != "1" {
		t.Errorf("expected edit of tx 1, got %v", edit)
	}
}

func TestTransactions_DeleteFlow(t *testing.T) {
	api := newMockAPI()
	m := newLoadedTransactions(t, api)
	_, cmd := updateModel(m, keyMsg("D"))
	p, ok := findMsg[prompt.PromptMsg](runCmd(cmd))
	if !ok {
		t.Fatal("delete should ask for confirmation")
	}
	if hasMsg[DeleteTransactionMsg](runCmd(p.Callback("no"))) {
		t.Error("only 'yes!' confirms")
	}
	del, ok := findMsg[DeleteTransactionMsg](runCmd(p.Callback("yes!")))
	if !ok || del.Transaction.ID != "1" {
		t.Fatalf("expected delete of tx 1, got %v", del)
	}

	_, cmd = updateModel(m, del)
	res, ok := findMsg[TransactionDeleteResultMsg](runCmd(cmd))
	if !ok || res.Err != nil || len(api.deleted) != 1 {
		t.Fatalf("delete result = %+v, deleted %v", res, api.deleted)
	}
	_, cmd = updateModel(m, res)
	msgs := runCmd(cmd)
	if !hasMsg[RefreshAccountsMsg](msgs) || !hasMsg[RefreshStatsMsg](msgs) || !hasMsg[RefreshTransactionsMsg](msgs) {
		t.Errorf("delete should refresh accounts, stats and transactions: %v", msgs)
	}
}

func TestTransactions_NewPrefilledFromFilter(t *testing.T) {
	tests := []struct {
		name     string
		filter   FilterMsg
		wantType domain.TransactionType
	}{
		{"expense category", FilterMsg{Category: catGroceries}, domain.TxExpense},
		{"income category", FilterMsg{Category: catMainJob}, domain.TxIncome},
		{"transfer category", FilterMsg{Category: catMove}, domain.TxTransfer},
		{"account only", FilterMsg{Account: accVisa}, domain.TxExpense},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newLoadedTransactions(t, newMockAPI())
			m, _ = updateModel(m, tt.filter)
			_, cmd := updateModel(m, keyMsg("n"))
			nt, ok := findMsg[NewTransactionMsg](runCmd(cmd))
			if !ok || nt.Transaction.Type != tt.wantType {
				t.Fatalf("got %+v", nt)
			}
			if nt.Transaction.Source.ID != tt.filter.Account.ID || nt.Transaction.Category.ID != tt.filter.Category.ID {
				t.Errorf("prefill = %+v", nt.Transaction)
			}
		})
	}
}

func TestTransactions_UnfocusedIgnoresKeys(t *testing.T) {
	m := newLoadedTransactions(t, newMockAPI())
	m.Blur()
	_, cmd := updateModel(m, keyMsg("n"))
	if cmd != nil {
		if msgs := runCmd(cmd); hasMsg[NewTransactionMsg](msgs) {
			t.Error("unfocused list must ignore keys")
		}
	}
	var _ tea.Model = m
}
