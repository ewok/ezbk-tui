/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package domain

import "testing"

func TestParseMoney(t *testing.T) {
	tests := []struct {
		in      string
		want    Money
		wantErr bool
	}{
		{"12.34", 1234, false},
		{"12", 1200, false},
		{"12.3", 1230, false},
		{".5", 50, false},
		{"-7.05", -705, false},
		{"+1", 100, false},
		{"1,234.56", 123456, false},
		{" 3.00 ", 300, false},
		{"0", 0, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1.234", 0, true},
		{".", 0, true},
		{"1.2.3", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseMoney(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseMoney(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseMoney(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestMoneyString(t *testing.T) {
	tests := []struct {
		in   Money
		want string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{1234, "12.34"},
		{-1234, "-12.34"},
		{-5, "-0.05"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAmountsFormat(t *testing.T) {
	a := Amounts{}
	if got := a.Format("USD"); got != "0.00 USD" {
		t.Errorf("empty Format = %q", got)
	}
	a.Add("EUR", 350)
	a.Add("USD", 1200)
	a.Add("CHF", 0)
	if got := a.Format("USD"); got != "12.00 USD, 3.50 EUR" {
		t.Errorf("Format = %q", got)
	}
	if a.IsZero() {
		t.Error("IsZero = true, want false")
	}
}

func TestParseAccountCategory(t *testing.T) {
	tests := []struct {
		in      string
		want    AccountCategory
		wantErr bool
	}{
		{"cash", AccountCash, false},
		{"Credit Card", AccountCreditCard, false},
		{"credit-card", AccountCreditCard, false},
		{"savings", AccountSavings, false},
		{"nope", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseAccountCategory(tt.in)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("ParseAccountCategory(%q) = %v, %v", tt.in, got, err)
			}
		})
	}
}
