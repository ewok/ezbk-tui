/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"

	"ezbk-tui/internal/domain"
	"ezbk-tui/internal/ui/notify"
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

func TestCategoriesModel_TotalRow(t *testing.T) {
	tests := []struct {
		name    string
		convert bool
		catType domain.CategoryType
		expense domain.Amounts
		want    string
	}{
		{"income converted", true, domain.CategoryIncome, nil, "Earned: 3750.00 USD"},
		{"income breakdown", false, domain.CategoryIncome, nil, "Earned: 3000.00 EUR"},
		{"expense mixed converted", true, domain.CategoryExpense, domain.Amounts{"USD": 100, "EUR": 100}, "Spent: 2.25 USD"},
		{"expense mixed breakdown", false, domain.CategoryExpense, domain.Amounts{"USD": 100, "EUR": 100}, "Spent: 1.00 USD, 1.00 EUR"},
		{"expense missing rate", true, domain.CategoryExpense, domain.Amounts{"USD": 100, "KGS": 500}, "Spent: ~1.00 USD, 5.00 KGS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newMockAPI()
			if tt.expense != nil {
				api.expense = tt.expense
			}
			m := newModelCategories(api, tt.catType, expenseView, false)
			m, _ = updateModel(m, CurrencyModeMsg{Converted: tt.convert})
			if got := m.list.Items()[0].(simpleItem).Description(); got != tt.want {
				t.Errorf("total = %q, want %q", got, tt.want)
			}
		})
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

func accountOrder(items []list.Item) string {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.(accountItem).account.ID)
	}
	return strings.Join(ids, ",")
}

func TestAccountItems(t *testing.T) {
	api := newMockAPI()
	items := accountItems(api.accounts, sortDefault, "USD")
	if items[1].(accountItem).Title() != "Bank / EUR" {
		t.Errorf("title = %q", items[1].(accountItem).Title())
	}
	if got := items[3].(accountItem).Description(); got != "-450.00 USD · Credit Card" {
		t.Errorf("description = %q", got)
	}

	tests := []struct {
		name    string
		mode    accountSort
		primary string
		want    string
	}{
		{"default keeps server order", sortDefault, "USD", "10,21,22,30"},
		{"by name", sortName, "USD", "21,22,30,10"},
		{"by balance desc", sortBalance, "USD", "21,10,22,30"},
		{"by currency primary first", sortCurrency, "USD", "22,30,10,21"},
		{"by currency other primary", sortCurrency, "EUR", "21,22,30,10"},
		{"by category", sortCategory, "USD", "10,21,22,30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accountOrder(accountItems(api.accounts, tt.mode, tt.primary)); got != tt.want {
				t.Errorf("order = %s, want %s", got, tt.want)
			}
		})
	}
	if accountOrder(accountItems(api.accounts, sortDefault, "")) != "10,21,22,30" {
		t.Error("sorting must not mutate the source slice")
	}
}

