package compress

import (
	"bytes"
	"io"
	"testing"
)

func TestGzipRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter("gzip", &buf)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if _, err := w.Write([]byte("hello dbtote")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := NewReader("gzip", &buf)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "hello dbtote" {
		t.Errorf("got %q, want %q", got, "hello dbtote")
	}
}

func TestNoneRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter("none", &buf)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	w.Write([]byte("raw"))
	w.Close()
	if buf.String() != "raw" {
		t.Errorf("got %q, want %q", buf.String(), "raw")
	}
}

func TestExt(t *testing.T) {
	if Ext("gzip") != ".gz" {
		t.Errorf("Ext(gzip) = %q, want .gz", Ext("gzip"))
	}
	if Ext("none") != "" {
		t.Errorf("Ext(none) = %q, want empty", Ext("none"))
	}
}

func TestNewWriter_UnsupportedAlgorithm(t *testing.T) {
	if _, err := NewWriter("bogus", &bytes.Buffer{}); err == nil {
		t.Error("expected an error for an unsupported algorithm")
	}
}
