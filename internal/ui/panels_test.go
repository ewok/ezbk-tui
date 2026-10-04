/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"strings"
	"testing"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/prompt"
)

func TestParseNewAccount(t *testing.T) {
	tests := []struct {
		in      string
		want    NewAccountMsg
		wantErr bool
	}{
		{"Pocket", NewAccountMsg{Name: "Pocket", Currency: "USD", Category: domain.AccountChecking}, false},
		{"Pocket, eur", NewAccountMsg{Name: "Pocket", Currency: "EUR", Category: domain.AccountChecking}, false},
		{"Card,usd,credit", NewAccountMsg{Name: "Card", Currency: "USD", Category: domain.AccountCreditCard}, false},
		{"Box,,cash", NewAccountMsg{Name: "Box", Currency: "USD", Category: domain.AccountCash}, false},
		{",usd", NewAccountMsg{}, true},
		{"X,dollars", NewAccountMsg{}, true},
		{"X,usd,unknown", NewAccountMsg{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseNewAccount(tt.in, "USD")
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("parseNewAccount(%q) = %+v, %v", tt.in, got, err)
			}
		})
	}
}

func TestParseNewCategory(t *testing.T) {
	tests := []struct {
		in      string
		want    NewCategoryMsg
		wantErr bool
	}{
		{"Food", NewCategoryMsg{Type: domain.CategoryExpense, Name: "Food"}, false},
		{"Food / Snacks", NewCategoryMsg{Type: domain.CategoryExpense, Parent: "Food", Name: "Snacks"}, false},
		{"Food/", NewCategoryMsg{}, true},
		{"/Snacks", NewCategoryMsg{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseNewCategory(domain.CategoryExpense, tt.in)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("parseNewCategory(%q) = %+v, %v", tt.in, got, err)
			}
		})
	}
}

func TestCreateCategoryPath(t *testing.T) {
	tests := []struct {
		name string
		msg  NewCategoryMsg
		want []string
	}{
		{"primary", NewCategoryMsg{Type: domain.CategoryExpense, Name: "Pets"}, []string{"Pets@"}},
		{"existing parent case-insensitive", NewCategoryMsg{Type: domain.CategoryExpense, Parent: "food", Name: "Snacks"}, []string{"Snacks@200"}},
		{"missing parent is created", NewCategoryMsg{Type: domain.CategoryExpense, Parent: "Health", Name: "Pharmacy"}, []string{"Health@", "Pharmacy@9Health"}},
		{"parent of other type not reused", NewCategoryMsg{Type: domain.CategoryIncome, Parent: "Food", Name: "Refund"}, []string{"Food@", "Refund@9Food"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newMockAPI()
			if err := createCategoryPath(api, tt.msg); err != nil {
				t.Fatal(err)
			}
			if strings.Join(api.newCats, ",") != strings.Join(tt.want, ",") {
				t.Errorf("created = %v, want %v", api.newCats, tt.want)
			}
		})
	}
}

func TestCategoryItems(t *testing.T) {
	api := newMockAPI()
	items := categoryItems(api, domain.CategoryExpense, false)
	if len(items) != 4 {
		t.Fatalf("tree items = %d", len(items))
	}
	if got := items[1].(categoryItem).Title(); got != "  · Groceries" {
		t.Errorf("child title = %q", got)
	}
	if got := items[0].(categoryItem).Description(); got != "Spent: 15.50 USD" {
		t.Errorf("primary description = %q", got)
	}

	sorted := categoryItems(api, domain.CategoryExpense, true)
	if len(sorted) != 2 {
		t.Fatalf("sorted items = %d", len(sorted))
	}
	first := sorted[0].(categoryItem)
	if first.category.ID != "211" || first.Title() != "Transport / Fuel" {
		t.Errorf("sorted first = %q", first.Title())
	}

	income := categoryItems(api, domain.CategoryIncome, false)
	if got := income[0].(categoryItem).Description(); got != "Earned: 3000.00 EUR" {
		t.Errorf("income description = %q", got)
	}
}

func TestCategoriesModel_OnlyLoaderRefreshes(t *testing.T) {
	api := newMockAPI()
	exp := newModelCategories(api, domain.CategoryExpense, expenseView, true)
	inc := newModelCategories(api, domain.CategoryIncome, incomeView, false)
	if _, cmd := updateModel(exp, RefreshCategoriesMsg{}); !hasMsg[CategoriesUpdatedMsg](runCmd(cmd)) {
		t.Error("loader should refresh categories")
	}
	if _, cmd := updateModel(inc, RefreshCategoriesMsg{}); cmd != nil {
		t.Error("non-loader must not refresh")
	}
	if _, cmd := updateModel(inc, NewCategoryMsg{Type: domain.CategoryExpense, Name: "x"}); cmd != nil {
		t.Error("income model must ignore expense category creation")
	}
}

