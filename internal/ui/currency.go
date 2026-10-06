/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"slices"
	"strings"

	"github.com/spf13/viper"

	"ezbk-tui/internal/domain"
)

const convertTotalsKey = "ui.convert_totals"

// CurrencyModeMsg switches totals between converted (default currency) and
// per-currency breakdown.
type CurrencyModeMsg struct{ Converted bool }

// convertTotalsSetting reads ui.convert_totals; converted is the default.
func convertTotalsSetting() bool {
	if !viper.IsSet(convertTotalsKey) {
		return true
	}
	return viper.GetBool(convertTotalsKey)
}

// converted is a total in the target currency plus what could not be converted.
type converted struct {
	total    domain.Money
	left     domain.Amounts
	currency string
}

func convertAmounts(a domain.Amounts, api CurrencyAPI) converted {
	target := api.DefaultCurrency()
	total, left := a.ConvertTo(target, api.ExchangeRates())
	return converted{total: total, left: left, currency: target}
}

// approx reports whether some amounts could not be converted.
func (c converted) approx() bool { return len(c.left) > 0 }

// value renders the converted total, prefixed with "~" when partial.
func (c converted) value(v domain.Money) string {
	s := v.Format(c.currency)
	if c.approx() {
		return "~" + s
	}
	return s
}

// formatTotal renders a total for single-line rows: converted with leftover
// currencies appended, or the plain per-currency breakdown.
func formatTotal(a domain.Amounts, api CurrencyAPI, convert bool) string {
	primary := api.DefaultCurrency()
	if !convert {
		return a.Format(primary)
	}
	c := convertAmounts(a, api)
	s := c.value(c.total)
	if c.approx() {
		s += ", " + c.left.Format(primary)
	}
	return s
}

// currencyView carries what list builders need to show amounts in the
// default currency. With convert false, amounts stay in native currencies.
type currencyView struct {
	primary string
	rates   domain.ExchangeRates
	convert bool
}

func newCurrencyView(api CurrencyAPI, convert bool) currencyView {
	return currencyView{primary: api.DefaultCurrency(), rates: api.ExchangeRates(), convert: convert}
}

// DefaultCurrency and ExchangeRates let currencyView act as a CurrencyAPI.
func (v currencyView) DefaultCurrency() string             { return v.primary }
func (v currencyView) ExchangeRates() domain.ExchangeRates { return v.rates }

// sortKey returns the value used to order multi-currency totals: the
// converted sum in converted mode, otherwise the primary-currency part.
func (v currencyView) sortKey(a domain.Amounts) domain.Money {
	if !v.convert {
		return a.Get(v.primary)
	}
	total, _ := a.ConvertTo(v.primary, v.rates)
	return total
}

// missingCurrencies returns the sorted union of unconverted currencies.
func missingCurrencies(parts ...domain.Amounts) []string {
	out := []string{}
	for _, p := range parts {
		for c := range p {
			if !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	slices.Sort(out)
	return out
}

func missingRatesWarning(missing []string) string {
	return "No exchange rate for " + strings.Join(missing, ", ") + "; shown separately"
}
