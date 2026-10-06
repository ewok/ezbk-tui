/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"ezbk-tui/internal/domain"
)

func newFormWith(t *testing.T, api *mockAPI, tx domain.Transaction, isNew bool) modelTransaction {
	t.Helper()
	m := newModelTransaction(api)
	m.SetTransaction(tx, isNew)
	m.UpdateForm()
	return m
}

func TestForm_SetTransactionNewDefaults(t *testing.T) {
	api := newMockAPI()
	m := newFormWith(t, api, domain.Transaction{}, true)
	d := m.data
	if d.txType != domain.TxExpense || d.sourceID != "10" || d.id != "" || !m.new {
		t.Errorf("defaults = %+v new=%v", d, m.new)
	}
	now := time.Now().In(api.loc)
	if d.dateString() != now.Format("2006-01-02") {
		t.Errorf("date = %s", d.dateString())
	}
}

func TestForm_BuildRequest(t *testing.T) {
	api := newMockAPI()
	tests := []struct {
		name     string
		setup    func(d *formData)
		wantErr  string
		validate func(t *testing.T, req domain.TransactionRequest)
	}{
		{
			name: "expense",
			setup: func(d *formData) {
				d.txType, d.sourceID, d.categoryID, d.amount, d.comment = domain.TxExpense, "10", "201", "12.5", "bread"
				d.tagIDs = []string{"7"}
			},
			validate: func(t *testing.T, r domain.TransactionRequest) {
				if r.SourceAmount != 1250 || r.CategoryID != "201" || r.DestinationAccountID != "" || r.Comment != "bread" || len(r.TagIDs) != 1 {
					t.Errorf("req = %+v", r)
				}
			},
		},
		{
			name: "transfer same currency copies amount",
			setup: func(d *formData) {
				d.txType, d.sourceID, d.destID, d.categoryID, d.amount, d.destAmount = domain.TxTransfer, "10", "22", "301", "5", "999"
			},
			validate: func(t *testing.T, r domain.TransactionRequest) {
				if r.DestinationAccountID != "22" || r.DestinationAmount != 500 {
					t.Errorf("req = %+v", r)
				}
			},
		},
		{
			name: "transfer cross currency uses destination amount",
			setup: func(d *formData) {
				d.txType, d.sourceID, d.destID, d.categoryID, d.amount, d.destAmount = domain.TxTransfer, "10", "21", "301", "100", "91"
			},
			validate: func(t *testing.T, r domain.TransactionRequest) {
				if r.SourceAmount != 10000 || r.DestinationAmount != 9100 {
					t.Errorf("req = %+v", r)
				}
			},
		},
		{
			name: "cross currency with bad destination amount",
			setup: func(d *formData) {
				d.txType, d.sourceID, d.destID, d.categoryID, d.amount, d.destAmount = domain.TxTransfer, "10", "21", "301", "100", ""
			},
			wantErr: "destination amount",
		},
		{
			name:    "bad amount",
			setup:   func(d *formData) { d.amount = "abc" },
			wantErr: "invalid amount",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newFormWith(t, api, domain.Transaction{}, true)
			tt.setup(m.data)
			req, err := m.buildRequest()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tt.validate(t, req)
		})
	}
}

func TestForm_EditKeepsTimeOfDay(t *testing.T) {
	api := newMockAPI()
	tx := api.txs[0]
	m := newFormWith(t, api, tx, false)
	if m.new || m.data.id != "1" || m.data.amount != "15.50" {
		t.Fatalf("edit form = %+v new=%v", m.data, m.new)
	}
	req, err := m.buildRequest()
	if err != nil {
		t.Fatal(err)
	}
	if !req.Time.Equal(tx.Time) || req.ID != "1" {
		t.Errorf("time = %v, want %v", req.Time, tx.Time)
	}

	m.data.day = "05"
	req, _ = m.buildRequest()
	want := time.Date(2026, 10, 5, 9, 15, 0, 0, api.loc)
	if !req.Time.Equal(want) {
		t.Errorf("changed day: time = %v, want %v", req.Time, want)
	}
}

func TestForm_NewFromExistingClearsID(t *testing.T) {
	api := newMockAPI()
	m := newFormWith(t, api, copyForNew(api.txs[3]), true)
	if m.data.id != "" || !m.new || m.data.sourceID != "30" || m.data.categoryID != "211" {
		t.Errorf("copy = %+v", m.data)
	}
	if m.data.dateString() != time.Now().In(api.loc).Format("2006-01-02") {
		t.Error("copied transaction should default to today")
	}
}

func TestForm_TransferDestAmountPrefill(t *testing.T) {
	api := newMockAPI()
	m := newFormWith(t, api, api.txs[2], false)
	if m.data.destAmount != "91.00" || !m.needsDestAmount() {
		t.Errorf("destAmount = %q needs=%v", m.data.destAmount, m.needsDestAmount())
	}
}

