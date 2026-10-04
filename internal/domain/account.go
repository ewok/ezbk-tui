/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package domain

import (
	"fmt"
	"strings"
)

// AccountCategory mirrors ezBookkeeping account categories.
type AccountCategory int

const (
	AccountCash                 AccountCategory = 1
	AccountChecking             AccountCategory = 2
	AccountCreditCard           AccountCategory = 3
	AccountVirtual              AccountCategory = 4
	AccountDebt                 AccountCategory = 5
	AccountReceivables          AccountCategory = 6
	AccountInvestment           AccountCategory = 7
	AccountSavings              AccountCategory = 8
	AccountCertificateOfDeposit AccountCategory = 9
)

// AllAccountCategories lists categories in display order.
var AllAccountCategories = []AccountCategory{
	AccountCash, AccountChecking, AccountSavings, AccountCreditCard, AccountVirtual,
	AccountDebt, AccountReceivables, AccountInvestment, AccountCertificateOfDeposit,
}

var accountCategoryNames = map[AccountCategory]string{
	AccountCash:                 "Cash",
	AccountChecking:             "Checking",
	AccountCreditCard:           "Credit Card",
	AccountVirtual:              "Virtual",
	AccountDebt:                 "Debt",
	AccountReceivables:          "Receivables",
	AccountInvestment:           "Investment",
	AccountSavings:              "Savings",
	AccountCertificateOfDeposit: "Certificate of Deposit",
}

var accountCategoryAliases = map[string]AccountCategory{
	"cash":        AccountCash,
	"checking":    AccountChecking,
	"bank":        AccountChecking,
	"credit":      AccountCreditCard,
	"creditcard":  AccountCreditCard,
	"card":        AccountCreditCard,
	"virtual":     AccountVirtual,
	"debt":        AccountDebt,
	"receivables": AccountReceivables,
	"receivable":  AccountReceivables,
	"investment":  AccountInvestment,
	"savings":     AccountSavings,
	"saving":      AccountSavings,
	"deposit":     AccountCertificateOfDeposit,
	"cd":          AccountCertificateOfDeposit,
}

func (c AccountCategory) String() string {
	if n, ok := accountCategoryNames[c]; ok {
		return n
	}
	return fmt.Sprintf("Category %d", int(c))
}

// IsLiability reports whether the category is a liability per ezBookkeeping.
func (c AccountCategory) IsLiability() bool {
	return c == AccountCreditCard || c == AccountDebt
}

// ParseAccountCategory resolves a user-entered category name or alias.
func ParseAccountCategory(s string) (AccountCategory, error) {
	key := strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(s))
	if c, ok := accountCategoryAliases[key]; ok {
		return c, nil
	}
	return 0, fmt.Errorf("unknown account category %q", s)
}

// Account is a transaction-capable account (single account or sub-account).
type Account struct {
	ID          string
	Name        string
	ParentID    string
	ParentName  string
	Category    AccountCategory
	Currency    string
	Balance     Money
	IsAsset     bool
	IsLiability bool
	Hidden      bool
	Comment     string
}

// IsEmpty reports whether the account is the zero value.
func (a Account) IsEmpty() bool {
	return a.ID == ""
}

// DisplayName includes the parent account name for sub-accounts.
func (a Account) DisplayName() string {
	if a.ParentName != "" {
		return a.ParentName + " / " + a.Name
	}
	return a.Name
}
