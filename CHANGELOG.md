# Changelog

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
