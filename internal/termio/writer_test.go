package termio

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func newTestWriter(color bool) (*Writer, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	return &Writer{Out: out, Err: err, Color: color}, out, err
}

func TestNewDefault(t *testing.T) {
	w := NewDefault()
	if w.Out == nil || w.Err == nil {
		t.Fatal("NewDefault() returned writer with nil streams")
	}
}

func TestPrintJSON(t *testing.T) {
	w, out, _ := newTestWriter(false)
	if err := w.PrintJSON(map[string]int{"a": 1}); err != nil {
		t.Fatalf("PrintJSON: %v", err)
	}
	want := "{\n  \"a\": 1\n}\n"
	if out.String() != want {
		t.Errorf("PrintJSON out = %q, want %q", out.String(), want)
	}
}

func TestPrintJSON_Error(t *testing.T) {
	w, _, _ := newTestWriter(false)
	err := w.PrintJSON(make(chan int))
	if err == nil {
		t.Error("PrintJSON(chan) expected error, got nil")
	}
}

type failWriter struct{}

func (failWriter) Write(_ []byte) (int, error) { return 0, errors.New("boom") }

func TestPrintJSON_WriteError(t *testing.T) {
	w := &Writer{Out: failWriter{}, Err: &bytes.Buffer{}}
	if err := w.PrintJSON(1); err == nil {
		t.Error("expected write error")
	}
}

func TestPrintNDJSON(t *testing.T) {
	w, out, _ := newTestWriter(false)
	if err := w.PrintNDJSON(map[string]int{"a": 1}); err != nil {
		t.Fatalf("PrintNDJSON: %v", err)
	}
	want := "{\"a\":1}\n"
	if out.String() != want {
		t.Errorf("PrintNDJSON out = %q, want %q", out.String(), want)
	}
}

func TestPrintNDJSON_Error(t *testing.T) {
	w, _, _ := newTestWriter(false)
	if err := w.PrintNDJSON(make(chan int)); err == nil {
		t.Error("expected error")
	}
}

func TestPrintNDJSON_WriteError(t *testing.T) {
	w := &Writer{Out: failWriter{}, Err: &bytes.Buffer{}}
	if err := w.PrintNDJSON(1); err == nil {
		t.Error("expected write error")
	}
}

func TestStatus(t *testing.T) {
	w, out, errB := newTestWriter(false)
	w.Status("hello\x1bworld")
	if out.Len() != 0 {
		t.Errorf("Status wrote to Out: %q", out.String())
	}
	if errB.String() != "helloworld\n" {
		t.Errorf("Status err = %q, want %q", errB.String(), "helloworld\n")
	}
}

func TestWarn_NoColor(t *testing.T) {
	w, _, errB := newTestWriter(false)
	w.Warn("careful\u202Enow")
	if errB.String() != "warning: carefulnow\n" {
		t.Errorf("Warn = %q", errB.String())
	}
}

func TestWarn_Color(t *testing.T) {
	w, _, errB := newTestWriter(true)
	w.Warn("careful")
	s := errB.String()
	if !strings.Contains(s, "\x1b[1;33mwarning:\x1b[0m") {
		t.Errorf("Warn color = %q", s)
	}
	if !strings.Contains(s, "careful\n") {
		t.Errorf("Warn missing msg: %q", s)
	}
}

func TestErrorf_NoColor(t *testing.T) {
	w, _, errB := newTestWriter(false)
	w.Errorf("code %d: %s", 42, "bad\x07news")
	if errB.String() != "error: code 42: badnews\n" {
		t.Errorf("Errorf = %q", errB.String())
	}
}

func TestErrorf_Color(t *testing.T) {
	w, _, errB := newTestWriter(true)
	w.Errorf("boom")
	s := errB.String()
	if !strings.Contains(s, "\x1b[1;31merror:\x1b[0m") {
		t.Errorf("Errorf color = %q", s)
	}
}

func TestInfof(t *testing.T) {
	w, out, errB := newTestWriter(false)
	w.Infof("n=%d %s", 7, "ok\x1bx")
	if out.Len() != 0 {
		t.Errorf("Infof wrote to Out")
	}
	if errB.String() != "n=7 okx\n" {
		t.Errorf("Infof = %q", errB.String())
	}
}
