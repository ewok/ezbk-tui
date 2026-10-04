/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ezbk-tui/internal/domain"
)

const (
	profileJSON  = `{"username":"john","defaultCurrency":"USD","defaultAccountId":"0"}`
	accountsJSON = `[
		{"id":"10","name":"Wallet","parentId":"0","category":1,"type":1,"currency":"USD","balance":"1250","isAsset":true},
		{"id":"20","name":"Bank","parentId":"0","category":2,"type":2,"currency":"---","balance":"0","isAsset":true,
		 "subAccounts":[
			{"id":"21","name":"EUR","parentId":"20","category":2,"type":1,"currency":"EUR","balance":"-500"},
			{"id":"22","name":"Old","parentId":"20","category":2,"type":1,"currency":"EUR","balance":"0","hidden":true}
		 ]},
		{"id":"30","name":"Visa","parentId":"0","category":3,"type":1,"currency":"USD","balance":"-9999","isLiability":true},
		{"id":"40","name":"Closed","parentId":"0","category":1,"type":1,"currency":"USD","balance":"0","hidden":true}
	]`
	categoriesJSON = `{
		"1":[{"id":"100","name":"Salary","parentId":"0","type":1,"subCategories":[{"id":"101","name":"Main job","parentId":"100","type":1}]}],
		"2":[{"id":"200","name":"Food","parentId":"0","type":2,"subCategories":[
				{"id":"201","name":"Groceries","parentId":"200","type":2},
				{"id":"202","name":"Hidden","parentId":"200","type":2,"hidden":true}]}],
		"3":[{"id":"300","name":"General","parentId":"0","type":3,"subCategories":[{"id":"301","name":"Move","parentId":"300","type":3}]}]
	}`
	tagsJSON         = `[{"id":"7","name":"trip","groupId":"0"},{"id":"8","name":"gone","groupId":"0","hidden":true}]`
	transactionsJSON = `[
		{"id":"1","type":3,"categoryId":"201","time":1759320000,"utcOffset":120,"sourceAccountId":"10","sourceAmount":1500,"tagIds":["7"],"comment":"milk","editable":true},
		{"id":"2","type":2,"categoryId":"101","time":1759406400,"utcOffset":120,"sourceAccountId":"21","sourceAmount":100000,"tagIds":[],"editable":true},
		{"id":"3","type":4,"categoryId":"301","time":1759492800,"utcOffset":120,"sourceAccountId":"10","destinationAccountId":"21","sourceAmount":1000,"destinationAmount":900,"tagIds":[],"editable":false},
		{"id":"4","type":3,"categoryId":"201","time":1759579200,"utcOffset":120,"sourceAccountId":"40","sourceAmount":250,"tagIds":["7"],"editable":true}
	]`
	templatesJSON = `[{"id":"55","name":"Coffee","templateType":1,"type":3,"categoryId":"201","sourceAccountId":"10","sourceAmount":350,"tagIds":[],"comment":"coffee"}]`
)

type fakeServer struct {
	t        *testing.T
	mu       sync.Mutex
	requests []*recordedRequest
	override map[string]string
}

type recordedRequest struct {
	Method string
	Path   string
	Query  map[string][]string
	Body   map[string]any
	Header http.Header
}

func newFakeServer(t *testing.T) (*fakeServer, *httptest.Server) {
	t.Helper()
	fs := &fakeServer{t: t, override: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handle))
	t.Cleanup(srv.Close)
	return fs, srv
}

