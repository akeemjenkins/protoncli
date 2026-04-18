package mbox

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// JSONLRow is a single row in the JSONL export format. RFC822 is stored
// as a base64-encoded string so that control bytes in the body are safe
// inside a one-line-per-record envelope.
type JSONLRow struct {
	UID          uint32    `json:"uid"`
	InternalDate time.Time `json:"internal_date"`
	Flags        []string  `json:"flags"`
	Size         int       `json:"size"`
	RFC822       string    `json:"rfc822"`
}

// JSONLWriter writes rows as newline-delimited JSON.
type JSONLWriter struct {
	w   io.Writer
	enc *json.Encoder
}

// NewJSONLWriter returns a JSONLWriter that serializes to w. The encoder
// is configured not to escape HTML so RFC822 base64 content is preserved
// verbatim.
func NewJSONLWriter(w io.Writer) *JSONLWriter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &JSONLWriter{w: w, enc: enc}
}

// Write serializes a row. The RFC822 field is auto-populated from
// rfc822 via base64; Size is set from len(rfc822) if not already set.
func (w *JSONLWriter) Write(uid uint32, flags []string, internalDate time.Time, rfc822 []byte) error {
	row := JSONLRow{
		UID:          uid,
		InternalDate: internalDate.UTC(),
		Flags:        append([]string{}, flags...),
		Size:         len(rfc822),
		RFC822:       base64.StdEncoding.EncodeToString(rfc822),
	}
	if row.Flags == nil {
		row.Flags = []string{}
	}
	return w.enc.Encode(&row)
}

// Close is a no-op for API symmetry.
func (w *JSONLWriter) Close() error { return nil }

// JSONLReader decodes rows written by JSONLWriter. Malformed lines are
// surfaced as errors from Next.
type JSONLReader struct {
	sc *bufio.Scanner
}

// NewJSONLReader returns a reader over r. It lifts the default scanner
// buffer to accommodate large base64-encoded bodies.
func NewJSONLReader(r io.Reader) *JSONLReader {
	sc := bufio.NewScanner(r)
	// 16 MiB max line — each row is one RFC822 message base64-encoded.
	sc.Buffer(make([]byte, 0, 1<<16), 16<<20)
	return &JSONLReader{sc: sc}
}

// Next returns the next row as a struct plus the decoded RFC822 body.
// Returns io.EOF when the stream is exhausted.
func (r *JSONLReader) Next() (row JSONLRow, rfc822 []byte, err error) {
	for r.sc.Scan() {
		line := r.sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var decoded JSONLRow
		if uerr := json.Unmarshal(line, &decoded); uerr != nil {
			return JSONLRow{}, nil, fmt.Errorf("jsonl: decode row: %w", uerr)
		}
		body, berr := base64.StdEncoding.DecodeString(decoded.RFC822)
		if berr != nil {
			return JSONLRow{}, nil, fmt.Errorf("jsonl: decode rfc822 base64 (uid=%d): %w", decoded.UID, berr)
		}
		return decoded, body, nil
	}
	if serr := r.sc.Err(); serr != nil {
		return JSONLRow{}, nil, serr
	}
	return JSONLRow{}, nil, io.EOF
}
