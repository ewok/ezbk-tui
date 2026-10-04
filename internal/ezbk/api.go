/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"fmt"
	"sync"
	"time"

	"ezbk-tui/internal/domain"
)

const (
	defaultIcon  = "1"
	defaultColor = "000000"
)

// Api is the cached ezBookkeeping backend used by the UI.
type Api struct {
	client *Client
	mu     sync.RWMutex

	profile     userProfile
	periodStart time.Time
	periodEnd   time.Time
	timeout     int

	accounts       []domain.Account
	accountByID    map[string]domain.Account
	allAccountByID map[string]domain.Account
	categories     []domain.Category
	categoryByID   map[string]domain.Category
	tags           []domain.Tag
	tagByID        map[string]domain.Tag
	templates      []*templateInfo
	stats          PeriodStats
}

// NewApi connects to ezBookkeeping, verifies the token and sets the period to the current month.
func NewApi(cfg Config) (*Api, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return newApiWithClient(client, cfg.TimeoutSeconds)
}

func newApiWithClient(client *Client, timeout int) (*Api, error) {
	a := &Api{
		client:         client,
		timeout:        timeout,
		accountByID:    map[string]domain.Account{},
		allAccountByID: map[string]domain.Account{},
		categoryByID:   map[string]domain.Category{},
		tagByID:        map[string]domain.Tag{},
		stats:          newPeriodStats(),
	}
	now := time.Now().In(client.Location())
	a.SetPeriod(now.Year(), now.Month())

	if err := a.UpdateProfile(); err != nil {
		return nil, explainAuthError(err, client.token, client.apiTokensEnabled())
	}
	return a, nil
}

// UpdateProfile loads the user profile (also used as connection check).
func (a *Api) UpdateProfile() error {
	var p userProfile
	if err := a.client.get("users/profile/get.json", nil, &p); err != nil {
		return fmt.Errorf("failed to load user profile: %w", err)
	}
	a.mu.Lock()
	a.profile = p
	a.mu.Unlock()
	return nil
}

// Username returns the logged in user name.
func (a *Api) Username() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.profile.Username
}

// DefaultCurrency returns the user's default currency.
func (a *Api) DefaultCurrency() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.profile.DefaultCurrency
}

// DefaultAccountID returns the user's default account, if any.
func (a *Api) DefaultAccountID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.profile.DefaultAccount == "0" {
		return ""
	}
	return a.profile.DefaultAccount
}

// TimeoutSeconds returns the configured timeout.
func (a *Api) TimeoutSeconds() int {
	if a.timeout <= 0 {
		return 10
	}
	return a.timeout
}

// Location returns the configured timezone.
func (a *Api) Location() *time.Location {
	return a.client.Location()
}

// SetPeriod selects a calendar month.
func (a *Api) SetPeriod(year int, month time.Month) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, a.client.Location())
	a.mu.Lock()
	a.periodStart = start
	a.periodEnd = start.AddDate(0, 1, 0).Add(-time.Second)
	a.mu.Unlock()
}

// PeriodStart returns the first second of the selected month.
func (a *Api) PeriodStart() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.periodStart
}

// PeriodEnd returns the last second of the selected month.
func (a *Api) PeriodEnd() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.periodEnd
}
