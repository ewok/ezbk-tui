/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"fmt"
	"os"
	"testing"
	"time"

	"ezbk-tui/internal/domain"
)

const testPrefix = "ezbk-tui-test-"

func integrationApi(t *testing.T) *Api {
	t.Helper()
	url, token := os.Getenv("EZBK_API_URL"), os.Getenv("EZBK_TOKEN")
	if url == "" || token == "" {
		t.Skip("EZBK_API_URL and EZBK_TOKEN environment variables required")
	}
	api, err := NewApi(Config{ApiUrl: url, Token: token, Timezone: os.Getenv("EZBK_TIMEZONE"), TimeoutSeconds: 15})
	if err != nil {
		t.Fatalf("NewApi: %v", err)
	}
	return api
}

func TestIntegration_ReadAll(t *testing.T) {
	api := integrationApi(t)
	steps := []struct {
		name string
		fn   func() error
	}{
		{"accounts", api.UpdateAccounts},
		{"categories", api.UpdateCategories},
		{"tags", api.UpdateTags},
		{"templates", api.UpdateTemplates},
		{"period stats", api.UpdatePeriodStats},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			if err := s.fn(); err != nil {
				t.Fatal(err)
			}
		})
	}
	txs, err := api.ListTransactions("")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("user=%s currency=%s accounts=%d expenseCats=%d tags=%d templates=%d tx(month)=%d",
		api.Username(), api.DefaultCurrency(), len(api.Accounts()),
		len(api.Categories(domain.CategoryExpense)), len(api.Tags()), len(api.Templates()), len(txs))
}

func TestIntegration_TransactionLifecycle(t *testing.T) {
	if os.Getenv("EZBK_WRITE_TESTS") != "1" {
		t.Skip("EZBK_WRITE_TESTS=1 required for write tests")
	}
	api := integrationApi(t)
	if err := api.UpdateAccounts(); err != nil {
		t.Fatal(err)
	}
	if err := api.UpdateCategories(); err != nil {
		t.Fatal(err)
	}

	var account domain.Account
	for _, a := range api.Accounts() {
		if !a.IsLiability {
			account = a
			break
		}
	}
	var category domain.Category
	for _, c := range api.Categories(domain.CategoryExpense) {
		if !c.IsPrimary() {
			category = c
			break
		}
	}
	if account.IsEmpty() || category.IsEmpty() {
		t.Skip("need at least one asset account and one secondary expense category")
	}

	comment := fmt.Sprintf("%s%d", testPrefix, time.Now().Unix())
	req := domain.TransactionRequest{
		Type:            domain.TxExpense,
		Time:            time.Now(),
		CategoryID:      category.ID,
		SourceAccountID: account.ID,
		SourceAmount:    1,
		Comment:         comment,
	}
	id, err := api.CreateTransaction(req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		if err := api.DeleteTransaction(id); err != nil {
			t.Errorf("cleanup delete %s: %v", id, err)
		}
	})

	req.ID = id
	req.SourceAmount = 2
	if _, err := api.UpdateTransaction(req); err != nil {
		t.Fatalf("update: %v", err)
	}

	found, err := api.ListTransactions(comment)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].SourceAmount != 2 {
		t.Fatalf("search after update = %+v", found)
	}
}
