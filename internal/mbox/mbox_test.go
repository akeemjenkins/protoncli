package mbox

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

type sample struct {
	body   []byte
	sender string
	ts     time.Time
}

func mustReadAll(t *testing.T, buf *bytes.Buffer) []sample {
	t.Helper()
	r := NewReader(buf)
	var out []sample
	for {
		body, sender, ts, err := r.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		out = append(out, sample{body: body, sender: sender, ts: ts})
	}
}

func TestWriterReaderRoundTrip(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	inputs := []sample{
		{body: []byte("Subject: plain\r\n\r\nhello world\r\n"), sender: "a@x.com", ts: ts},
		{body: []byte("Subject: tricky\r\n\r\nFrom the mountains\r\nFrom nowhere\r\n>From escaped\r\n"), sender: "b@x.com", ts: ts.Add(time.Hour)},
		{body: []byte(""), sender: "c@x.com", ts: ts.Add(2 * time.Hour)},
		{body: []byte("Subject: lf-only\n\nno crlf here\n"), sender: "d@x.com", ts: ts.Add(3 * time.Hour)},
		{body: bytes.Repeat([]byte("line of text\r\n"), 500), sender: "e@x.com", ts: ts.Add(4 * time.Hour)},
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	for _, s := range inputs {
		if err := w.Write(s.body, s.sender, s.ts); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got := mustReadAll(t, bytes.NewBuffer(buf.Bytes()))
	if len(got) != len(inputs) {
		t.Fatalf("count: got %d, want %d", len(got), len(inputs))
	}
	for i, in := range inputs {
		// Normalize expected body the same way the Writer does (CRLF->LF; trailing LF ensured).
		exp := bytes.ReplaceAll(in.body, []byte("\r\n"), []byte("\n"))
		if len(exp) > 0 && exp[len(exp)-1] != '\n' {
			exp = append(exp, '\n')
		}
		if len(exp) == 0 {
			exp = []byte("\n")
		}
		if !bytes.Equal(got[i].body, exp) {
			t.Errorf("msg %d body mismatch\n got: %q\nwant: %q", i, got[i].body, exp)
		}
		if got[i].sender != in.sender {
			t.Errorf("msg %d sender = %q, want %q", i, got[i].sender, in.sender)
		}
		if !got[i].ts.Equal(in.ts) {
			t.Errorf("msg %d ts = %v, want %v", i, got[i].ts, in.ts)
		}
	}
}

func TestWriterFromEscaping(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	body := []byte("From this line\n>From already quoted\n>>From double\nok\n")
	if err := w.Write(body, "s@x.com", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Every body line that looked like ^>*From  should be prefixed by an additional '>'.
	if !strings.Contains(out, "\n>From this line\n") {
		t.Errorf("missing escape for leading From: %q", out)
	}
	if !strings.Contains(out, "\n>>From already quoted\n") {
		t.Errorf("missing escape for >From: %q", out)
	}
	if !strings.Contains(out, "\n>>>From double\n") {
		t.Errorf("missing escape for >>From: %q", out)
	}

	// Round-trip recovers the original body exactly.
	body2, _, _, err := NewReader(bytes.NewReader(buf.Bytes())).Next()
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(body2) != string(body) {
		t.Errorf("round-trip mismatch\n got: %q\nwant: %q", body2, body)
	}
}

func TestWriterEmptyBody(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(nil, "nobody@x.com", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	// Stream is "From ...\n\n".
	if !strings.HasPrefix(buf.String(), "From nobody@x.com ") {
		t.Errorf("bad prefix: %q", buf.String())
	}
	body, sender, _, err := NewReader(bytes.NewReader(buf.Bytes())).Next()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if sender != "nobody@x.com" {
		t.Errorf("sender = %q", sender)
	}
	if len(body) != 1 || body[0] != '\n' {
		t.Errorf("body = %q, want single newline", body)
	}
}

func TestReaderEOFOnEmpty(t *testing.T) {
	r := NewReader(bytes.NewReader(nil))
	if _, _, _, err := r.Next(); err != io.EOF {
		t.Errorf("want EOF, got %v", err)
	}
}

func TestReaderRejectsGarbage(t *testing.T) {
	r := NewReader(strings.NewReader("not a from line\nanother line\n"))
	if _, _, _, err := r.Next(); err == nil {
		t.Error("expected framing error")
	}
}

func TestSenderSanitization(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	// Spaces in sender would otherwise break the "From " separator.
	if err := w.Write([]byte("body\n"), "first last@x.com", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	body, sender, _, err := NewReader(bytes.NewReader(buf.Bytes())).Next()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if sender != "first_last@x.com" {
		t.Errorf("sender = %q, want sanitized", sender)
	}
	if string(body) != "body\n" {
		t.Errorf("body = %q", body)
	}

	// Blank sender falls back to placeholder.
	var buf2 bytes.Buffer
	w2 := NewWriter(&buf2)
	_ = w2.Write([]byte("x\n"), "", time.Time{})
	_, s2, _, err := NewReader(bytes.NewReader(buf2.Bytes())).Next()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if s2 == "" {
		t.Errorf("blank sender should be replaced")
	}
}

func TestRoundTripBodyHashParity(t *testing.T) {
	// Build a large, tricky corpus and verify bit-exact body parity through
	// the writer+reader pipeline after CRLF normalization.
	bodies := [][]byte{
		[]byte("Subject: small\n\nhi\n"),
		append([]byte("Subject: big\n\n"), bytes.Repeat([]byte("payload\n"), 4096)...),
		[]byte("Subject: from-at-start\n\nFrom me\nFrom you\nend\n"),
	}
	var buf bytes.Buffer
	w := NewWriter(&buf)
	for i, b := range bodies {
		if err := w.Write(b, fmt.Sprintf("u%d@x.com", i), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	got := mustReadAll(t, bytes.NewBuffer(buf.Bytes()))
	if len(got) != len(bodies) {
		t.Fatalf("count = %d", len(got))
	}
	for i, b := range bodies {
		if sha256.Sum256(got[i].body) != sha256.Sum256(b) {
			t.Errorf("hash mismatch at %d:\n got=%q\nwant=%q", i, got[i].body, b)
		}
	}
}

// failWriter returns err after n successful bytes.
type failWriter struct {
	budget int
	err    error
}

func (w *failWriter) Write(p []byte) (int, error) {
	if w.budget <= 0 {
		return 0, w.err
	}
	if len(p) <= w.budget {
		w.budget -= len(p)
		return len(p), nil
	}
	n := w.budget
	w.budget = 0
	return n, w.err
}

func TestWriterPropagatesErrors(t *testing.T) {
	// Fail on the separator write (second message).
	fw := &failWriter{budget: 1 << 20, err: io.ErrShortWrite}
	w := NewWriter(fw)
	_ = w.Write([]byte("body1\n"), "a@x.com", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	fw.budget = 0
	if err := w.Write([]byte("body2\n"), "b@x.com", time.Time{}); err == nil {
		t.Error("expected error on separator")
	}
	// Subsequent Write returns cached error.
	if err := w.Write([]byte("x"), "c", time.Time{}); err == nil {
		t.Error("expected sticky error")
	}

	// Fail on header write.
	fw2 := &failWriter{budget: 0, err: io.ErrShortWrite}
	w2 := NewWriter(fw2)
	if err := w2.Write([]byte("body\n"), "a@x.com", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Error("expected header write error")
	}
}

func TestReaderTruncForErr(t *testing.T) {
	long := strings.Repeat("x", 200)
	r := NewReader(strings.NewReader(long + "\n"))
	err := errNext(r)
	if err == nil || !strings.Contains(err.Error(), "...") {
		t.Errorf("expected truncated error, got %v", err)
	}
}

func errNext(r *Reader) error {
	_, _, _, err := r.Next()
	return err
}

func TestReaderEOFAfterExhaustion(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	_ = w.Write([]byte("only\n"), "a@x.com", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	r := NewReader(bytes.NewReader(buf.Bytes()))
	if _, _, _, err := r.Next(); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, _, _, err := r.Next(); err != io.EOF {
		t.Errorf("second: want EOF, got %v", err)
	}
	// Third read also EOF (done flag).
	if _, _, _, err := r.Next(); err != io.EOF {
		t.Errorf("third: want EOF, got %v", err)
	}
}

func TestReaderUnterminatedLastLine(t *testing.T) {
	// "From ..." header is fine; body without trailing newline.
	data := "From a@x.com Mon Jan  1 00:00:00 2026\n" + "body-no-nl"
	body, _, _, err := NewReader(strings.NewReader(data)).Next()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "body-no-nl\n" {
		t.Errorf("body = %q", body)
	}
}

func TestParseFromLineFallbacks(t *testing.T) {
	sender, ts := parseFromLine([]byte("From me@x.com Mon Jan  2 15:04:05 2026\n"))
	if sender != "me@x.com" {
		t.Errorf("sender = %q", sender)
	}
	if ts.Year() != 2026 {
		t.Errorf("ts year = %d", ts.Year())
	}
	// Unknown timestamp format → zero time, sender still parses.
	sender, ts = parseFromLine([]byte("From me@x.com xyz\n"))
	if sender != "me@x.com" || !ts.IsZero() {
		t.Errorf("fallback parse: %q %v", sender, ts)
	}
	// No timestamp at all.
	sender, ts = parseFromLine([]byte("From lone\n"))
	if sender != "lone" || !ts.IsZero() {
		t.Errorf("lone: %q %v", sender, ts)
	}
}
