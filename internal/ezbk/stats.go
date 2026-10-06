/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"net/url"
	"strconv"

	"go.uber.org/zap"

	"ezbk-tui/internal/domain"
)

// PeriodStats are totals of the selected period, computed client-side from
// the period's transactions. Amounts are grouped by account currency.
type PeriodStats struct {
	Income     domain.Amounts
	Expense    domain.Amounts
	ByCategory map[string]domain.Amounts
	TagIncome  map[string]domain.Amounts
	TagExpense map[string]domain.Amounts
}

func newPeriodStats() PeriodStats {
	return PeriodStats{
		Income:     domain.Amounts{},
		Expense:    domain.Amounts{},
		ByCategory: map[string]domain.Amounts{},
		TagIncome:  map[string]domain.Amounts{},
		TagExpense: map[string]domain.Amounts{},
	}
}

// UpdatePeriodStats reloads the period's transactions and recomputes totals.
// Exchange rates are refreshed too; a rate failure only logs a warning.
func (a *Api) UpdatePeriodStats() error {
	if err := a.UpdateExchangeRates(); err != nil {
		zap.L().Warn("keeping cached exchange rates", zap.Error(err))
	}
	params := url.Values{
		"start_time": {strconv.FormatInt(a.PeriodStart().Unix(), 10)},
		"end_time":   {strconv.FormatInt(a.PeriodEnd().Unix(), 10)},
	}
	txs, err := a.listTransactions(params)
	if err != nil {
		return err
	}
	stats := computeStats(txs)
	a.mu.Lock()
	a.stats = stats
	a.mu.Unlock()
	return nil
}

func computeStats(txs []domain.Transaction) PeriodStats {
	s := newPeriodStats()
	add := func(m map[string]domain.Amounts, key, currency string, v domain.Money) {
		if key == "" {
			return
		}
		if m[key] == nil {
			m[key] = domain.Amounts{}
		}
		m[key].Add(currency, v)
	}
	for _, tx := range txs {
		var total domain.Amounts
		var byTag map[string]domain.Amounts
		switch tx.Type {
		case domain.TxIncome:
			total, byTag = s.Income, s.TagIncome
		case domain.TxExpense:
			total, byTag = s.Expense, s.TagExpense
		default:
			continue
		}
		cur := tx.Source.Currency
		total.Add(cur, tx.SourceAmount)
		add(s.ByCategory, tx.Category.ID, cur, tx.SourceAmount)
		add(s.ByCategory, tx.Category.ParentID, cur, tx.SourceAmount)
		for _, tag := range tx.Tags {
			add(byTag, tag.ID, cur, tx.SourceAmount)
		}
	}
	return s
}

// PeriodIncome returns total income of the period by currency.
func (a *Api) PeriodIncome() domain.Amounts {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stats.Income
}

// PeriodExpense returns total expense of the period by currency.
func (a *Api) PeriodExpense() domain.Amounts {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stats.Expense
}

// CategoryTotal returns the period total of a category (primary includes children).
func (a *Api) CategoryTotal(id string) domain.Amounts {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return orEmpty(a.stats.ByCategory[id])
}

// TagTotals returns the period expense and income for a tag.
func (a *Api) TagTotals(id string) (expense, income domain.Amounts) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return orEmpty(a.stats.TagExpense[id]), orEmpty(a.stats.TagIncome[id])
}

func orEmpty(a domain.Amounts) domain.Amounts {
	if a == nil {
		return domain.Amounts{}
	}
	return a
}
