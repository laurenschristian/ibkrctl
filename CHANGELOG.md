# Changelog

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.4.1] - 2026-09-22

### Fixed
- `modify` could not change a live order at all. The gateway requires `conid` on
  its modify endpoint, which is a whole-order replace, so every call failed with
  `HTTP 400: conid or conidex is required`. `modify` now reads the order back off
  the live book and sends the conid plus the side, order type, TIF, quantity and
  price the caller did not override. The display order type ("Limit") is mapped
  back to the API code ("LMT").
- `modify` on a filled or cancelled order reached IBKR with quantity 0 and came
  back as `Order size 0 is not valid`. Terminal orders are now refused up front
  with the order's actual status.
- Errors leaked the real account number. The request path embeds the account id
  and errors carry the path, a route `redact` never covered, so `account redact on`
  did not hold on any failure. Errors are now redacted on both the CLI and MCP
  surfaces.
- `orders` reported "no orders" while orders were live. A cold call returns
  `{"orders":[],"snapshot":false}` because the gateway has not built its cache;
  only a follow-up call sees the book. `orders` now retries once on that response.
- `cancel` rejected `--confirm` with `unknown flag`, although `place` and `modify`
  both require it. `cancel` accepts and ignores it.


## [0.1.0] - 2026-09-14

### Added
- Gateway lifecycle: `gateway install|start|stop|restart|status|uninstall`. Install
  copies a Client Portal Gateway and a bundled JRE into the support dir, patches the
  listen port to 5001, and loads always-on gateway + 60s keepalive launchd agents.
  Install also strips the stock conf's malformed deny IP that 404s the login page.
- Auto-login: `init` stores the IBKR username and password (macOS Keychain), and
  `login` drives a headless Chrome to fill them, then waits for the 2FA approval.
  `--show`, `--manual`, and `--otp` variants included.
- Session + data commands: `status`, `tickle`, `account`, `positions`, `pnl`,
  `orders`, `quote`, `chain`, `place --confirm`, `cancel`, `raw`, `logout`.
- MCP server with read tools: `ibkr_status`, `ibkr_accounts`, `ibkr_positions`,
  `ibkr_summary`, `ibkr_pnl`, `ibkr_orders`, `ibkr_quote`, `ibkr_chain`, `ibkr_raw`.
  Order placement stays CLI-only.
- Market and research data: `summary`, `ledger`, `allocation`, `trades`,
  `search`, `info`, `history`, `fundamentals`, `scanner`, with matching MCP
  read tools.
- Privacy: `account autoname` / `account alias` map real account ids to stable
  aliases, and `account redact on` strips real account numbers and holder names
  from every command and MCP tool. The agent only ever sees `account-1`.
- More markets and data: `watchlists` (+ get/create/delete), `news`,
  `notifications`, `fx` (exchange rates), `futures`, `alerts`, and
  `transactions`, with matching MCP read tools.
- Order decision tools: `place --preview` (whatif: commission, margin impact,
  post-trade position without submitting), `rules` (valid order types and
  increments), `position` (single contract), and `modify` (change a live
  order). 28 MCP tools total.
- `gateway install` reuses an existing install (no bundled source needed once
  copied) and prefers the configured JRE.

### Notes
- Alert creation is not available (the gateway returns 403); alerts are
  read-only (list/get/delete). `pa/summary` returns 401. Both reachable via
  `raw` if that changes.
