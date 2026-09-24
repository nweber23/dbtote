package compress

import (
	"compress/gzip"
	"fmt"
	"io"
)

type nopWriterCloser struct { io.Writer }

func (n nopWriterCloser) Close() error { return nil }

func NewWriter(alg string, w io.Writer) (io.WriteCloser, error) {
	switch alg {
		case "", "none":
			return nopWriterCloser{w}, nil
		case "gzip":
			return gzip.NewWriter(w), nil
		default:
			return nil, fmt.Errorf("compress: unsupported algorithm %q", alg)
	}
}

func NewReader(alg string, r io.Reader) (io.ReadCloser, error) {
	switch alg {
	case "gzip":
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("compress: failed to create gzip reader: %w", err)
		}
		return gr, nil
	default:
		return nil, fmt.Errorf("compress: unsupported algorithm %q", alg)
	}
}

func Ext(alg string) string {
	if alg == "gzip" {
		return ".gz"
	}
	return ""
}