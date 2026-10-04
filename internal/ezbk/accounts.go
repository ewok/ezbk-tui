/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"fmt"
	"net/url"
	"strings"

	"ezbk-tui/internal/domain"
)

// UpdateAccounts reloads accounts, flattening sub-accounts. Hidden accounts are
// kept only for resolving names in transactions.
func (a *Api) UpdateAccounts() error {
	var infos []*accountInfo
	if err := a.client.get("accounts/list.json", url.Values{}, &infos); err != nil {
		return fmt.Errorf("failed to load accounts: %w", err)
	}
	all := flattenAccounts(infos)
	visible := make([]domain.Account, 0, len(all))
	byID := make(map[string]domain.Account, len(all))
	allByID := make(map[string]domain.Account, len(all))
	for _, acc := range all {
		allByID[acc.ID] = acc
		if !acc.Hidden {
			visible = append(visible, acc)
			byID[acc.ID] = acc
		}
	}
	a.mu.Lock()
	a.accounts = visible
	a.accountByID = byID
	a.allAccountByID = allByID
	a.mu.Unlock()
	return nil
}

func flattenAccounts(infos []*accountInfo) []domain.Account {
	accounts := []domain.Account{}
	for _, info := range infos {
		if info == nil {
			continue
		}
		if len(info.SubAccounts) == 0 {
			accounts = append(accounts, toAccount(info, ""))
			continue
		}
		for _, sub := range info.SubAccounts {
			if sub == nil {
				continue
			}
			if sub.Category == 0 {
				sub.Category = info.Category
			}
			sub.Hidden = sub.Hidden || info.Hidden
			accounts = append(accounts, toAccount(sub, info.Name))
		}
	}
	return accounts
}

func toAccount(info *accountInfo, parentName string) domain.Account {
	balance, _ := domain.ParseCents(info.Balance)
	category := domain.AccountCategory(info.Category)
	isLiability := info.IsLiability || category.IsLiability()
	parentID := info.ParentID
	if parentID == "0" {
		parentID = ""
	}
	return domain.Account{
		ID:          info.ID,
		Name:        info.Name,
		ParentID:    parentID,
		ParentName:  parentName,
		Category:    category,
		Currency:    info.Currency,
		Balance:     balance,
		IsAsset:     !isLiability,
		IsLiability: isLiability,
		Hidden:      info.Hidden,
		Comment:     info.Comment,
	}
}

// Accounts returns all visible transaction-capable accounts.
func (a *Api) Accounts() []domain.Account {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]domain.Account, len(a.accounts))
	copy(out, a.accounts)
	return out
}

// AccountByID returns a cached account.
func (a *Api) AccountByID(id string) (domain.Account, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	acc, ok := a.accountByID[id]
	return acc, ok
}

// NetWorth returns asset and liability totals by currency.
func (a *Api) NetWorth() (assets, liabilities domain.Amounts) {
	assets, liabilities = domain.Amounts{}, domain.Amounts{}
	for _, acc := range a.Accounts() {
		if acc.IsLiability {
			liabilities.Add(acc.Currency, acc.Balance)
		} else {
			assets.Add(acc.Currency, acc.Balance)
		}
	}
	return assets, liabilities
}

// CreateAccount creates a single account with default icon and color.
func (a *Api) CreateAccount(name string, category domain.AccountCategory, currency string) error {
	req := accountCreateRequest{
		Name:     strings.TrimSpace(name),
		Category: int(category),
		Type:     1,
		Icon:     defaultIcon,
		Color:    defaultColor,
		Currency: strings.ToUpper(strings.TrimSpace(currency)),
	}
	if err := a.client.post("accounts/add.json", req, nil); err != nil {
		return fmt.Errorf("failed to create account %q: %w", name, err)
	}
	return nil
}