func TestAccountsModel_SortCycle(t *testing.T) {
	api := newMockAPI()
	m := newModelAccounts(api)
	m, _ = updateModel(m, AccountsUpdatedMsg{})
	m.Focus()

	steps := []struct {
		title, order, help string
	}{
		{"Accounts · by name", "21,22,30,10", "sort: by balance"},
		{"Accounts · by balance", "21,10,22,30", "sort: by currency"},
		{"Accounts · by currency", "22,30,10,21", "sort: by category"},
		{"Accounts · by category", "10,21,22,30", "sort: default"},
		{"Accounts", "10,21,22,30", "sort: by name"},
	}
	for i, s := range steps {
		var cmd tea.Cmd
		m, cmd = updateModel(m, keyMsg("s"))
		if _, ok := findMsg[AccountsUpdatedMsg](runCmd(cmd)); !ok {
			t.Fatalf("step %d: expected AccountsUpdatedMsg", i)
		}
		m, _ = updateModel(m, AccountsUpdatedMsg{})
		if m.list.Title != s.title {
			t.Errorf("step %d: title = %q, want %q", i, m.list.Title, s.title)
		}
		if got := accountOrder(m.list.Items()); got != s.order {
			t.Errorf("step %d: order = %s, want %s", i, got, s.order)
		}
		if got := m.keymap.Sort.Help().Desc; got != s.help {
			t.Errorf("step %d: help = %q, want %q", i, got, s.help)
		}
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

func summaryTitles(items []list.Item) ([]string, map[string]string) {
	titles := []string{}
	got := map[string]string{}
	for _, it := range items {
		si := it.(summaryItem)
		titles = append(titles, si.title)
		got[si.title] = si.value
	}
	return titles, got
}

func TestSummaryItems(t *testing.T) {
	kgsAccount := domain.Account{ID: "40", Name: "Som", Currency: "KGS", Balance: 10000, IsAsset: true}
	tests := []struct {
		name        string
		convert     bool
		setup       func(*mockAPI)
		wantTitles  []string
		want        map[string]string
		wantMissing []string
	}{
		{
			name:       "breakdown",
			convert:    false,
			wantTitles: []string{"Net worth USD", "Net worth EUR", "Income USD", "Expense USD", "Balance USD", "Income EUR", "Expense EUR", "Balance EUR"},
			want: map[string]string{
				"Net worth USD": "-324.00", "Net worth EUR": "2500.00",
				"Income EUR": "3000.00", "Expense USD": "-75.50", "Balance USD": "-75.50",
			},
		},
		{
			name:       "converted",
			convert:    true,
			wantTitles: []string{"Net worth", "Income", "Expense", "Balance"},
			want: map[string]string{
				// -324 USD + 2500 EUR * 1.25 = 2801 USD
				"Net worth": "2801.00 USD", "Income": "3750.00 USD",
				"Expense": "-75.50 USD", "Balance": "3674.50 USD",
			},
		},
		{
			name:    "converted with missing rate",
			convert: true,
			setup: func(m *mockAPI) {
				m.accounts = append(m.accounts, kgsAccount)
				m.expense = domain.Amounts{"USD": 7550, "KGS": 2000}
			},
			wantTitles: []string{"Net worth", "Net worth KGS", "Income", "Expense", "Balance", "Income KGS", "Expense KGS", "Balance KGS"},
			want: map[string]string{
				"Net worth": "~2801.00 USD", "Net worth KGS": "100.00",
				"Income": "3750.00 USD", "Expense": "~-75.50 USD", "Balance": "~3674.50 USD",
				"Expense KGS": "-20.00", "Balance KGS": "-20.00",
			},
			wantMissing: []string{"KGS"},
		},
		{
			name:    "converted without rates",
			convert: true,
			setup:   func(m *mockAPI) { m.rates = domain.ExchangeRates{} },
			want: map[string]string{
				"Net worth": "~-324.00 USD", "Net worth EUR": "2500.00",
				"Income": "~0.00 USD", "Income EUR": "3000.00",
			},
			wantMissing: []string{"EUR"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newMockAPI()
			if tt.setup != nil {
				tt.setup(api)
			}
			items, missing := summaryItems(api, DefaultStyles(), tt.convert)
			titles, got := summaryTitles(items)
			if tt.wantTitles != nil && strings.Join(titles, "|") != strings.Join(tt.wantTitles, "|") {
				t.Errorf("titles = %v, want %v", titles, tt.wantTitles)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("%s = %q, want %q (all: %v)", k, got[k], v, got)
				}
			}
			if strings.Join(missing, ",") != strings.Join(tt.wantMissing, ",") {
				t.Errorf("missing = %v, want %v", missing, tt.wantMissing)
			}
		})
	}
}

func TestSummary_NoWarningBeforeRatesLoaded(t *testing.T) {
	api := newMockAPI()
	api.rates = domain.ExchangeRates{}
	m := newModelSummary(api)
	m, cmd := updateModel(m, SummaryUpdateMsg{})
	if hasMsg[notify.NotifyMsg](runCmd(cmd)) {
		t.Error("must not warn while exchange rates are not loaded")
	}
	if got := m.list.Items()[0].(summaryItem).value; got != "~-324.00 USD" {
		t.Errorf("net worth = %q, want partial value", got)
	}
}

func TestSummary_CurrencyModeAndWarning(t *testing.T) {
	api := newMockAPI()
	api.accounts = append(api.accounts, domain.Account{ID: "40", Name: "Som", Currency: "KGS", Balance: 10000, IsAsset: true})
	m := newModelSummary(api)

	m, cmd := updateModel(m, StatsUpdatedMsg{})
	if !hasMsg[notify.NotifyMsg](runCmd(cmd)) {
		t.Error("a currency without a loaded rate should produce a warning")
	}
	if m.list.Items()[0].(summaryItem).title != "Net worth" {
		t.Errorf("default mode should be converted")
	}
	m, cmd = updateModel(m, StatsUpdatedMsg{})
	if hasMsg[notify.NotifyMsg](runCmd(cmd)) {
		t.Error("the same missing set must warn only once")
	}
	m, _ = updateModel(m, CurrencyModeMsg{Converted: false})
	if m.list.Items()[0].(summaryItem).title != "Net worth USD" {
		t.Errorf("breakdown expected, got %q", m.list.Items()[0].(summaryItem).title)
	}
}

func TestUI_ToggleCurrencyKey(t *testing.T) {
	viper.Set(convertTotalsKey, true)
	t.Cleanup(func() { viper.Set(convertTotalsKey, true) })
	m := NewModelUI(newMockAPI())
	if !m.convertTotals || !m.summary.converted || !m.expense.converted {
		t.Fatal("converted mode should be on by default")
	}
	next, cmd := m.Update(keyMsg("c"))
	ui := next.(modelUI)
	msg, ok := findMsg[CurrencyModeMsg](runCmd(cmd))
	if !ok || msg.Converted || ui.convertTotals || viper.GetBool(convertTotalsKey) {
		t.Errorf("toggle failed: msg=%v ok=%v root=%v", msg, ok, ui.convertTotals)
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
