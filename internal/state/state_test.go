package state

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTest(t *testing.T) (*DB, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func sampleMsg(mb string, uid uint32) *ProcessedMessage {
	return &ProcessedMessage{
		Mailbox:         mb,
		UIDValidity:     100,
		UID:             uid,
		Subject:         "s",
		From:            "f@example.com",
		Date:            time.Now().UTC().Truncate(time.Second),
		SuggestedLabels: []string{"Orders"},
		Confidence:      0.8,
		Rationale:       "r",
		IsMailingList:   false,
		LabelsApplied:   false,
		ProcessedAt:     time.Now().UTC().Truncate(time.Second),
	}
}

func TestOpen_NewAndReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Reopen should succeed without error (migrations are idempotent).
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_ = db2.Close()
}

func TestOpen_DefaultPath(t *testing.T) {
	// Use temp dir as HOME so default-path branch runs without touching
	// the real filesystem.
	t.Setenv("HOME", t.TempDir())
	db, err := Open("")
	if err != nil {
		t.Fatalf("Open(\"\"): %v", err)
	}
	_ = db.Close()
}

func TestMarkProcessed_InsertAndUpdate(t *testing.T) {
	db, _ := openTest(t)
	m := sampleMsg("INBOX", 1)
	if err := db.MarkProcessed(m); err != nil {
		t.Fatalf("insert: %v", err)
	}
	ok, err := db.IsProcessed("INBOX", 100, 1)
	if err != nil {
		t.Fatalf("IsProcessed: %v", err)
	}
	if !ok {
		t.Fatal("expected processed=true after insert")
	}
	// Update (upsert) with new rationale/confidence.
	m.Rationale = "updated"
	m.Confidence = 0.99
	m.LabelsApplied = true
	if err := db.MarkProcessed(m); err != nil {
		t.Fatalf("update: %v", err)
	}
	total, applied, failed, err := db.GetStats("INBOX")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if total != 1 || applied != 1 || failed != 0 {
		t.Errorf("stats=%d/%d/%d, want 1/1/0", total, applied, failed)
	}
}

func TestGetProcessedUIDs(t *testing.T) {
	db, _ := openTest(t)
	for _, uid := range []uint32{1, 2, 3} {
		if err := db.MarkProcessed(sampleMsg("INBOX", uid)); err != nil {
			t.Fatalf("mark: %v", err)
		}
	}
	// Different mailbox
	if err := db.MarkProcessed(sampleMsg("Sent", 5)); err != nil {
		t.Fatal(err)
	}
	// Different uidvalidity
	m := sampleMsg("INBOX", 99)
	m.UIDValidity = 200
	if err := db.MarkProcessed(m); err != nil {
		t.Fatal(err)
	}

	got, err := db.GetProcessedUIDs("INBOX", 100)
	if err != nil {
		t.Fatalf("GetProcessedUIDs: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d UIDs, want 3: %v", len(got), got)
	}
	for _, uid := range []uint32{1, 2, 3} {
		if !got[uid] {
			t.Errorf("missing uid %d", uid)
		}
	}
}

