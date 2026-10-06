/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package domain

import (
	"testing"
	"time"
)

func testRates(t *testing.T) ExchangeRates {
	t.Helper()
	// Real-world shape from the server: base UZS, rates per 1 UZS.
	r, err := NewExchangeRates("UZS", time.Time{}, map[string]string{
		"USD": "0.0849008146233163",
		"EUR": "0.07576371721041024",
		"RUB": "7.228044813877847",
		"GBP": "0.5",
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestNewExchangeRates_Invalid(t *testing.T) {
	r, err := NewExchangeRates("USD", time.Time{}, map[string]string{"EUR": "0.9", "BAD": "x", "ZERO": "0"})
	if err == nil {
		t.Error("expected error for invalid rates")
	}
	if _, ok := r.Convert(100, "EUR", "USD"); !ok {
		t.Error("valid rates must be kept")
	}
	if _, ok := r.Convert(100, "ZERO", "USD"); ok {
		t.Error("zero rate must be skipped")
	}
}

func TestExchangeRates_Convert(t *testing.T) {
	r := testRates(t)
	tests := []struct {
		name     string
		m        Money
		from, to string
		want     Money
		ok       bool
	}{
		{"same currency", 1234, "XYZ", "XYZ", 1234, true},
		{"to base", 100, "GBP", "UZS", 200, true},
		{"from base", 200, "UZS", "GBP", 100, true},
		{"cross rate", 10000, "EUR", "USD", 11206, true},
		{"rub to usd", 850000, "RUB", "USD", 9984, true},
		{"negative", -10000, "EUR", "USD", -11206, true},
		{"round half up", 1, "UZS", "GBP", 1, true},
		{"round half negative", -1, "UZS", "GBP", -1, true},
		{"round down", 1, "GBP", "EUR", 0, true},
		{"missing from", 100, "KGS", "USD", 0, false},
		{"missing to", 100, "USD", "KGS", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := r.Convert(tt.m, tt.from, tt.to)
			if got != tt.want || ok != tt.ok {
				t.Errorf("Convert(%d, %s, %s) = %d, %v; want %d, %v", tt.m, tt.from, tt.to, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestAmounts_ConvertTo(t *testing.T) {
	r := testRates(t)
	tests := []struct {
		name      string
		in        Amounts
		want      Money
		wantLeft  Amounts
		emptyRate bool
	}{
		{"empty", Amounts{}, 0, Amounts{}, false},
		{"all convertible", Amounts{"USD": 100, "GBP": 100}, 100 + 17, Amounts{}, false},
		{"partial", Amounts{"USD": 100, "KGS": 500, "KZT": 0}, 100, Amounts{"KGS": 500}, false},
		{"no rates, target only", Amounts{"USD": 100, "EUR": 5}, 100, Amounts{"EUR": 5}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rates := r
			if tt.emptyRate {
				rates = ExchangeRates{}
			}
			got, left := tt.in.ConvertTo("USD", rates)
			if got != tt.want || len(left) != len(tt.wantLeft) {
				t.Fatalf("got %d %v, want %d %v", got, left, tt.want, tt.wantLeft)
			}
			for c, v := range tt.wantLeft {
				if left[c] != v {
					t.Errorf("left[%s] = %d, want %d", c, left[c], v)
				}
			}
		})
	}
}