func (fs *fakeServer) handle(w http.ResponseWriter, r *http.Request) {
	rec := &recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Header: r.Header.Clone()}
	if r.Body != nil {
		data, _ := io.ReadAll(r.Body)
		if len(data) > 0 {
			_ = json.Unmarshal(data, &rec.Body)
		}
	}
	fs.mu.Lock()
	fs.requests = append(fs.requests, rec)
	override, hasOverride := fs.override[r.URL.Path]
	fs.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer secret" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"success":false,"errorCode":202001,"errorMessage":"unauthorized access","path":"`+r.URL.Path+`"}`)
		return
	}
	if hasOverride {
		_, _ = io.WriteString(w, override)
		return
	}
	results := map[string]string{
		"/api/v1/users/profile/get.json":           profileJSON,
		"/api/v1/accounts/list.json":               accountsJSON,
		"/api/v1/transaction/categories/list.json": categoriesJSON,
		"/api/v1/transaction/tags/list.json":       tagsJSON,
		"/api/v1/transaction/templates/list.json":  templatesJSON,
		"/api/v1/transactions/list/all.json":       transactionsJSON,
		"/api/v1/transactions/add.json":            `{"id":"999","type":3}`,
		"/api/v1/transactions/modify.json":         `{"id":"1","type":3}`,
		"/api/v1/transactions/delete.json":         `true`,
		"/api/v1/accounts/add.json":                `{"id":"50"}`,
		"/api/v1/transaction/categories/add.json":  `{"id":"500"}`,
		"/api/v1/transaction/tags/add.json":        `{"id":"9"}`,
	}
	result, ok := results[r.URL.Path]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"success":false,"errorCode":100001,"errorMessage":"api not found"}`)
		return
	}
	_, _ = io.WriteString(w, `{"success":true,"result":`+result+`}`)
}

func (fs *fakeServer) last(path string) *recordedRequest {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for i := len(fs.requests) - 1; i >= 0; i-- {
		if fs.requests[i].Path == path {
			return fs.requests[i]
		}
	}
	fs.t.Fatalf("no request to %s", path)
	return nil
}

func newTestApi(t *testing.T) (*Api, *fakeServer) {
	t.Helper()
	fs, srv := newFakeServer(t)
	api, err := NewApi(Config{ApiUrl: srv.URL, Token: "secret", Timezone: "Europe/Berlin", TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("NewApi: %v", err)
	}
	return api, fs
}

func loadAll(t *testing.T, api *Api) {
	t.Helper()
	for _, f := range []func() error{api.UpdateAccounts, api.UpdateCategories, api.UpdateTags} {
		if err := f(); err != nil {
			t.Fatalf("load: %v", err)
		}
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"https://ez.example.com", "https://ez.example.com/api/v1", false},
		{"https://ez.example.com/", "https://ez.example.com/api/v1", false},
		{"https://ez.example.com/api", "https://ez.example.com/api/v1", false},
		{"https://ez.example.com/api/v1/", "https://ez.example.com/api/v1", false},
		{"https://ex.com/ezbk", "https://ex.com/ezbk/api/v1", false},
		{"not a url", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := normalizeBaseURL(tt.in)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("normalizeBaseURL(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
}

func TestNewApi_ProfileAndHeaders(t *testing.T) {
	api, fs := newTestApi(t)
	if api.Username() != "john" || api.DefaultCurrency() != "USD" {
		t.Errorf("profile = %q %q", api.Username(), api.DefaultCurrency())
	}
	if api.DefaultAccountID() != "" {
		t.Errorf("DefaultAccountID = %q, want empty", api.DefaultAccountID())
	}
	req := fs.last("/api/v1/users/profile/get.json")
	if got := req.Header.Get("X-Timezone-Name"); got != "Europe/Berlin" {
		t.Errorf("X-Timezone-Name = %q", got)
	}
	if req.Header.Get("X-Timezone-Offset") == "" {
		t.Error("X-Timezone-Offset missing")
	}
	start := api.PeriodStart()
	if start.Day() != 1 || start.Hour() != 0 || api.PeriodEnd().Month() != start.Month() {
		t.Errorf("period = %v - %v", start, api.PeriodEnd())
	}
}

func TestNewApi_Unauthorized(t *testing.T) {
	_, srv := newFakeServer(t)
	_, err := NewApi(Config{ApiUrl: srv.URL, Token: "wrong"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 202001 {
		t.Fatalf("err = %v, want APIError 202001", err)
	}
}

func TestUpdateAccounts(t *testing.T) {
	api, _ := newTestApi(t)
	if err := api.UpdateAccounts(); err != nil {
		t.Fatal(err)
	}
	accs := api.Accounts()
	if len(accs) != 3 {
		t.Fatalf("got %d visible accounts, want 3: %+v", len(accs), accs)
	}
	tests := []struct {
		idx       int
		id        string
		display   string
		balance   domain.Money
		liability bool
	}{
		{0, "10", "Wallet", 1250, false},
		{1, "21", "Bank / EUR", -500, false},
		{2, "30", "Visa", -9999, true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			a := accs[tt.idx]
			if a.ID != tt.id || a.DisplayName() != tt.display || a.Balance != tt.balance || a.IsLiability != tt.liability {
				t.Errorf("account = %+v", a)
			}
		})
	}
	if _, ok := api.AccountByID("40"); ok {
		t.Error("hidden account should not be visible")
	}
	if got := api.resolveAccount("40").Name; got != "Closed" {
		t.Errorf("hidden account resolve = %q", got)
	}
	assets, liabilities := api.NetWorth()
	if assets.Get("USD") != 1250 || assets.Get("EUR") != -500 || liabilities.Get("USD") != -9999 {
		t.Errorf("net worth = %v / %v", assets, liabilities)
	}
}

func TestUpdateCategories(t *testing.T) {
	api, _ := newTestApi(t)
	if err := api.UpdateCategories(); err != nil {
		t.Fatal(err)
	}
	exp := api.Categories(domain.CategoryExpense)
	if len(exp) != 2 || !exp[0].IsPrimary() || exp[1].DisplayName() != "Food / Groceries" {
		t.Fatalf("expense categories = %+v", exp)
	}
	if len(api.Categories(domain.CategoryIncome)) != 2 || len(api.Categories(domain.CategoryTransfer)) != 2 {
		t.Error("unexpected income/transfer categories")
	}
	if c, ok := api.CategoryByID("202"); !ok || !c.Hidden {
		t.Error("hidden category should still resolve")
	}
}

func TestListTransactions(t *testing.T) {
	api, fs := newTestApi(t)
	loadAll(t, api)

	txs, err := api.ListTransactions("")
	if err != nil {
		t.Fatal(err)
	}
	req := fs.last("/api/v1/transactions/list/all.json")
	if req.Query["start_time"] == nil || req.Query["end_time"] == nil || req.Query["keyword"] != nil {
		t.Errorf("period query = %v", req.Query)
	}
	if len(txs) != 4 || txs[0].ID != "4" {
		t.Fatalf("expected 4 tx sorted desc, got %+v", txs)
	}
	byID := map[string]domain.Transaction{}
	for _, tx := range txs {
		byID[tx.ID] = tx
	}
	exp := byID["1"]
	if exp.Source.Name != "Wallet" || exp.Category.DisplayName() != "Food / Groceries" || exp.TagNames() != "trip" || exp.SourceAmount != 1500 {
		t.Errorf("expense = %+v", exp)
	}
	if _, off := exp.Time.Zone(); off != 7200 {
		t.Errorf("time offset = %d", off)
	}
	tr := byID["3"]
	if tr.Destination.DisplayName() != "Bank / EUR" || tr.DestinationAmount != 900 || tr.Editable {
		t.Errorf("transfer = %+v", tr)
	}
	if !tr.InvolvesAccount("21") || exp.InvolvesAccount("21") {
		t.Error("InvolvesAccount mismatch")
	}

	if _, err := api.ListTransactions("milk"); err != nil {
		t.Fatal(err)
	}
	req = fs.last("/api/v1/transactions/list/all.json")
	if strings.Join(req.Query["keyword"], "") != "milk" || req.Query["start_time"] != nil {
		t.Errorf("search query = %v", req.Query)
	}
}

func TestPeriodStats(t *testing.T) {
	api, _ := newTestApi(t)
	loadAll(t, api)
	if err := api.UpdatePeriodStats(); err != nil {
		t.Fatal(err)
	}
	if got := api.PeriodExpense().Get("USD"); got != 1750 {
		t.Errorf("expense USD = %d", got)
	}
	if got := api.PeriodIncome().Get("EUR"); got != 100000 {
		t.Errorf("income EUR = %d", got)
	}
	if got := api.CategoryTotal("200").Get("USD"); got != 1750 {
		t.Errorf("primary category total = %d", got)
	}
	if got := api.CategoryTotal("201").Get("USD"); got != 1750 {
		t.Errorf("secondary category total = %d", got)
	}
	if !api.CategoryTotal("301").IsZero() {
		t.Error("transfers must not count in category totals")
	}
	spent, earned := api.TagTotals("7")
	if spent.Get("USD") != 1750 || !earned.IsZero() {
		t.Errorf("tag totals = %v / %v", spent, earned)
	}
}

func TestCreateTransaction_Body(t *testing.T) {
	api, fs := newTestApi(t)
	when := time.Date(2026, 10, 3, 14, 30, 0, 0, api.Location())

	tests := []struct {
		name     string
		req      domain.TransactionRequest
		wantDest string
		wantDAmt float64
	}{
		{
			name:     "expense clears destination",
			req:      domain.TransactionRequest{Type: domain.TxExpense, Time: when, CategoryID: "201", SourceAccountID: "10", DestinationAccountID: "21", SourceAmount: 1234, DestinationAmount: 5, Comment: "x"},
			wantDest: "0",
			wantDAmt: 0,
		},
		{
			name:     "transfer defaults destination amount",
			req:      domain.TransactionRequest{Type: domain.TxTransfer, Time: when, CategoryID: "301", SourceAccountID: "10", DestinationAccountID: "21", SourceAmount: 1000},
			wantDest: "21",
			wantDAmt: 1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := api.CreateTransaction(tt.req)
			if err != nil || id != "999" {
				t.Fatalf("CreateTransaction = %q, %v", id, err)
			}
			body := fs.last("/api/v1/transactions/add.json").Body
			if body["destinationAccountId"] != tt.wantDest || body["destinationAmount"] != tt.wantDAmt {
				t.Errorf("body = %v", body)
			}
			if body["time"] != float64(when.Unix()) || body["utcOffset"] != float64(120) {
				t.Errorf("time = %v offset = %v", body["time"], body["utcOffset"])
			}
			if _, hasID := body["id"]; hasID {
				t.Error("create must not send id")
			}
			if tags, ok := body["tagIds"].([]any); !ok || len(tags) != 0 {
				t.Errorf("tagIds = %v", body["tagIds"])
			}
		})
	}
}

func TestUpdateAndDeleteTransaction(t *testing.T) {
	api, fs := newTestApi(t)
	req := domain.TransactionRequest{ID: "1", Type: domain.TxExpense, CategoryID: "201", SourceAccountID: "10", SourceAmount: 100}
	if _, err := api.UpdateTransaction(req); err != nil {
		t.Fatal(err)
	}
	if fs.last("/api/v1/transactions/modify.json").Body["id"] != "1" {
		t.Error("modify must send id")
	}

	fs.override["/api/v1/transactions/modify.json"] = `{"success":false,"errorCode":200000,"errorMessage":"nothing will be updated"}`
	if _, err := api.UpdateTransaction(req); err != nil {
		t.Errorf("nothing-updated should not be an error, got %v", err)
	}
	if _, err := api.UpdateTransaction(domain.TransactionRequest{}); err == nil {
		t.Error("update without id must fail")
	}

	if err := api.DeleteTransaction("1"); err != nil {
		t.Fatal(err)
	}
	if fs.last("/api/v1/transactions/delete.json").Body["id"] != "1" {
		t.Error("delete must send id")
	}
}

func TestCreateAccountCategoryTag(t *testing.T) {
	api, fs := newTestApi(t)

	if err := api.CreateAccount(" Pocket ", domain.AccountCash, "usd"); err != nil {
		t.Fatal(err)
	}
	body := fs.last("/api/v1/accounts/add.json").Body
	if body["name"] != "Pocket" || body["currency"] != "USD" || body["category"] != float64(1) || body["type"] != float64(1) || body["icon"] != "1" || body["color"] != "000000" {
		t.Errorf("account body = %v", body)
	}

	if id, err := api.CreateCategory("Snacks", domain.CategoryExpense, "200"); err != nil || id != "500" {
		t.Fatalf("CreateCategory = %q, %v", id, err)
	}
	body = fs.last("/api/v1/transaction/categories/add.json").Body
	if body["parentId"] != "200" || body["type"] != float64(2) {
		t.Errorf("category body = %v", body)
	}
	if _, err := api.CreateCategory("Top", domain.CategoryIncome, ""); err != nil {
		t.Fatal(err)
	}
	if fs.last("/api/v1/transaction/categories/add.json").Body["parentId"] != "0" {
		t.Error("primary category must send parentId 0")
	}

	if err := api.CreateTag("work"); err != nil {
		t.Fatal(err)
	}
	if fs.last("/api/v1/transaction/tags/add.json").Body["groupId"] != "0" {
		t.Error("tag must send groupId 0")
	}
}

func TestTagsAndTemplates(t *testing.T) {
	api, fs := newTestApi(t)
	loadAll(t, api)
	if tags := api.Tags(); len(tags) != 1 || tags[0].Name != "trip" {
		t.Errorf("tags = %+v", tags)
	}
	if err := api.UpdateTemplates(); err != nil {
		t.Fatal(err)
	}
	if fs.last("/api/v1/transaction/templates/list.json").Query["templateType"][0] != "1" {
		t.Error("templateType=1 expected")
	}
	tpls := api.Templates()
	if len(tpls) != 1 || tpls[0].Name != "Coffee" || tpls[0].Transaction.Source.Name != "Wallet" || tpls[0].Transaction.SourceAmount != 350 {
		t.Errorf("templates = %+v", tpls)
	}
}

func TestAPIErrorOnBadJSON(t *testing.T) {
	api, fs := newTestApi(t)
	fs.override["/api/v1/transaction/tags/list.json"] = `<html>oops</html>`
	err := api.UpdateTags()
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !strings.Contains(apiErr.Message, "oops") {
		t.Errorf("err = %v", err)
	}
}