func TestCategoriesModel_NewPromptPrefillsParent(t *testing.T) {
	api := newMockAPI()
	m := newModelCategories(api, domain.CategoryExpense, expenseView, true)
	m, _ = updateModel(m, CategoriesUpdatedMsg{})
	m.Focus()
	m.list.Select(2)
	_, cmd := updateModel(m, keyMsg("n"))
	p, ok := findMsg[prompt.PromptMsg](runCmd(cmd))
	if !ok || p.Value != "Food/" {
		t.Fatalf("prompt = %+v", p)
	}
}

func TestAccountItems(t *testing.T) {
	api := newMockAPI()
	items := accountItems(api.accounts, false)
	if items[1].(accountItem).Title() != "Bank / EUR" {
		t.Errorf("title = %q", items[1].(accountItem).Title())
	}
	if got := items[3].(accountItem).Description(); got != "-450.00 USD · Credit Card" {
		t.Errorf("description = %q", got)
	}
	grouped := accountItems(api.accounts, true)
	order := []string{}
	for _, it := range grouped {
		order = append(order, it.(accountItem).account.ID)
	}
	if strings.Join(order, ",") != "10,21,22,30" {
		t.Errorf("grouped order = %v", order)
	}
}

func TestAccountsModel_FilterBy(t *testing.T) {
	api := newMockAPI()
	m := newModelAccounts(api)
	m, _ = updateModel(m, AccountsUpdatedMsg{})
	m.Focus()
	_, cmd := updateModel(m, keyMsg("f"))
	f, ok := findMsg[FilterMsg](runCmd(cmd))
	if !ok || f.Account.ID != "10" {
		t.Fatalf("filter = %+v", f)
	}
	_, cmd = updateModel(m, keyMsg("enter"))
	msgs := runCmd(cmd)
	if v, ok := findMsg[SetFocusedViewMsg](msgs); !ok || v.state != transactionsView {
		t.Errorf("enter should switch to transactions: %v", msgs)
	}
}

func TestTagItems(t *testing.T) {
	api := newMockAPI()
	items := tagItems(api, false)
	if len(items) != 2 || items[0].(tagItem).Description() != "Spent: 15.50 USD" || items[1].(tagItem).Description() != "No transactions" {
		t.Errorf("items = %+v", items)
	}
	if sorted := tagItems(api, true); len(sorted) != 1 {
		t.Errorf("sorted should hide empty tags, got %d", len(sorted))
	}
}

func TestSummaryItems(t *testing.T) {
	api := newMockAPI()
	items := summaryItems(api, DefaultStyles())
	got := map[string]string{}
	for _, it := range items {
		si := it.(summaryItem)
		got[si.title] = si.value
	}
	want := map[string]string{
		"Net worth USD": "-324.00",
		"Net worth EUR": "2500.00",
		"Income EUR":    "3000.00",
		"Expense USD":   "-75.50",
		"Balance USD":   "-75.50",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	if first := items[0].(summaryItem).title; first != "Net worth USD" {
		t.Errorf("primary currency first, got %q", first)
	}
}

func TestSummary_RefreshStats(t *testing.T) {
	api := newMockAPI()
	m := newModelSummary(api)
	_, cmd := updateModel(m, RefreshStatsMsg{})
	if !hasMsg[StatsUpdatedMsg](runCmd(cmd)) || api.statsUpdates != 1 {
		t.Error("RefreshStatsMsg should update stats")
	}
}

func TestTemplatesModel_Select(t *testing.T) {
	api := newMockAPI()
	api.templates = []domain.Template{{ID: "55", Name: "Coffee", Transaction: domain.Transaction{ID: "55", Type: domain.TxExpense, Source: accWallet, Category: catGroceries, SourceAmount: 350}}}
	m := newModelTemplates(api)
	m, _ = updateModel(m, TemplatesUpdatedMsg{})
	m.Focus()
	_, cmd := updateModel(m, keyMsg("enter"))
	nt, ok := findMsg[NewTransactionMsg](runCmd(cmd))
	if !ok || nt.Transaction.ID != "" || nt.Transaction.SourceAmount != 350 {
		t.Fatalf("template -> %+v", nt)
	}
	if got := m.list.Items()[0].(templateItem).Description(); got != "← 3.50 USD · Wallet · Food / Groceries" {
		t.Errorf("description = %q", got)
	}
}
