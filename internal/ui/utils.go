/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ui

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func daysIn(m int, year int) int {
	return time.Date(year, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// Cmd wraps a message into a command.
func Cmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// CaseInsensitiveContains reports whether substr is within s, ignoring case.
func CaseInsensitiveContains(s, substr string) bool {
	return strings.Contains(strings.ToUpper(s), strings.ToUpper(substr))
}

func isDataMsg(msg tea.Msg) bool {
	switch msg.(type) {
	case RefreshAccountsMsg, AccountsUpdatedMsg,
		RefreshCategoriesMsg, CategoriesUpdatedMsg,
		RefreshTagsMsg, TagsUpdatedMsg,
		RefreshStatsMsg, StatsUpdatedMsg,
		RefreshTemplatesMsg, TemplatesUpdatedMsg,
		RefreshTransactionsMsg, TransactionsUpdateMsg,
		FilterMsg, DataLoadCompletedMsg:
		return true
	}
	return false
}

// refreshAfterWrite reloads everything that a transaction change affects.
func refreshAfterWrite(trxID string) tea.Cmd {
	return tea.Batch(
		Cmd(RefreshAccountsMsg{}),
		Cmd(RefreshStatsMsg{}),
		Cmd(RefreshTransactionsMsg{TrxID: trxID}),
	)
}

// loadTracker counts running background operations for the header spinner.
type loadTracker struct {
	count atomic.Int32
	ops   sync.Map
	seq   atomic.Uint64
}

var loading = &loadTracker{}

func startLoading(message string) string {
	loading.count.Add(1)
	opID := fmt.Sprintf("op_%d", loading.seq.Add(1))
	loading.ops.Store(opID, message)
	return opID
}

func stopLoading(opID string) {
	if _, ok := loading.ops.LoadAndDelete(opID); ok {
		loading.count.Add(-1)
	}
}

func isLoading() bool {
	return loading.count.Load() > 0
}

func buildLoadingMessage() string {
	var messages []string
	loading.ops.Range(func(_, value any) bool {
		if msg, ok := value.(string); ok {
			if runes := []rune(msg); len(runes) > 25 {
				msg = string(runes[:22]) + "..."
			}
			messages = append(messages, msg)
		}
		return true
	})
	switch n := len(messages); {
	case n == 0:
		return "..."
	case n == 1:
		return messages[0]
	case n <= 4:
		return fmt.Sprintf("(%d) %s", n, strings.Join(messages, " "))
	default:
		return fmt.Sprintf("(%d) %s | +%d more", n, strings.Join(messages[:4], " "), n-4)
	}
}
