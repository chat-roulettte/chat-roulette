package iox

import (
	"bytes"
	"io"
)

// ReadAndReset reads from an io.ReadCloser
// and resets it so it can be read again.
func ReadAndReset(body *io.ReadCloser) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	_, err := io.Copy(buf, *body)
	(*body).Close()

	if err != nil {
		return nil, err
	}

	b := buf.Bytes()

	*body = io.NopCloser(bytes.NewReader(b))

	return b, nil
}
