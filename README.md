# ezbk-tui

A terminal user interface for [ezBookkeeping](https://ezbookkeeping.mayswind.net), the self-hosted
personal finance app. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea); the UI is
derived from [ffiii-tui](https://github.com/ewok/ffiii-tui) (Firefly III TUI) and adapted to
ezBookkeeping's model: accounts with sub-accounts, two-level income/expense/transfer categories,
tags and transaction templates.

## Features

- Monthly transaction list with period picker, client-side filters (account, category, tag, text)
  and all-time keyword search
- Create / edit / delete expense, income and transfer transactions (cross-currency transfers
  with a separate destination amount); read-only transactions are protected
- Accounts tab: balances of all accounts and sub-accounts, net worth per currency, create accounts
- Expense and Income tabs: category tree with monthly totals, create categories and sub-categories
- Tags tab: monthly spent/earned per tag, create tags, tag picker in the form
- Quick-add from ezBookkeeping transaction templates
- Summary panel: net worth, income, expense and balance per currency

## Requirements

- ezBookkeeping with API tokens enabled: set `enable_api_token = true` in the `[security]` section
  (or `EBK_SECURITY_ENABLE_API_TOKEN=true`).
- An **API** token: *User Settings → Security → Generate Token* (choose the API type),
  or `ezbookkeeping userdata user-session-new`. MCP tokens do not work with the REST API
  (error 202004 "current token type is invalid").

## Install

```bash
go install ./...        # or
go build -o ezbk-tui .
```

## Configuration

`config.yaml` is looked up in the current directory and in `~/.config/ezbk-tui/`:

```yaml
ezbk:
  api_url: https://ezbookkeeping.example.com   # /api/v1 is appended automatically
  token: <your API token>
  timezone: Europe/Berlin                      # optional, defaults to the system timezone
timeout: 10
logging:
  debug: false
  file: /tmp/ezbk-tui.log
```

Generate one with `ezbk-tui init-config -u <url> -k <token>`. All keys can also be passed as flags
(`-u`, `-k`, `-z`, `-t`) or environment variables (`EZBK_TUI_EZBK_TOKEN`, ...).

## Keys

| Where | Keys |
|---|---|
| Global | `p` period picker, `?` help, `ctrl+c` quit |
| Tabs | `a` accounts, `e` expense, `i` income, `g` tags, `t` transactions |
| Transactions | `n` new, `N` copy selected, `T` from template, `enter` edit, `D` delete, `s` search, `/` filter, `ctrl+a` reset filters, `t` full view, `r` refresh |
| Lists | `f` filter by item (twice: exclusive), `enter` filter and jump to transactions, `n` new, `s` sort / hide empty, `/` search list |
| Form | `ctrl+s` save, `esc` back (form is kept, `esc` in list resumes), `ctrl+n` reset, `ctrl+e` edit again, `ctrl+t` last saved date |

In the Expense/Income tabs `n` pre-fills `<parent>/` from the selected category; enter
`Parent/Name` to create a sub-category (the parent is created if missing) or `Name` for a top-level one.
New accounts are entered as `<name>[,<currency>[,<category>]]`, e.g. `Visa,USD,credit`.

## Development

```bash
go test ./...
EZBK_API_URL=https://host EZBK_TOKEN=... go test ./internal/ezbk -run Integration -v
EZBK_WRITE_TESTS=1 EZBK_API_URL=... EZBK_TOKEN=... go test ./internal/ezbk -run Lifecycle -v
```

See [PLAN.md](PLAN.md) for design decisions and project state.

## License

Apache-2.0
