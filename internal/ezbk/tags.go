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

// UpdateTags reloads visible tags.
func (a *Api) UpdateTags() error {
	var infos []*tagInfo
	if err := a.client.get("transaction/tags/list.json", nil, &infos); err != nil {
		return fmt.Errorf("failed to load tags: %w", err)
	}
	tags := []domain.Tag{}
	byID := map[string]domain.Tag{}
	for _, info := range infos {
		if info == nil || info.Hidden {
			continue
		}
		tag := domain.Tag{ID: info.ID, Name: info.Name, GroupID: info.GroupID}
		tags = append(tags, tag)
		byID[tag.ID] = tag
	}
	a.mu.Lock()
	a.tags = tags
	a.tagByID = byID
	a.mu.Unlock()
	return nil
}

// Tags returns visible tags.
func (a *Api) Tags() []domain.Tag {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]domain.Tag, len(a.tags))
	copy(out, a.tags)
	return out
}

// CreateTag creates a tag without group.
func (a *Api) CreateTag(name string) error {
	req := tagCreateRequest{GroupID: "0", Name: strings.TrimSpace(name)}
	if err := a.client.post("transaction/tags/add.json", req, nil); err != nil {
		return fmt.Errorf("failed to create tag %q: %w", name, err)
	}
	return nil
}

// UpdateTemplates reloads normal (non scheduled) transaction templates.
func (a *Api) UpdateTemplates() error {
	var infos []*templateInfo
	params := url.Values{"templateType": {"1"}}
	if err := a.client.get("transaction/templates/list.json", params, &infos); err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}
	visible := make([]*templateInfo, 0, len(infos))
	for _, info := range infos {
		if info != nil && !info.Hidden {
			visible = append(visible, info)
		}
	}
	a.mu.Lock()
	a.templates = visible
	a.mu.Unlock()
	return nil
}

// Templates returns cached templates; references are resolved on each call
// so that names are correct even if templates loaded before accounts.
func (a *Api) Templates() []domain.Template {
	a.mu.RLock()
	infos := a.templates
	a.mu.RUnlock()
	out := make([]domain.Template, 0, len(infos))
	for _, info := range infos {
		out = append(out, domain.Template{
			ID:          info.ID,
			Name:        info.Name,
			Transaction: a.toTransaction(&info.transactionInfo),
		})
	}
	return out
}
