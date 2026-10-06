/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/huh"

	"ezbk-tui/internal/domain"
)

// formOptions are prebuilt once per redraw so that dynamic option funcs stay cheap.
type formOptions struct {
	accounts    []domain.Account
	accountByID map[string]domain.Account
	categories  map[domain.CategoryType][]huh.Option[string]
	tags        []huh.Option[string]
	tagIDs      map[string]bool
}

func (m *modelTransaction) buildFormOptions() *formOptions {
	opts := &formOptions{
		accounts:    m.api.Accounts(),
		accountByID: map[string]domain.Account{},
		categories:  map[domain.CategoryType][]huh.Option[string]{},
		tagIDs:      map[string]bool{},
	}
	for _, acc := range opts.accounts {
		opts.accountByID[acc.ID] = acc
	}
	for _, t := range []domain.CategoryType{domain.CategoryExpense, domain.CategoryIncome, domain.CategoryTransfer} {
		for _, c := range m.api.Categories(t) {
			if !c.IsPrimary() {
				opts.categories[t] = append(opts.categories[t], huh.NewOption(c.DisplayName(), c.ID))
			}
		}
	}
	for _, tag := range m.api.Tags() {
		opts.tags = append(opts.tags, huh.NewOption(tag.Name, tag.ID))
		opts.tagIDs[tag.ID] = true
	}
	return opts
}

func accountLabel(acc domain.Account) string {
	return fmt.Sprintf("%s (%s)", acc.DisplayName(), acc.Currency)
}

func (o *formOptions) accountOptions(excludeID string) []huh.Option[string] {
	out := make([]huh.Option[string], 0, len(o.accounts))
	for _, acc := range o.accounts {
		if acc.ID != excludeID {
			out = append(out, huh.NewOption(accountLabel(acc), acc.ID))
		}
	}
	if len(out) == 0 {
		out = append(out, huh.NewOption("No accounts — create one first", ""))
	}
	return out
}

func (o *formOptions) categoryOptions(t domain.TransactionType) []huh.Option[string] {
	out := o.categories[t.CategoryType()]
	if len(out) == 0 {
		return []huh.Option[string]{huh.NewOption("No "+t.CategoryType().String()+" sub-categories", "")}
	}
	return out
}

// UpdateForm rebuilds the huh form from the current data.
func (m *modelTransaction) UpdateForm() {
	m.opts = m.buildFormOptions()
	d := m.data
	if d.sourceID == "" && len(m.opts.accounts) > 0 {
		d.sourceID = m.opts.accounts[0].ID
	}
	d.splitTagIDs(m.opts.tagIDs)
	m.form = huh.NewForm(
		m.mainGroup(),
		m.amountGroup(),
		m.detailsGroup(),
		m.dateGroup(),
	).WithLayout(huh.LayoutGrid(2, 2)).WithShowHelp(false)
}

func (m *modelTransaction) mainGroup() *huh.Group {
	d := m.data
	return huh.NewGroup(
		huh.NewSelect[domain.TransactionType]().
			Title("Type").
			Options(
				huh.NewOption("Expense", domain.TxExpense),
				huh.NewOption("Income", domain.TxIncome),
				huh.NewOption("Transfer", domain.TxTransfer),
			).
			Value(&d.txType).
			Inline(true),
		huh.NewSelect[string]().
			Title("Account").
			TitleFunc(func() string {
				if d.txType == domain.TxTransfer {
					return "From account"
				}
				return "Account"
			}, &d.txType).
			Options(m.opts.accountOptions("")...).
			Value(&d.sourceID).
			Validate(requireValue("account")).
			WithHeight(6),
		huh.NewSelect[string]().
			Title("To account").
			Options(m.destOptions()...).
			OptionsFunc(m.destOptions, []any{&d.txType, &d.sourceID}).
			Value(&d.destID).
			Validate(func(id string) error {
				if d.txType == domain.TxTransfer && id == "" {
					return errors.New("select the destination account")
				}
				return nil
			}).
			WithHeight(5),
	)
}

func (m *modelTransaction) destOptions() []huh.Option[string] {
	if m.data.txType != domain.TxTransfer {
		return []huh.Option[string]{huh.NewOption("— (transfers only)", "")}
	}
	return m.opts.accountOptions(m.data.sourceID)
}

func (m *modelTransaction) amountGroup() *huh.Group {
	d := m.data
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Category").
			Options(m.opts.categoryOptions(d.txType)...).
			OptionsFunc(func() []huh.Option[string] {
				return m.opts.categoryOptions(d.txType)
			}, &d.txType).
			Value(&d.categoryID).
			Validate(requireValue("category")).
			WithHeight(6),
		huh.NewInput().
			Title("Amount "+m.sourceAccount().Currency).
			TitleFunc(func() string {
				return "Amount " + m.sourceAccount().Currency
			}, &d.sourceID).
			Value(&d.amount).
			Validate(validateAmount),
		huh.NewInput().
			Title("To amount").
			TitleFunc(func() string {
				if !m.needsDestAmount() {
					return "To amount (N/A)"
				}
				return "To amount " + m.destAccount().Currency
			}, []any{&d.txType, &d.sourceID, &d.destID}).
			Value(&d.destAmount).
			Validate(func(s string) error {
				if !m.needsDestAmount() {
					return nil
				}
				return validateAmount(s)
			}),
	)
}

func (m *modelTransaction) detailsGroup() *huh.Group {
	d := m.data
	fields := []huh.Field{
		huh.NewInput().
			Title("Comment").
			Value(&d.comment).
			CharLimit(maxCommentLength).
			WithWidth(30),
	}
	if len(m.opts.tags) > 0 {
		// Value must be bound before Options: huh only scrolls to the
		// selected options when they are known at Options() time.
		fields = append(fields, huh.NewMultiSelect[string]().
			Title("Tags").
			Description("space/x toggle · enter next").
			Value(&d.tagIDs).
			Options(m.opts.tags...).
			Limit(max(1, maxTags-len(d.keptTagIDs))).
			Height(7))
	} else {
		fields = append(fields, huh.NewNote().Title("Tags").Description("No tags (create in Tags tab: o, n)"))
	}
	return huh.NewGroup(fields...)
}

func (m *modelTransaction) dateGroup() *huh.Group {
	d := m.data
	now := time.Now()
	years := []string{}
	for y := now.Year() - 9; y <= now.Year()+1; y++ {
		years = append(years, strconv.Itoa(y))
	}
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Year").
			Options(huh.NewOptions(years...)...).
			Value(&d.year).
			WithHeight(3),
		huh.NewSelect[string]().
			Title("Month").
			Options(huh.NewOptions("01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12")...).
			Value(&d.month).
			WithHeight(4),
		huh.NewSelect[string]().
			Title("Day").
			Options(huh.NewOptions(d.day)...).
			OptionsFunc(func() []huh.Option[string] {
				month, _ := strconv.Atoi(d.month)
				year, _ := strconv.Atoi(d.year)
				days := []string{}
				for day := 1; day <= daysIn(month, year); day++ {
					days = append(days, fmt.Sprintf("%02d", day))
				}
				return huh.NewOptions(days...)
			}, []any{&d.month, &d.year}).
			Value(&d.day).
			WithHeight(4),
	)
}

func requireValue(name string) func(string) error {
	return func(v string) error {
		if v == "" {
			return fmt.Errorf("select a %s", name)
		}
		return nil
	}
}
