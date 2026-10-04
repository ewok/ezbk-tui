/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package domain

import (
	"slices"
	"strings"
	"time"
)

// TransactionType mirrors ezBookkeeping transaction types.
type TransactionType int

const (
	TxBalanceModification TransactionType = 1
	TxIncome              TransactionType = 2
	TxExpense             TransactionType = 3
	TxTransfer            TransactionType = 4
)

func (t TransactionType) String() string {
	switch t {
	case TxBalanceModification:
		return "Balance"
	case TxIncome:
		return "Income"
	case TxExpense:
		return "Expense"
	case TxTransfer:
		return "Transfer"
	}
	return "Unknown"
}

// Icon returns a short symbol for list display.
func (t TransactionType) Icon() string {
	switch t {
	case TxBalanceModification:
		return "≡"
	case TxIncome:
		return "→"
	case TxExpense:
		return "←"
	case TxTransfer:
		return "⇄"
	}
	return "?"
}

// CategoryType returns the category type that transactions of this type use.
func (t TransactionType) CategoryType() CategoryType {
	switch t {
	case TxIncome:
		return CategoryIncome
	case TxTransfer:
		return CategoryTransfer
	default:
		return CategoryExpense
	}
}

// Transaction is a single ezBookkeeping transaction with resolved references.
// For income and expense only Source is set (the account that changes).
type Transaction struct {
	ID                string
	Type              TransactionType
	Time              time.Time
	Category          Category
	Source            Account
	Destination       Account
	SourceAmount      Money
	DestinationAmount Money
	Tags              []Tag
	Comment           string
	Editable          bool
}

// TagIDs returns the IDs of the transaction tags.
func (t Transaction) TagIDs() []string {
	ids := make([]string, 0, len(t.Tags))
	for _, tag := range t.Tags {
		ids = append(ids, tag.ID)
	}
	return ids
}

// TagNames returns a comma separated list of tag names.
func (t Transaction) TagNames() string {
	names := make([]string, 0, len(t.Tags))
	for _, tag := range t.Tags {
		names = append(names, tag.Name)
	}
	return strings.Join(names, ", ")
}

// HasTag reports whether the transaction carries the tag.
func (t Transaction) HasTag(id string) bool {
	return slices.ContainsFunc(t.Tags, func(tag Tag) bool { return tag.ID == id })
}

// InvolvesAccount reports whether the account is source or destination.
func (t Transaction) InvolvesAccount(id string) bool {
	return t.Source.ID == id || (t.Type == TxTransfer && t.Destination.ID == id)
}

// Description returns the comment, or a generated summary when empty.
func (t Transaction) Description() string {
	if t.Comment != "" {
		return t.Comment
	}
	if t.Type == TxTransfer {
		return t.Source.Name + " -> " + t.Destination.Name
	}
	return t.Category.Name
}

// TransactionRequest is used to create (ID empty) or modify a transaction.
type TransactionRequest struct {
	ID                   string
	Type                 TransactionType
	Time                 time.Time
	CategoryID           string
	SourceAccountID      string
	DestinationAccountID string
	SourceAmount         Money
	DestinationAmount    Money
	TagIDs               []string
	Comment              string
}

// Template is a saved transaction template (normal templates only).
type Template struct {
	ID          string
	Name        string
	Transaction Transaction
}
