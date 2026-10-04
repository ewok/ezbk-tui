/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package domain

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Money is an amount in minor units (cents), matching the ezBookkeeping API.
type Money int64

// ParseMoney parses a decimal string like "12.34", "-5", "1,234.5" into cents.
func ParseMoney(s string) (Money, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0, errors.New("empty amount")
	}
	neg := false
	switch s[0] {
	case '-':
		neg = true
		s = s[1:]
	case '+':
		s = s[1:]
	}
	intPart, fracPart, hasDot := strings.Cut(s, ".")
	if intPart == "" && (!hasDot || fracPart == "") {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	if len(fracPart) > 2 {
		return 0, fmt.Errorf("amount %q has more than two decimals", s)
	}
	for len(fracPart) < 2 {
		fracPart += "0"
	}
	if intPart == "" {
		intPart = "0"
	}
	i, err := strconv.ParseUint(intPart, 10, 63)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", s, err)
	}
	f, err := strconv.ParseUint(fracPart, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", s, err)
	}
	v := Money(i*100 + f)
	if neg {
		v = -v
	}
	return v, nil
}

// ParseCents parses an integer cents string as returned by the API (e.g. "1234").
func ParseCents(s string) (Money, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cents value %q: %w", s, err)
	}
	return Money(v), nil
}

// String formats the amount with two decimals, e.g. "-12.34".
func (m Money) String() string {
	sign := ""
	v := int64(m)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// Cents returns the raw integer amount as string, as expected by the API.
func (m Money) Cents() string {
	return strconv.FormatInt(int64(m), 10)
}

// Format returns "12.34 USD".
func (m Money) Format(currency string) string {
	if currency == "" {
		return m.String()
	}
	return m.String() + " " + currency
}

// Amounts is a multi-currency sum.
type Amounts map[string]Money

// Add adds v in the given currency.
func (a Amounts) Add(currency string, v Money) {
	a[currency] += v
}

// Currencies returns the currency codes sorted, with primary first if present.
func (a Amounts) Currencies(primary string) []string {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(x, y string) int {
		if x == primary {
			return -1
		}
		if y == primary {
			return 1
		}
		return strings.Compare(x, y)
	})
	return keys
}

// IsZero reports whether all amounts are zero.
func (a Amounts) IsZero() bool {
	for _, v := range a {
		if v != 0 {
			return false
		}
	}
	return true
}

// Format renders "12.00 USD, 3.50 EUR" (primary currency first).
func (a Amounts) Format(primary string) string {
	parts := []string{}
	for _, c := range a.Currencies(primary) {
		if a[c] == 0 {
			continue
		}
		parts = append(parts, a[c].Format(c))
	}
	if len(parts) == 0 {
		return Money(0).Format(primary)
	}
	return strings.Join(parts, ", ")
}

// Get returns the amount in the given currency.
func (a Amounts) Get(currency string) Money {
	return a[currency]
}
