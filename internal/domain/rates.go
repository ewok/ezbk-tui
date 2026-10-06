/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package domain

import (
	"fmt"
	"math/big"
	"time"
)

// ExchangeRates holds rates relative to a base currency: rate[c] is how many
// units of c equal one unit of Base. Rates are exact rationals, never floats.
type ExchangeRates struct {
	Base    string
	Updated time.Time
	rates   map[string]*big.Rat
}

// NewExchangeRates parses decimal rate strings. Invalid or non-positive rates
// are skipped and reported in the returned error; valid ones are still kept.
func NewExchangeRates(base string, updated time.Time, raw map[string]string) (ExchangeRates, error) {
	r := ExchangeRates{Base: base, Updated: updated, rates: map[string]*big.Rat{}}
	var bad []string
	for c, s := range raw {
		v, ok := new(big.Rat).SetString(s)
		if !ok || v.Sign() <= 0 {
			bad = append(bad, c+"="+s)
			continue
		}
		r.rates[c] = v
	}
	if len(bad) > 0 {
		return r, fmt.Errorf("invalid exchange rates: %v", bad)
	}
	return r, nil
}

// IsEmpty reports whether no rates are loaded.
func (r ExchangeRates) IsEmpty() bool {
	return len(r.rates) == 0
}

func (r ExchangeRates) rate(c string) (*big.Rat, bool) {
	if v, ok := r.rates[c]; ok {
		return v, true
	}
	if c != "" && c == r.Base {
		return big.NewRat(1, 1), true
	}
	return nil, false
}

// Convert converts m from one currency to another, rounding half away from
// zero to cents. It returns false when a rate is missing.
func (r ExchangeRates) Convert(m Money, from, to string) (Money, bool) {
	if from == to {
		return m, true
	}
	rf, ok := r.rate(from)
	if !ok {
		return 0, false
	}
	rt, ok := r.rate(to)
	if !ok {
		return 0, false
	}
	v := new(big.Rat).SetInt64(int64(m))
	v.Mul(v, rt)
	v.Quo(v, rf)
	return roundRat(v), true
}

// roundRat rounds to the nearest integer, half away from zero.
func roundRat(v *big.Rat) Money {
	num := new(big.Int).Abs(v.Num())
	den := v.Denom()
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if rem.Lsh(rem, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if v.Sign() < 0 {
		q.Neg(q)
	}
	return Money(q.Int64())
}

// ConvertTo sums all amounts converted to target. Non-zero amounts that cannot
// be converted are returned in unconverted (empty when everything converted).
func (a Amounts) ConvertTo(target string, rates ExchangeRates) (total Money, unconverted Amounts) {
	unconverted = Amounts{}
	for c, v := range a {
		if v == 0 {
			continue
		}
		if conv, ok := rates.Convert(v, c, target); ok {
			total += conv
			continue
		}
		unconverted.Add(c, v)
	}
	return total, unconverted
}
