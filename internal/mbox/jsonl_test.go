package mbox

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestJSONLRoundTrip(t *testing.T) {
	ts := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	type input struct {
		uid   uint32
		flags []string
		date  time.Time
		body  []byte
	}
	ins := []input{
		{uid: 1, flags: []string{`\Seen`}, date: ts, body: []byte("Subject: a\r\n\r\nhello\r\n")},
		{uid: 2, flags: nil, date: ts.Add(time.Minute), body: []byte("")},
		{uid: 3, flags: []string{`\Flagged`, "custom"}, date: ts.Add(2 * time.Minute), body: bytes.Repeat([]byte("x"), 2048)},
	}

	var buf bytes.Buffer
	w := NewJSONLWriter(&buf)
	for _, in := range ins {
		if err := w.Write(in.uid, in.flags, in.date, in.body); err != nil {
			t.Fatalf("write uid=%d: %v", in.uid, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	r := NewJSONLReader(bytes.NewReader(buf.Bytes()))
	for i, in := range ins {
		row, body, err := r.Next()
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if row.UID != in.uid {
			t.Errorf("uid = %d, want %d", row.UID, in.uid)
		}
		if !row.InternalDate.Equal(in.date) {
			t.Errorf("date = %v, want %v", row.InternalDate, in.date)
		}
		if row.Size != len(in.body) {
			t.Errorf("size = %d, want %d", row.Size, len(in.body))
		}
		if !bytes.Equal(body, in.body) {
			t.Errorf("body mismatch at %d", i)
		}
		wantFlags := in.flags
		if wantFlags == nil {
			wantFlags = []string{}
		}
		if len(row.Flags) != len(wantFlags) {
			t.Errorf("flags len = %d, want %d", len(row.Flags), len(wantFlags))
		}
	}
	if _, _, err := r.Next(); err != io.EOF {
		t.Errorf("want EOF, got %v", err)
	}
}

func TestJSONLEmptyStream(t *testing.T) {
	r := NewJSONLReader(strings.NewReader(""))
	if _, _, err := r.Next(); err != io.EOF {
		t.Errorf("empty: want EOF, got %v", err)
	}
}

func TestJSONLSkipsBlankLines(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSONLWriter(&buf)
	_ = w.Write(1, []string{`\Seen`}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), []byte("body\n"))
	// Prepend a blank line to simulate tool output that padded the file.
	data := append([]byte("\n"), buf.Bytes()...)
	r := NewJSONLReader(bytes.NewReader(data))
	row, body, err := r.Next()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if row.UID != 1 || string(body) != "body\n" {
		t.Errorf("unexpected: %+v %q", row, body)
	}
}

func TestJSONLMalformedLine(t *testing.T) {
	r := NewJSONLReader(strings.NewReader("not-json\n"))
	if _, _, err := r.Next(); err == nil {
		t.Error("expected decode error")
	}
}

type jsonlFailWriter struct{}

func (jsonlFailWriter) Write(_ []byte) (int, error) { return 0, io.ErrClosedPipe }

func TestJSONLWriterPropagatesError(t *testing.T) {
	w := NewJSONLWriter(jsonlFailWriter{})
	if err := w.Write(1, nil, time.Time{}, []byte("x")); err == nil {
		t.Error("expected write error")
	}
}

func TestJSONLInvalidBase64(t *testing.T) {
	// Valid JSON but invalid base64 in rfc822 field.
	line := `{"uid":5,"flags":["\\Seen"],"internal_date":"2026-01-01T00:00:00Z","size":0,"rfc822":"@@@not-base64@@@"}` + "\n"
	r := NewJSONLReader(strings.NewReader(line))
	if _, _, err := r.Next(); err == nil {
		t.Error("expected base64 error")
	}
}
