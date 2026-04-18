package termio

import (
	"bytes"
	"strings"
	"testing"
)

func setTestDefault(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	prev := defaultWriter
	t.Cleanup(func() { defaultWriter = prev })
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	SetDefault(&Writer{Out: out, Err: err, Color: false})
	return out, err
}

func TestDefault_LazyInit(t *testing.T) {
	prev := defaultWriter
	t.Cleanup(func() { defaultWriter = prev })
	defaultWriter = nil
	w := Default()
	if w == nil {
		t.Fatal("Default() returned nil")
	}
	if w.Out == nil || w.Err == nil {
		t.Error("Default() streams nil")
	}
}

func TestInfo_PackageLevel(t *testing.T) {
	_, errB := setTestDefault(t)
	Info("n=%d val=%s", 3, "hi\x1bx")
	if errB.String() != "n=3 val=hix\n" {
		t.Errorf("Info = %q", errB.String())
	}
}

func TestWarn_PackageLevel(t *testing.T) {
	_, errB := setTestDefault(t)
	Warn("be careful %s", "now\u202E")
	if !strings.HasPrefix(errB.String(), "warning: be careful now") {
		t.Errorf("Warn = %q", errB.String())
	}
}

func TestError_PackageLevel(t *testing.T) {
	_, errB := setTestDefault(t)
	Error("oops %d", 42)
	if errB.String() != "error: oops 42\n" {
		t.Errorf("Error = %q", errB.String())
	}
}

func TestSetDefault(t *testing.T) {
	prev := defaultWriter
	t.Cleanup(func() { defaultWriter = prev })
	custom := &Writer{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	SetDefault(custom)
	if Default() != custom {
		t.Error("SetDefault did not replace default writer")
	}
}
