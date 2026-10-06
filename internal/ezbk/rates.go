/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"ezbk-tui/internal/domain"
)

// UpdateExchangeRates loads the server's latest exchange rates.
// On failure the previously cached rates are kept.
func (a *Api) UpdateExchangeRates() error {
	var res latestExchangeRates
	if err := a.client.get("exchange_rates/latest.json", nil, &res); err != nil {
		return fmt.Errorf("failed to load exchange rates: %w", err)
	}
	raw := make(map[string]string, len(res.ExchangeRates))
	for _, r := range res.ExchangeRates {
		raw[r.Currency] = r.Rate
	}
	rates, err := domain.NewExchangeRates(res.BaseCurrency, time.Unix(res.UpdateTime, 0), raw)
	if err != nil {
		zap.L().Warn("some exchange rates were skipped", zap.Error(err))
	}
	a.mu.Lock()
	a.rates = rates
	a.mu.Unlock()
	return nil
}

// ExchangeRates returns the cached exchange rates (empty if never loaded).
func (a *Api) ExchangeRates() domain.ExchangeRates {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rates
}
