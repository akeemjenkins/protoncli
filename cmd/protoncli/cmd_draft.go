package main

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/spf13/cobra"

	"github.com/akeemjenkins/protoncli/internal/cerr"
	"github.com/akeemjenkins/protoncli/internal/config"
	"github.com/akeemjenkins/protoncli/internal/imapclient"
	"github.com/akeemjenkins/protoncli/internal/imapfetch"
	"github.com/akeemjenkins/protoncli/internal/imaputil"
	"github.com/akeemjenkins/protoncli/internal/smtp"
	"github.com/akeemjenkins/protoncli/internal/termio"
)

type draftFlags struct {
	from           string
	to             []string
	cc             []string
	bcc            []string
	subject        string
	bodyFile       string
	bodyHTMLFile   string
	inReplyToUID   string
	sourceMailbox  string
	draftsMailbox  string
	dryRun         bool
}

func newDraftCmd() *cobra.Command {
	f := &draftFlags{}
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Compose a message and save it to the Drafts mailbox via IMAP APPEND (does NOT send)",
		Long: `draft composes an RFC5322 message from the given flags and APPENDs it to
the Drafts mailbox with the \Draft flag set. This command does NOT send email
over SMTP — use an MUA or dedicated tool for outbound delivery.`,
		Annotations: map[string]string{
			"stdout_format": "json",
			"exit_codes":    "0,2,3,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdDraft(f)
		},
	}
	cmd.Flags().StringVar(&f.from, "from", "", "From address (defaults to PM_IMAP_USERNAME)")
	cmd.Flags().StringSliceVar(&f.to, "to", nil, "Recipient(s) — may repeat or use comma-separated list")
	cmd.Flags().StringSliceVar(&f.cc, "cc", nil, "Cc recipient(s)")
	cmd.Flags().StringSliceVar(&f.bcc, "bcc", nil, "Bcc recipient(s)")
	cmd.Flags().StringVar(&f.subject, "subject", "", "Subject")
	cmd.Flags().StringVar(&f.bodyFile, "body-file", "", "Path to UTF-8 plain-text body (use '-' for stdin)")
	cmd.Flags().StringVar(&f.bodyHTMLFile, "body-html-file", "", "Optional path to HTML body (makes message multipart/alternative)")
	cmd.Flags().StringVar(&f.inReplyToUID, "in-reply-to", "", "UID of source message to reply to (fetches its Message-ID)")
	cmd.Flags().StringVar(&f.sourceMailbox, "source-mailbox", "INBOX", "Mailbox containing the source message for --in-reply-to")
	cmd.Flags().StringVar(&f.draftsMailbox, "drafts-mailbox", "Drafts", "Target mailbox for the saved draft")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "Emit the composed RFC5322 bytes instead of APPENDing")
	return cmd
}

