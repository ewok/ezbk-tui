/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"fmt"
	"strings"

	"ezbk-tui/internal/domain"
)

var categoryTypeOrder = []domain.CategoryType{
	domain.CategoryExpense, domain.CategoryIncome, domain.CategoryTransfer,
}

// UpdateCategories reloads all visible categories (primary followed by their children).
func (a *Api) UpdateCategories() error {
	var byType map[string][]*categoryInfo
	if err := a.client.get("transaction/categories/list.json", nil, &byType); err != nil {
		return fmt.Errorf("failed to load categories: %w", err)
	}
	categories := flattenCategories(byType)
	byID := make(map[string]domain.Category, len(categories))
	for _, c := range categories {
		byID[c.ID] = c
	}
	a.mu.Lock()
	a.categories = categories
	a.categoryByID = byID
	a.mu.Unlock()
	return nil
}

func flattenCategories(byType map[string][]*categoryInfo) []domain.Category {
	out := []domain.Category{}
	for _, t := range categoryTypeOrder {
		for _, primary := range byType[fmt.Sprint(int(t))] {
			if primary == nil {
				continue
			}
			out = append(out, toCategory(primary, t, ""))
			for _, sub := range primary.SubCategories {
				if sub == nil {
					continue
				}
				sub.Hidden = sub.Hidden || primary.Hidden
				out = append(out, toCategory(sub, t, primary.Name))
			}
		}
	}
	return out
}

func toCategory(info *categoryInfo, t domain.CategoryType, parentName string) domain.Category {
	parentID := info.ParentID
	if parentID == "0" {
		parentID = ""
	}
	return domain.Category{
		ID:         info.ID,
		Name:       info.Name,
		ParentID:   parentID,
		ParentName: parentName,
		Type:       t,
		Hidden:     info.Hidden,
		Comment:    info.Comment,
	}
}

// Categories returns visible categories of a type (primary then children).
func (a *Api) Categories(t domain.CategoryType) []domain.Category {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := []domain.Category{}
	for _, c := range a.categories {
		if c.Type == t && !c.Hidden {
			out = append(out, c)
		}
	}
	return out
}

// CategoryByID returns a cached category.
func (a *Api) CategoryByID(id string) (domain.Category, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c, ok := a.categoryByID[id]
	return c, ok
}

// CreateCategory creates a primary (parentID empty) or secondary category and returns its ID.
func (a *Api) CreateCategory(name string, t domain.CategoryType, parentID string) (string, error) {
	if parentID == "" {
		parentID = "0"
	}
	req := categoryCreateRequest{
		Name:     strings.TrimSpace(name),
		Type:     int(t),
		ParentID: parentID,
		Icon:     defaultIcon,
		Color:    defaultColor,
	}
	var created categoryInfo
	if err := a.client.post("transaction/categories/add.json", req, &created); err != nil {
		return "", fmt.Errorf("failed to create category %q: %w", name, err)
	}
	return created.ID, nil
}
