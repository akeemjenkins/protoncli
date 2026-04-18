// Package imapwrite provides typed operations that mutate messages:
// flag changes, moves, copies, archive/trash, and read/unread toggling.
package imapwrite

import (
	"fmt"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

// DefaultArchiveMailbox is the mailbox Archive uses if the caller does not
// override it.
const DefaultArchiveMailbox = "Archive"

// DefaultTrashMailbox is the mailbox Trash uses if the caller does not
// override it.
const DefaultTrashMailbox = "Trash"

// MarkRead sets or clears the \Seen flag on the given UIDs.
func MarkRead(c *client.Client, mailbox string, uids []uint32, read bool) error {
	return SetFlag(c, mailbox, uids, imap.SeenFlag, read)
}

// SetFlag adds or removes a flag (system or custom keyword) on the UID set.
func SetFlag(c *client.Client, mailbox string, uids []uint32, flag string, add bool) error {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return fmt.Errorf("empty flag")
	}
	if len(uids) == 0 {
		return nil
	}
	if _, err := c.Select(mailbox, false); err != nil {
		return fmt.Errorf("SELECT %q: %w", mailbox, err)
	}
	op := imap.FormatFlagsOp(imap.AddFlags, true)
	if !add {
		op = imap.FormatFlagsOp(imap.RemoveFlags, true)
	}
	seq := &imap.SeqSet{}
	seq.AddNum(uids...)
	if err := c.UidStore(seq, op, []interface{}{flag}, nil); err != nil {
		return fmt.Errorf("UID STORE %s %q on %q: %w", storeOpString(add), flag, mailbox, err)
	}
	return nil
}

// Copy UID-copies the given UIDs from srcMailbox to dstMailbox.
func Copy(c *client.Client, srcMailbox, dstMailbox string, uids []uint32) error {
	if len(uids) == 0 {
		return nil
	}
	if _, err := c.Select(srcMailbox, false); err != nil {
		return fmt.Errorf("SELECT %q: %w", srcMailbox, err)
	}
	seq := &imap.SeqSet{}
	seq.AddNum(uids...)
	if err := c.UidCopy(seq, dstMailbox); err != nil {
		return fmt.Errorf("UID COPY %q→%q: %w", srcMailbox, dstMailbox, err)
	}
	return nil
}

// Move moves messages from srcMailbox to dstMailbox. Uses IMAP MOVE when
// the server advertises it; falls back to COPY + STORE \Deleted + EXPUNGE.
func Move(c *client.Client, srcMailbox, dstMailbox string, uids []uint32) error {
	if len(uids) == 0 {
		return nil
	}
	if _, err := c.Select(srcMailbox, false); err != nil {
		return fmt.Errorf("SELECT %q: %w", srcMailbox, err)
	}
	seq := &imap.SeqSet{}
	seq.AddNum(uids...)

	if hasMove, _ := c.Support("MOVE"); hasMove {
		if err := c.UidMove(seq, dstMailbox); err == nil {
			return nil
		}
		// Fallthrough: server advertised MOVE but rejected the call (some
		// backends advertise capabilities they don't fully implement).
	}

	if err := c.UidCopy(seq, dstMailbox); err != nil {
		return fmt.Errorf("UID COPY %q→%q: %w", srcMailbox, dstMailbox, err)
	}
	if err := c.UidStore(seq, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return fmt.Errorf("UID STORE \\Deleted on %q: %w", srcMailbox, err)
	}
	if err := c.Expunge(nil); err != nil {
		return fmt.Errorf("EXPUNGE %q: %w", srcMailbox, err)
	}
	return nil
}

// Archive is a thin wrapper over Move with a default target mailbox.
func Archive(c *client.Client, srcMailbox string, uids []uint32, target string) error {
	if target == "" {
		target = DefaultArchiveMailbox
	}
	return Move(c, srcMailbox, target, uids)
}

// Trash is a thin wrapper over Move with a default target mailbox.
func Trash(c *client.Client, srcMailbox string, uids []uint32, target string) error {
	if target == "" {
		target = DefaultTrashMailbox
	}
	return Move(c, srcMailbox, target, uids)
}

func storeOpString(add bool) string {
	if add {
		return "+"
	}
	return "-"
}
