# protoncli

A local-only CLI that classifies, labels, and organizes a Proton Mail inbox using a locally hosted Ollama model — no data leaves your machine.

[![CI](https://github.com/akeemjenkins/protoncli/actions/workflows/ci.yml/badge.svg)](https://github.com/akeemjenkins/protoncli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/akeemjenkins/protoncli?label=release&logo=github)](https://github.com/akeemjenkins/protoncli/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/akeemjenkins/protoncli.svg)](https://pkg.go.dev/github.com/akeemjenkins/protoncli)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-blue?logo=go)](https://go.dev/)

## What it does

`protoncli` connects to a running [Proton Mail Bridge](https://proton.me/mail/bridge) over IMAP and routes each message through a local [Ollama](https://ollama.com/) model for classification. The model proposes one or more labels from an 11-category taxonomy (Orders, Finance, Newsletters, etc.) and the CLI applies those labels back to the mailbox. Every step runs on `localhost`: no API keys, no cloud inference, no telemetry. Output is structured JSON designed for pipelines and LLM-driven workflows.

## Why you might want it

- Keep mail triage private: classification happens on your hardware against a model you control.
- Replace hand-rolled Proton Mail filter rules with an LLM that reads subject, sender, and body context.
- Script against a stable JSON/NDJSON contract with typed errors and deterministic exit codes.

## Quickstart

1. Install and sign into [Proton Mail Bridge](https://proton.me/mail/bridge). Note the IMAP username and bridge password it assigns.

2. Install [Ollama](https://ollama.com/download) and pull a model:

   ```bash
   ollama pull gemma4
   ```

3. Install `protoncli` (pick one):

   ```bash
   # Homebrew (upcoming)
   brew install akeemjenkins/tap/protoncli

   # Release tarball
   curl -L https://github.com/akeemjenkins/protoncli/releases/latest/download/protoncli_0.1.0_darwin_arm64.tar.gz | tar xz

   # go install
   go install github.com/akeemjenkins/protoncli/cmd/protoncli@latest
   ```

   Or build from source:

   ```bash
   git clone https://github.com/akeemjenkins/protoncli.git
   cd personal-proton
   make build
   ```

4. Export the Bridge and Ollama settings. A `.env` at the repo root works:

   ```bash
   export PM_IMAP_USERNAME="alice@proton.me"
   export PM_IMAP_PASSWORD="bridge-generated-password"
   export PM_IMAP_HOST="127.0.0.1"
   export PM_IMAP_PORT="1143"
   export PM_OLLAMA_MODEL="gemma4"
   ```

5. Dry-run a classification pass against your inbox:

   ```bash
   protoncli classify --dry-run --limit 20 INBOX
   ```

   Review the NDJSON output. When the suggestions look right, rerun with `--apply` to write labels back to Proton.

## Usage

The intended flow is: discover mailboxes, classify them, apply labels. Everything else (`backfill`, `cleanup-labels`, `state`) exists to repair or inspect state along the way.

### scan-folders

Enumerate IMAP mailboxes and return the canonical `All Mail` and `Labels` roots as a single JSON object.

```bash
protoncli scan-folders
```

```json
{
  "mailboxes": [
    {"name": "INBOX", "delimiter": "/", "messages": 1204, "unseen": 18, "attributes": []},
    {"name": "Labels/Orders", "delimiter": "/", "attributes": ["\\HasNoChildren"]}
  ],
  "all_mail": "All Mail",
  "labels_root": "Labels"
}
```

### classify

Stream messages through Ollama and emit one NDJSON object per message, followed by a `summary` terminator. Add `--apply` to write labels back to IMAP in the same pass.

```bash
protoncli classify --dry-run --limit 20 INBOX
protoncli classify --apply --workers 4 "Folders/Accounts"
protoncli classify --reprocess --no-state INBOX
```

Flags: `--dry-run`, `--apply`, `--limit N`, `--no-state`, `--reprocess`, `--workers N`.

```json
{"mailbox":"INBOX","uid":1842,"uid_validity":1,"subject":"Your order has shipped","from":"ship-confirm@amazon.com","date":"2026-04-09T14:22:10Z","suggested_labels":["Orders"],"confidence":0.94,"rationale":"Shipping notification with tracking number","is_mailing_list":false}
{"type":"summary","mailbox":"INBOX","classified":20,"errors":0,"skipped":0}
```

### apply-labels

Apply pending labels recorded in the state DB to messages in a mailbox. Use this when `classify` ran without `--apply`.

```bash
protoncli apply-labels --limit 100 "Folders/Accounts"
protoncli apply-labels --dry-run "Folders/Accounts"
```

```json
{"uid":1842,"labels":["Labels/Orders"],"applied":true}
{"type":"summary","processed":100,"skipped":0,"failed":0,"reconnects":0}
```

### cleanup-labels

Consolidate legacy or user-created labels into the canonical 11-label taxonomy. Useful after migrating from Proton's built-in filters.

```bash
protoncli cleanup-labels --dry-run
protoncli cleanup-labels
```

```json
{"label":"Labels/shipping","canonical":"Labels/Orders","messages_moved":37,"status":"ok"}
{"type":"summary","processed":12,"skipped":1,"failed":0,"reconnects":0}
```

### fetch-and-parse

Fetch and parse messages from a mailbox without classifying. Useful for piping into other tools.

```bash
protoncli fetch-and-parse INBOX | jq 'select(.from | contains("github"))'
```

### backfill

Replay a prior `classify` NDJSON log into the state DB. Recovers state after a crash or migration.

```bash
protoncli backfill classify.log
```

```json
{"inserted":482,"skipped":3,"errors":1,"totals":{"total":482,"applied":478,"failed":4}}
```

### state

Inspect or reset the SQLite state DB.

```bash
protoncli state stats
protoncli state stats "Folders/Accounts"
protoncli state clear "Folders/Accounts"
```

```json
{
  "db": "/Users/alice/.protoncli/state.db",
  "mailboxes": [
    {"name": "Folders/Accounts", "total": 500, "applied": 480, "failed": 5}
  ],
  "total": {"total": 500, "applied": 480, "failed": 5}
}
```

### schema

Print machine-readable command metadata as JSON. See [Schema-driven tool use](#schema-driven-tool-use).

```bash
protoncli schema
protoncli schema classify
```

## Output contract

- **stdout**: structured JSON. Single-object commands pretty-print; streaming commands emit NDJSON (one JSON object per line).
- **stderr**: human-readable progress and diagnostics, sanitized to strip ANSI escapes, bidi controls, and zero-width characters.
- **NDJSON terminator**: every streaming command ends with a single `{"type":"summary", ...}` line. Consumers should read until they see it.
- **Error envelope**: failures — whole-command or per-row — emit a typed envelope so consumers can branch on `.error.kind`.

```json
{
  "error": {
    "kind": "config",
    "code": 400,
    "reason": "configError",
    "message": "PM_IMAP_USERNAME is required",
    "hint": "export PM_IMAP_USERNAME=<bridge-username>"
  }
}
```

## Exit codes

| Code | Kind | Description |
|---|---|---|
| 0 | success | Command completed without error |
| 1 | api | Upstream API error (generic) |
| 2 | auth | Authentication or credential failure |
| 3 | validation | Invalid flags, arguments, or input |
| 4 | config | Missing or malformed configuration |
| 5 | imap | IMAP protocol or connection error |
| 6 | classify | Classification error (Ollama, prompt, parsing) |
| 7 | state | State DB error (SQLite) |
| 8 | discovery | Mailbox discovery failure |
| 9 | internal | Unexpected internal error |

## Configuration

All configuration is read from environment variables. Defaults target a standard Proton Mail Bridge + Ollama setup on the same host.

### Credentials

Credentials can be stored in the OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service via libsecret) so they never need to live in shell history or a `.env` file. When the keyring is unreachable, an encrypted-file fallback (`~/.protoncli/credentials.enc`, AES-256-GCM with Argon2id-derived keys) is available.

```sh
# Prompt interactively and store via the OS keyring (default).
protoncli auth login

# Pipe the password in from a secret manager:
pass show proton/bridge | protoncli auth login --username alice@proton.me --password-stdin

# Check where credentials live and which backends are available.
protoncli auth status

# Remove stored credentials from every backend.
protoncli auth logout
```

If `PM_IMAP_USERNAME` and/or `PM_IMAP_PASSWORD` are set in the environment they always win — useful for one-off overrides in CI or shells. To use the encrypted-file backend, export `PM_KEYSTORE_PASSPHRASE` (required to read or write the file) and optionally `PM_KEYSTORE_PATH` to relocate it.

### IMAP (Proton Mail Bridge)

| Variable | Default | Description |
|---|---|---|
| `PM_IMAP_HOST` | `127.0.0.1` | Bridge host |
| `PM_IMAP_PORT` | `1143` | Bridge IMAP port |
| `PM_IMAP_USERNAME` | *(required)* | Bridge IMAP username |
| `PM_IMAP_PASSWORD` | *(required)* | Bridge-generated password |
| `PM_IMAP_SECURITY` | `starttls` | One of `starttls`, `tls`, `insecure` |
| `PM_IMAP_TLS_SKIP_VERIFY` | auto | Skip TLS verification (auto-enabled for loopback) |
| `PM_IMAP_APPLY_TIMEOUT` | `180` | Per-command timeout (seconds) for `--apply` |

### Ollama

| Variable | Default | Description |
|---|---|---|
| `PM_OLLAMA_BASE_URL` | `http://localhost:11434` | Ollama API base URL |
| `PM_OLLAMA_MODEL` | `gemma4` | Model name passed to Ollama |

### Classify tuning

| Variable | Default | Description |
|---|---|---|
| `PM_CLASSIFY_LIMIT` | `100` | Max messages per `classify` run |
| `PM_CLASSIFY_BATCH_SIZE` | `25` | IMAP fetch batch size |
| `PM_CLASSIFY_WORKERS` | `4` | Parallel Ollama workers |

### State

| Variable | Default | Description |
|---|---|---|
| `PM_STATE_DB` | `~/.protoncli/state.db` | SQLite state DB path |

## Labels

The classifier is constrained to 11 canonical labels. 612 aliases in `internal/labels/data/labels.toml` normalize legacy or model-generated names back to this set.

| Label | Covers |
|---|---|
| Orders | Purchase confirmations, shipping, returns |
| Finance | Banks, cards, taxes, invoices |
| Newsletters | Editorial digests, blog mailings |
| Promotions | Marketing, discounts, sales |
| Jobs | Recruiters, job boards, offers |
| Social | Social network notifications, friend activity |
| Services | SaaS account activity, product updates |
| Health | Providers, pharmacy, insurance |
| Travel | Flights, hotels, itineraries |
| Security | 2FA, password resets, security alerts |
| Signups | Account creation, email verification |

See [`internal/labels/data/labels.toml`](internal/labels/data/labels.toml) for the full alias map.

## Schema-driven tool use

`protoncli schema` emits a JSON manifest of every subcommand — flags, positional arguments, stdout format, and the exit codes it may emit. LLM agents can load the manifest once and drive the CLI without prompt-engineering command syntax; wrappers and shell completions can be generated from it. Combine the manifest with the stable exit codes above to branch deterministically (e.g. retry on `5 imap`, prompt the user on `2 auth`, surface to the caller on `4 config`).

```bash
protoncli schema classify
```

```json
{
  "name": "classify",
  "summary": "Classify messages with Ollama and optionally apply labels",
  "args": [{"name": "mailbox", "required": false, "default": "INBOX"}],
  "flags": [
    {"name": "dry-run", "type": "bool", "description": "Preview suggestions without writing labels"},
    {"name": "apply", "type": "bool", "description": "Apply suggested labels to IMAP"},
    {"name": "limit", "type": "int", "default": 100},
    {"name": "workers", "type": "int", "default": 4},
    {"name": "no-state", "type": "bool"},
    {"name": "reprocess", "type": "bool"}
  ],
  "stdout": "ndjson",
  "exit_codes": [0, 2, 3, 4, 5, 6, 7, 9]
}
```

## Development

Every common task is wrapped in the repo `Makefile`.

| Target | Description |
|---|---|
| `make build` | Build the `./bin/protoncli` binary |
| `make test` | Run `go test ./...` |
| `make test-race` | Run tests with the race detector |
| `make cover` | Coverage profile plus `go tool cover -func` summary |
| `make cover-html` | HTML coverage report at `coverage.html` |
| `make vet` | Run `go vet ./...` |
| `make vuln` | Run `govulncheck` (installs into `./bin` if missing) |
| `make check` | `vet` + `test-race` + `vuln` |
| `make clean` | Remove the built binary and coverage outputs |
| `make tidy` | Run `go mod tidy` |

Run `make check` before opening a PR.

## Contributing

Bug reports and pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for the workflow and code-review expectations.

## Security

Please report vulnerabilities privately via the process in [SECURITY.md](SECURITY.md). Do not open public issues for security reports.

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## Acknowledgements

- [Proton Mail Bridge](https://github.com/ProtonMail/proton-bridge) — local IMAP gateway to Proton Mail
- [Ollama](https://github.com/ollama/ollama) — local LLM runtime
- [emersion/go-imap](https://github.com/emersion/go-imap) — IMAP client library
- [spf13/cobra](https://github.com/spf13/cobra) — CLI framework
- [BurntSushi/toml](https://github.com/BurntSushi/toml) — TOML decoder for the label taxonomy