func TestGetUnappliedMessages(t *testing.T) {
	db, _ := openTest(t)
	// 3 unapplied, 1 applied, 1 with apply_error.
	for _, uid := range []uint32{1, 2, 3} {
		_ = db.MarkProcessed(sampleMsg("INBOX", uid))
	}
	applied := sampleMsg("INBOX", 4)
	applied.LabelsApplied = true
	_ = db.MarkProcessed(applied)
	failed := sampleMsg("INBOX", 5)
	failed.ApplyError = "boom"
	_ = db.MarkProcessed(failed)

	rows, err := db.GetUnappliedMessages("INBOX", 100, 0)
	if err != nil {
		t.Fatalf("GetUnappliedMessages: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("got %d rows, want 3", len(rows))
	}

	// Limit honoured.
	limited, err := db.GetUnappliedMessages("INBOX", 100, 2)
	if err != nil {
		t.Fatalf("limit: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("limit got %d, want 2", len(limited))
	}

	count, err := db.CountUnapplied("INBOX", 100)
	if err != nil {
		t.Fatalf("CountUnapplied: %v", err)
	}
	if count != 3 {
		t.Errorf("CountUnapplied=%d, want 3", count)
	}
}

func TestMarkLabelsApplied(t *testing.T) {
	db, _ := openTest(t)
	_ = db.MarkProcessed(sampleMsg("INBOX", 1))

	if err := db.MarkLabelsApplied("INBOX", 100, 1, true, ""); err != nil {
		t.Fatalf("MarkLabelsApplied: %v", err)
	}
	count, err := db.CountUnapplied("INBOX", 100)
	if err != nil {
		t.Fatalf("CountUnapplied: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 unapplied after apply, got %d", count)
	}

	// Mark another as failed.
	_ = db.MarkProcessed(sampleMsg("INBOX", 2))
	if err := db.MarkLabelsApplied("INBOX", 100, 2, false, "oops"); err != nil {
		t.Fatal(err)
	}
	total, applied, failedCount, err := db.GetStats("INBOX")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if total != 2 || applied != 1 || failedCount != 1 {
		t.Errorf("stats=%d/%d/%d", total, applied, failedCount)
	}
}

func TestGetStats_AllMailboxes(t *testing.T) {
	db, _ := openTest(t)
	_ = db.MarkProcessed(sampleMsg("INBOX", 1))
	_ = db.MarkProcessed(sampleMsg("Sent", 2))

	total, applied, failed, err := db.GetStats("")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if total != 2 || applied != 0 || failed != 0 {
		t.Errorf("stats=%d/%d/%d, want 2/0/0", total, applied, failed)
	}

	// Percent wildcard
	total2, _, _, err := db.GetStats("%")
	if err != nil {
		t.Fatal(err)
	}
	if total2 != 2 {
		t.Errorf("wildcard total=%d, want 2", total2)
	}
}

func TestListMailboxes(t *testing.T) {
	db, _ := openTest(t)
	_ = db.MarkProcessed(sampleMsg("INBOX", 1))
	_ = db.MarkProcessed(sampleMsg("Sent", 2))
	_ = db.MarkProcessed(sampleMsg("Drafts", 3))

	got, err := db.ListMailboxes()
	if err != nil {
		t.Fatalf("ListMailboxes: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d, want 3: %v", len(got), got)
	}
	// Should be sorted.
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("not sorted: %v", got)
		}
	}
}

func TestClearMailbox(t *testing.T) {
	db, _ := openTest(t)
	_ = db.MarkProcessed(sampleMsg("INBOX", 1))
	old := sampleMsg("INBOX", 2)
	old.UIDValidity = 50 // older validity
	_ = db.MarkProcessed(old)

	// Clear rows whose uid_validity != 100 in mailbox INBOX.
	if err := db.ClearMailbox("INBOX", 100); err != nil {
		t.Fatalf("ClearMailbox: %v", err)
	}
	total, _, _, _ := db.GetStats("INBOX")
	if total != 1 {
		t.Errorf("after clear, total=%d, want 1", total)
	}
}

func TestConcurrentWrites(t *testing.T) {
	db, _ := openTest(t)
	const N = 20
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		go func(uid uint32) {
			defer wg.Done()
			if err := db.MarkProcessed(sampleMsg("INBOX", uid)); err != nil {
				errs <- err
			}
		}(uint32(i + 1))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent write error: %v", err)
	}
	total, _, _, err := db.GetStats("INBOX")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if total != N {
		t.Errorf("total=%d, want %d", total, N)
	}
}

func TestIsProcessed_NotFound(t *testing.T) {
	db, _ := openTest(t)
	ok, err := db.IsProcessed("INBOX", 100, 999)
	if err != nil {
		t.Fatalf("IsProcessed: %v", err)
	}
	if ok {
		t.Error("expected false for missing UID")
	}
}

func TestOperationsOnClosedDB(t *testing.T) {
	db, _ := openTest(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := db.IsProcessed("m", 1, 1); err == nil {
		t.Error("expected IsProcessed error on closed db")
	}
	if err := db.MarkProcessed(sampleMsg("m", 1)); err == nil {
		t.Error("expected MarkProcessed error on closed db")
	}
	if _, err := db.GetProcessedUIDs("m", 1); err == nil {
		t.Error("expected GetProcessedUIDs error")
	}
	if _, _, _, err := db.GetStats("m"); err == nil {
		t.Error("expected GetStats error")
	}
	if _, _, _, err := db.GetStats(""); err == nil {
		t.Error("expected GetStats(all) error")
	}
	if _, err := db.ListMailboxes(); err == nil {
		t.Error("expected ListMailboxes error")
	}
	if _, err := db.GetUnappliedMessages("m", 1, 0); err == nil {
		t.Error("expected GetUnappliedMessages error")
	}
}

// Open() with an invalid path returns an error. Use a path under a file, which
// cannot be a directory.
func TestOpen_BadPath(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	// Create blocker as a file so the path "blocker/state.db" is invalid (blocker is not a dir).
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	badPath := filepath.Join(blocker, "state.db")
	if _, err := Open(badPath); err == nil {
		t.Error("expected error for bad path")
	}
}