func TestForm_CategoryOptionsBySecondaryType(t *testing.T) {
	api := newMockAPI()
	m := newFormWith(t, api, domain.Transaction{}, true)
	tests := []struct {
		t    domain.TransactionType
		want []string
	}{
		{domain.TxExpense, []string{"201", "211"}},
		{domain.TxIncome, []string{"101"}},
		{domain.TxTransfer, []string{"301"}},
	}
	for _, tt := range tests {
		t.Run(tt.t.String(), func(t *testing.T) {
			opts := m.opts.categoryOptions(tt.t)
			if len(opts) != len(tt.want) {
				t.Fatalf("options = %v", opts)
			}
			for i, o := range opts {
				if o.Value != tt.want[i] {
					t.Errorf("option %d = %s, want %s", i, o.Value, tt.want[i])
				}
			}
		})
	}
	m.data.txType = domain.TxExpense
	if opts := m.destOptions(); len(opts) != 1 || opts[0].Value != "" {
		t.Errorf("non-transfer destination options = %v", opts)
	}
	m.data.txType = domain.TxTransfer
	m.data.sourceID = "10"
	for _, o := range m.destOptions() {
		if o.Value == "10" {
			t.Error("destination options must exclude the source account")
		}
	}
}

func TestForm_SaveResult(t *testing.T) {
	api := newMockAPI()
	m := newFormWith(t, api, domain.Transaction{}, true)
	m.created = true

	m2, cmd := updateModel(m, TransactionSaveResultMsg{Err: errors.New("boom")})
	if !m2.created || hasMsg[RefreshTransactionsMsg](runCmd(cmd)) {
		t.Error("failed save must keep the form")
	}

	m2, cmd = updateModel(m, TransactionSaveResultMsg{ID: "999"})
	msgs := runCmd(cmd)
	if m2.created || m2.lastDate == "" {
		t.Errorf("created=%v lastDate=%q", m2.created, m2.lastDate)
	}
	if r, ok := findMsg[RefreshTransactionsMsg](msgs); !ok || r.TrxID != "999" {
		t.Errorf("expected refresh selecting 999, got %v", msgs)
	}
}

func TestForm_SaveCmdCallsApi(t *testing.T) {
	api := newMockAPI()
	m := newFormWith(t, api, domain.Transaction{}, true)
	m.data.categoryID, m.data.amount = "201", "3"
	res, ok := findMsg[TransactionSaveResultMsg](runCmd(m.saveCmd()))
	if !ok || res.ID != "999" || len(api.created) != 1 || res.Updated {
		t.Fatalf("create result = %+v", res)
	}

	m = newFormWith(t, api, api.txs[0], false)
	res, _ = findMsg[TransactionSaveResultMsg](runCmd(m.saveCmd()))
	if !res.Updated || len(api.updated) != 1 || api.updated[0].ID != "1" {
		t.Fatalf("update result = %+v", res)
	}
}

func TestForm_EditTags(t *testing.T) {
	hidden := domain.Tag{ID: "99", Name: "hidden"}
	tests := []struct {
		name   string
		tags   []domain.Tag
		change func(d *formData)
		want   []string
	}{
		{"keeps existing", []domain.Tag{tagTrip}, func(*formData) {}, []string{"7"}},
		{"adds", []domain.Tag{tagTrip}, func(d *formData) { d.tagIDs = append(d.tagIDs, "8") }, []string{"7", "8"}},
		{"removes all", []domain.Tag{tagTrip}, func(d *formData) { d.tagIDs = []string{} }, []string{}},
		{"preserves hidden", []domain.Tag{hidden, tagTrip}, func(d *formData) { d.tagIDs = []string{} }, []string{"99"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newMockAPI()
			tx := api.txs[0]
			tx.Tags = tt.tags
			m := newFormWith(t, api, tx, false)
			if slices.Contains(m.data.tagIDs, hidden.ID) {
				t.Fatal("hidden tag must not be bound to the multi-select")
			}
			tt.change(m.data)
			m.UpdateForm() // redraws must not lose or duplicate tags
			req, err := m.buildRequest()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(req.TagIDs, tt.want) {
				t.Errorf("TagIDs = %v, want %v", req.TagIDs, tt.want)
			}
		})
	}
}

func TestForm_TagsFieldShowsSelection(t *testing.T) {
	api := newMockAPI()
	tx := api.txs[0]
	tx.Tags = []domain.Tag{tagWork}
	m := newFormWith(t, api, tx, false)
	if view := m.form.View(); !strings.Contains(view, "✓ work") {
		t.Errorf("selected tag not rendered as checked:\n%s", view)
	}
}

func TestForm_ConfirmDiscard(t *testing.T) {
	api := newMockAPI()
	m := newModelTransaction(api)
	msgs := runCmd(m.confirmLoad(api.txs[0], false))
	if _, ok := findMsg[LoadTransactionMsg](msgs); !ok {
		t.Error("without unsaved data the transaction loads directly")
	}
	m.created = true
	if _, ok := findMsg[LoadTransactionMsg](runCmd(m.confirmLoad(api.txs[0], false))); ok {
		t.Error("with unsaved data a confirmation is required")
	}
}
