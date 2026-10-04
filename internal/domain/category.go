/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package domain

// CategoryType mirrors ezBookkeeping transaction category types.
type CategoryType int

const (
	CategoryIncome   CategoryType = 1
	CategoryExpense  CategoryType = 2
	CategoryTransfer CategoryType = 3
)

func (t CategoryType) String() string {
	switch t {
	case CategoryIncome:
		return "Income"
	case CategoryExpense:
		return "Expense"
	case CategoryTransfer:
		return "Transfer"
	}
	return "Unknown"
}

// Category is a transaction category. Primary categories have an empty ParentID;
// only secondary categories can be assigned to transactions.
type Category struct {
	ID         string
	Name       string
	ParentID   string
	ParentName string
	Type       CategoryType
	Hidden     bool
	Comment    string
}

// IsEmpty reports whether the category is the zero value.
func (c Category) IsEmpty() bool {
	return c.ID == ""
}

// IsPrimary reports whether the category is a top-level category.
func (c Category) IsPrimary() bool {
	return c.ParentID == ""
}

// DisplayName renders "Parent / Child" for secondary categories.
func (c Category) DisplayName() string {
	if c.ParentName != "" {
		return c.ParentName + " / " + c.Name
	}
	return c.Name
}

// Tag is a transaction tag.
type Tag struct {
	ID      string
	Name    string
	GroupID string
	Hidden  bool
}

// IsEmpty reports whether the tag is the zero value.
func (t Tag) IsEmpty() bool {
	return t.ID == ""
}
