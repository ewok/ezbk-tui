/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"time"

	"ezbk-tui/internal/domain"
)

type PeriodAPI interface {
	PeriodStart() time.Time
	PeriodEnd() time.Time
	SetPeriod(year int, month time.Month)
}

type CurrencyAPI interface {
	DefaultCurrency() string
}

type AccountsAPI interface {
	CurrencyAPI
	UpdateAccounts() error
	Accounts() []domain.Account
	CreateAccount(name string, category domain.AccountCategory, currency string) error
}

type StatsAPI interface {
	UpdatePeriodStats() error
}

type CategoriesAPI interface {
	CurrencyAPI
	UpdateCategories() error
	Categories(t domain.CategoryType) []domain.Category
	CategoryTotal(id string) domain.Amounts
	PeriodIncome() domain.Amounts
	PeriodExpense() domain.Amounts
	CreateCategory(name string, t domain.CategoryType, parentID string) (string, error)
}

type TagsAPI interface {
	CurrencyAPI
	UpdateTags() error
	Tags() []domain.Tag
	TagTotals(id string) (expense, income domain.Amounts)
	CreateTag(name string) error
}

type SummaryAPI interface {
	CurrencyAPI
	StatsAPI
	NetWorth() (assets, liabilities domain.Amounts)
	PeriodIncome() domain.Amounts
	PeriodExpense() domain.Amounts
}

type TransactionAPI interface {
	ListTransactions(query string) ([]domain.Transaction, error)
	DeleteTransaction(id string) error
}

type TransactionFormAPI interface {
	Accounts() []domain.Account
	Categories(t domain.CategoryType) []domain.Category
	Tags() []domain.Tag
	DefaultAccountID() string
	Location() *time.Location
	CreateTransaction(req domain.TransactionRequest) (string, error)
	UpdateTransaction(req domain.TransactionRequest) (string, error)
}

type TemplatesAPI interface {
	UpdateTemplates() error
	Templates() []domain.Template
}

// UIAPI is everything the root model needs.
type UIAPI interface {
	PeriodAPI
	AccountsAPI
	CategoriesAPI
	TagsAPI
	SummaryAPI
	TransactionAPI
	TransactionFormAPI
	TemplatesAPI
	TimeoutSeconds() int
}