func cmdDraft(f *draftFlags) error {
	if len(f.to) == 0 && len(f.cc) == 0 && len(f.bcc) == 0 {
		return cerr.Validation("at least one of --to, --cc, or --bcc is required")
	}
	if strings.TrimSpace(f.bodyFile) == "" && strings.TrimSpace(f.bodyHTMLFile) == "" {
		return cerr.Validation("--body-file or --body-html-file is required")
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	from := strings.TrimSpace(f.from)
	if from == "" {
		from = cfg.IMAP.Username
	}
	if from == "" {
		return cerr.Validation("--from is required (or set PM_IMAP_USERNAME)")
	}

	bodyText, err := readBodyFile(f.bodyFile)
	if err != nil {
		return err
	}
	bodyHTML, err := readBodyFile(f.bodyHTMLFile)
	if err != nil {
		return err
	}

	to := flattenAddrs(f.to)
	cc := flattenAddrs(f.cc)
	bcc := flattenAddrs(f.bcc)

	env := smtp.Envelope{
		From:     from,
		To:       to,
		Cc:       cc,
		Bcc:      bcc,
		Subject:  f.subject,
		BodyText: bodyText,
		BodyHTML: bodyHTML,
	}

	// If replying, resolve reference headers from the source message.
	if strings.TrimSpace(f.inReplyToUID) != "" {
		ref, err := resolveInReplyTo(cfg, f.sourceMailbox, f.inReplyToUID)
		if err != nil {
			return err
		}
		env.InReplyTo = ref.MessageID
		env.References = ref.MessageID
		if ref.Subject != "" && !hasRePrefix(f.subject) {
			base := strings.TrimSpace(f.subject)
			if base == "" {
				base = ref.Subject
			}
			env.Subject = "Re: " + base
		}
	}

	raw, err := smtp.Build(env)
	if err != nil {
		return cerr.Validation("%s", err.Error())
	}

	w := termio.Default()
	if f.dryRun {
		return w.PrintJSON(map[string]any{
			"type":   "draft_preview",
			"bytes":  len(raw),
			"rfc822": string(raw),
		})
	}

	c, err := imapclient.Dial(cfg.IMAP)
	if err != nil {
		return cerr.Auth("%s", err.Error())
	}
	defer imapclient.CloseAndLogout(c)

	mboxes, err := imaputil.ListMailboxes(c, false)
	if err != nil {
		return cerr.IMAP(imapclient.Wrap(err), "LIST failed")
	}
	resolved, err := imaputil.ResolveMailboxName(f.draftsMailbox, mboxes)
	if err != nil {
		// If Drafts doesn't exist, attempt to create it.
		if createErr := c.Create(f.draftsMailbox); createErr != nil {
			return cerr.IMAP(imapclient.Wrap(createErr), "CREATE %q", f.draftsMailbox)
		}
		resolved = f.draftsMailbox
	}

	flags := []string{imap.DraftFlag, imap.SeenFlag}
	if err := c.Append(resolved, flags, time.Now(), bytes.NewReader(raw)); err != nil {
		return cerr.IMAP(imapclient.Wrap(err), "APPEND %q", resolved)
	}

	msgID := extractMessageID(raw)
	return w.PrintJSON(map[string]any{
		"drafted":    true,
		"mailbox":    resolved,
		"size":       len(raw),
		"message_id": msgID,
	})
}

// replyRef carries the fields we need from a source message to construct a reply.
type replyRef struct {
	MessageID string
	Subject   string
}

func resolveInReplyTo(cfg config.Config, mailbox, uidStr string) (replyRef, error) {
	var ref replyRef
	uid, err := strconv.ParseUint(strings.TrimSpace(uidStr), 10, 32)
	if err != nil {
		return ref, cerr.Validation("--in-reply-to must be a numeric UID: %s", err.Error())
	}

	c, err := imapclient.Dial(cfg.IMAP)
	if err != nil {
		return ref, cerr.Auth("%s", err.Error())
	}
	defer imapclient.CloseAndLogout(c)

	mboxes, err := imaputil.ListMailboxes(c, false)
	if err != nil {
		return ref, cerr.IMAP(imapclient.Wrap(err), "LIST failed")
	}
	resolved, err := imaputil.ResolveMailboxName(mailbox, mboxes)
	if err != nil {
		return ref, cerr.Validation("%s", err.Error())
	}
	if _, err := imapfetch.SelectMailbox(c, resolved); err != nil {
		return ref, cerr.IMAP(imapclient.Wrap(err), "SELECT %q", resolved)
	}
	msgs, err := imapfetch.FetchRFC822(c, []uint32{uint32(uid)})
	if err != nil {
		return ref, cerr.IMAP(imapclient.Wrap(err), "UID FETCH %d", uid)
	}
	if len(msgs) == 0 {
		return ref, cerr.Validation("no message with UID %d in %q", uid, resolved)
	}
	m := msgs[0]
	if m.Envelope != nil {
		ref.MessageID = m.Envelope.MessageId
		ref.Subject = m.Envelope.Subject
	}
	if ref.MessageID == "" {
		ref.MessageID = extractMessageID(m.RFC822)
	}
	return ref, nil
}

// extractMessageID pulls the Message-ID header value out of an RFC5322 byte
// slice. Returns "" if none is found.
func extractMessageID(raw []byte) string {
	// Scan headers until blank line.
	idx := bytes.Index(raw, []byte("\r\n\r\n"))
	if idx < 0 {
		idx = len(raw)
	}
	headers := string(raw[:idx])
	for _, line := range strings.Split(headers, "\r\n") {
		if len(line) >= 11 && strings.EqualFold(line[:11], "Message-ID:") {
			return strings.TrimSpace(line[11:])
		}
	}
	return ""
}

// hasRePrefix reports whether subject starts with a case-insensitive "Re:" prefix.
func hasRePrefix(subject string) bool {
	s := strings.TrimSpace(subject)
	if len(s) < 3 {
		return false
	}
	return strings.EqualFold(s[:3], "re:")
}

// flattenAddrs splits comma-separated entries and trims whitespace.
func flattenAddrs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// readBodyFile reads the named file. "-" means stdin. Empty path yields "".
func readBodyFile(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", nil
	}
	var r io.Reader
	if p == "-" {
		r = os.Stdin
	} else {
		fh, err := os.Open(p)
		if err != nil {
			return "", cerr.Validation("open body file %q: %s", p, err.Error())
		}
		defer fh.Close()
		r = fh
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", cerr.Internal(err, "read body %q", p)
	}
	if !utf8Valid(b) {
		return "", cerr.Validation("body file %q is not valid UTF-8", p)
	}
	return string(b), nil
}

func utf8Valid(b []byte) bool {
	// Simple UTF-8 validation: stdlib unicode/utf8 would work but to keep
	// imports light, use a manual scan.
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			i++
			continue
		}
		var size int
		switch {
		case c&0xE0 == 0xC0:
			size = 2
		case c&0xF0 == 0xE0:
			size = 3
		case c&0xF8 == 0xF0:
			size = 4
		default:
			return false
		}
		if i+size > len(b) {
			return false
		}
		for j := 1; j < size; j++ {
			if b[i+j]&0xC0 != 0x80 {
				return false
			}
		}
		i += size
	}
	return true
}

