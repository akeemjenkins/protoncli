# Proton Mail Bridge reference

This document summarizes how our IMAP client is aligned with the [Proton Mail Bridge](https://github.com/ProtonMail/proton-bridge) codebase (see `proton-bridge/` in this repo or upstream).

## IMAP client: go-imap v1 (same as Bridge)

We use **go-imap v1.2.1** (same as Bridge’s `go.mod`: `github.com/emersion/go-imap v1.2.1`) so the wire protocol and LIST behavior match Bridge’s tests. Bridge tests use `client.Dial(addr)` then `client.Login()`; we use `Dial(addr)` → `StartTLS()` when the server advertises STARTTLS → `Login()`.

## Connection

| Setting   | Bridge source                    | Our default / behavior |
|----------|-----------------------------------|-------------------------|
| Host     | `constants.Host` = `"127.0.0.1"` | `PM_IMAP_HOST` default `127.0.0.1` |
| Port     | `ports.FindFreePortFrom(1143)`   | `PM_IMAP_PORT` default `1143` |
| Security | `Settings.IMAPSSL` = false        | STARTTLS (upgrade after connect on 1143) |
| TLS cert | `certs.NewTLSTemplate()` uses CN `"127.0.0.1"` | Loopback → we default to skip TLS verify so Bridge’s local cert works |

Flow: `client.Dial(addr)` (plain TCP) → `c.SupportStartTLS()` then `c.StartTLS(tlsConfig)` → `c.Login(user, pass)` → `c.List("", "*", channel)` (same as `proton-bridge/tests/imap_test.go` clientList).

## SELECT, SEARCH, FETCH

We use the same go-imap v1 calls as Bridge tests:

- **SELECT:** `client.Select(mailbox, true)` (read-only). Same as Bridge’s `clientFetch` which uses `client.Select(mailbox, false)` for read-write; we use read-only for fetch-and-parse.
- **UID SEARCH:** `client.UidSearch(criteria)` with `imap.SearchCriteria` (e.g. `SentSince`, `WithoutFlags: []string{\\Seen}`).
- **UID FETCH:** `client.UidFetch(seqSet, items, channel)` with `[]imap.FetchItem{imap.FetchFlags, imap.FetchEnvelope, imap.FetchUid, "BODY.PEEK[]"}` — same items as `proton-bridge/tests/imap_test.go` `clientFetch` and `clientFetchSequence`. Message body is read via `imap.ParseBodySectionName("BODY[]")` and `msg.GetBody(section)` (same as Bridge’s `newMessageFromIMAP` in `tests/types_test.go`).

## Mailbox hierarchy

Bridge exposes Proton folders and labels as IMAP mailboxes (see `internal/services/imapservice/connector.go`):

- **Folders** parent: mailbox name `"Folders"` (`folderPrefix`).
- **Labels** parent: mailbox name `"Labels"` (`labelPrefix`).

User labels appear as children under `Labels` (e.g. `Labels/Work`). Our `DetectLabelsRoot` looks for a mailbox named `"Labels"` (case-insensitive).

## LIST command

Bridge’s IMAP server is provided by [gluon](https://github.com/ProtonMail/gluon). We use **plain LIST** (`LIST "" "*"` with no options) so we don’t rely on LIST-EXTENDED / LIST-STATUS, which can cause connection drops with some Bridge/gluon versions.

## Applying labels via IMAP

In Proton Mail, a message has **one folder** (e.g. INBOX, Folders/Accounts) and **zero or more labels** (metadata/tags). Labels are not folders — you apply a label to a message; you don’t “move” the message into a label.

Bridge exposes labels as IMAP mailboxes under `Labels/` (e.g. `Labels/Newsletter`). To apply a label to a message, the client sends **COPY** (or MOVE) from the message’s folder to that label’s mailbox; Bridge translates this to “add this label to the message” (the message stays in its folder and gains the label). Label mailboxes live under the `Labels` parent (e.g. `Labels/Work`).

### IMAP CREATE is implemented in Bridge

Bridge **does** implement IMAP CREATE for labels. See `proton-bridge/internal/services/imapservice/connector.go`:

- **CreateMailbox** (line 194) accepts `name []string` (e.g. `["Labels", "Newsletter"]` for `Labels/Newsletter`).
- For `Labels/...` it calls **createLabel** (line 553), which calls **s.client.CreateLabel(ctx, proton.CreateLabelReq{Name: name[0], Color: "#f66", Type: proton.LabelTypeLabel})** — i.e. the Proton API to create the label.
- Constants: `folderPrefix = "Folders"`, `labelPrefix = "Labels"` (lines 540–541).

So `client.Create("Labels/Newsletter")` creates the label via the Proton API if it does not exist.

### Our usage (same as Bridge tests)

- **CREATE:** `client.Create("Labels/Newsletter")` to create the label if missing (Bridge → Proton CreateLabel).
- **UID COPY:** Select the message’s folder, then `client.UidCopy(seqSet, "Labels/Newsletter")`. Bridge treats this as **applying** the label to the message (the message stays in its folder and gains the label); we are not moving the message into a second folder.
