/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"ezbk-tui/internal/domain"
)

// ListTransactions returns the transactions of the selected period, or, when
// query is not empty, all transactions matching the keyword (any time).
func (a *Api) ListTransactions(query string) ([]domain.Transaction, error) {
	params := url.Values{}
	query = strings.TrimSpace(query)
	if query == "" {
		params.Set("start_time", strconv.FormatInt(a.PeriodStart().Unix(), 10))
		params.Set("end_time", strconv.FormatInt(a.PeriodEnd().Unix(), 10))
	} else {
		params.Set("keyword", query)
		params.Set("match_mode", "1")
	}
	return a.listTransactions(params)
}

func (a *Api) listTransactions(params url.Values) ([]domain.Transaction, error) {
	params.Set("trim_account", "true")
	params.Set("trim_category", "true")
	params.Set("trim_tag", "true")

	var infos []*transactionInfo
	if err := a.client.get("transactions/list/all.json", params, &infos); err != nil {
		return nil, fmt.Errorf("failed to load transactions: %w", err)
	}
	out := make([]domain.Transaction, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		out = append(out, a.toTransaction(info))
	}
	slices.SortStableFunc(out, func(x, y domain.Transaction) int {
		return y.Time.Compare(x.Time)
	})
	return out, nil
}

func (a *Api) toTransaction(info *transactionInfo) domain.Transaction {
	tx := domain.Transaction{
		ID:           info.ID,
		Type:         domain.TransactionType(info.Type),
		Category:     a.resolveCategory(info.CategoryID),
		Source:       a.resolveAccount(info.SourceAccountID),
		SourceAmount: domain.Money(info.SourceAmount),
		Comment:      info.Comment,
		Editable:     info.Editable,
	}
	if info.Time > 0 {
		zone := time.FixedZone("", info.UtcOffset*60)
		tx.Time = time.Unix(info.Time, 0).In(zone)
	}
	if tx.Type == domain.TxTransfer {
		tx.Destination = a.resolveAccount(info.DestinationAccountID)
		tx.DestinationAmount = tx.SourceAmount
		if info.DestinationAmount != nil {
			tx.DestinationAmount = domain.Money(*info.DestinationAmount)
		}
	}
	for _, id := range info.TagIDs {
		tx.Tags = append(tx.Tags, a.resolveTag(id))
	}
	return tx
}

func (a *Api) resolveAccount(id string) domain.Account {
	if id == "" || id == "0" {
		return domain.Account{}
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if acc, ok := a.allAccountByID[id]; ok {
		return acc
	}
	return domain.Account{ID: id, Name: "#" + id}
}

func (a *Api) resolveCategory(id string) domain.Category {
	if id == "" || id == "0" {
		return domain.Category{}
	}
	if c, ok := a.CategoryByID(id); ok {
		return c
	}
	return domain.Category{ID: id, Name: "#" + id}
}

func (a *Api) resolveTag(id string) domain.Tag {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if t, ok := a.tagByID[id]; ok {
		return t
	}
	return domain.Tag{ID: id, Name: "#" + id}
}

// CreateTransaction creates a transaction and returns its ID.
func (a *Api) CreateTransaction(req domain.TransactionRequest) (string, error) {
	req.ID = ""
	var created transactionInfo
	if err := a.client.post("transactions/add.json", a.toWriteRequest(req), &created); err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}
	return created.ID, nil
}

// UpdateTransaction modifies an existing transaction.
func (a *Api) UpdateTransaction(req domain.TransactionRequest) (string, error) {
	if req.ID == "" {
		return "", errors.New("transaction ID is required for update")
	}
	var updated transactionInfo
	err := a.client.post("transactions/modify.json", a.toWriteRequest(req), &updated)
	if err != nil && !isNothingUpdated(err) {
		return "", fmt.Errorf("failed to update transaction %s: %w", req.ID, err)
	}
	return req.ID, nil
}

// DeleteTransaction deletes a transaction.
func (a *Api) DeleteTransaction(id string) error {
	if err := a.client.post("transactions/delete.json", idRequest{ID: id}, nil); err != nil {
		return fmt.Errorf("failed to delete transaction %s: %w", id, err)
	}
	return nil
}

func (a *Api) toWriteRequest(req domain.TransactionRequest) transactionWriteRequest {
	t := req.Time
	if t.IsZero() {
		t = time.Now()
	}
	_, offset := t.In(a.client.Location()).Zone()
	w := transactionWriteRequest{
		ID:                   req.ID,
		Type:                 int(req.Type),
		CategoryID:           idOrZero(req.CategoryID),
		Time:                 t.Unix(),
		UtcOffset:            offset / 60,
		SourceAccountID:      idOrZero(req.SourceAccountID),
		DestinationAccountID: "0",
		SourceAmount:         int64(req.SourceAmount),
		TagIDs:               req.TagIDs,
		PictureIDs:           []string{},
		Comment:              req.Comment,
	}
	if w.TagIDs == nil {
		w.TagIDs = []string{}
	}
	if req.Type == domain.TxTransfer {
		w.DestinationAccountID = idOrZero(req.DestinationAccountID)
		w.DestinationAmount = int64(req.DestinationAmount)
		if w.DestinationAmount == 0 {
			w.DestinationAmount = w.SourceAmount
		}
	}
	return w
}

func idOrZero(id string) string {
	if id == "" {
		return "0"
	}
	return id
}

func isNothingUpdated(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return strings.Contains(strings.ToLower(apiErr.Message), "nothing will be updated")
	}
	return false
}
