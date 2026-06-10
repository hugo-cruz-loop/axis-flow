package storage

import (
	"fmt"
	"io"

	"axis-flow-back/internal/bolsatrabajo"
)

// MaxCVSize is the maximum allowed CV file size (5 MB).
const MaxCVSize = 5_242_880 // 5 MB

// ValidateMagicBytes reads the first 8 bytes of r to detect whether the file
// is a PDF (%PDF header) or DOCX (PK zip 0x50 0x4B 0x03 0x04 header).
// It returns bolsatrabajo.ErrInvalidFile when:
//   - size exceeds 5 MB (5_242_880 bytes), or
//   - the magic bytes do not match PDF or DOCX signatures.
func ValidateMagicBytes(r io.Reader, size int64) error {
	if size > MaxCVSize {
		return fmt.Errorf("%w: file size %d exceeds 5MB limit", bolsatrabajo.ErrInvalidFile, size)
	}

	buf := make([]byte, 8)
	n, err := io.ReadFull(r, buf)
	if err != nil && n < 4 {
		return fmt.Errorf("%w: could not read magic bytes", bolsatrabajo.ErrInvalidFile)
	}

	if isPDF(buf) || isDOCX(buf) {
		return nil
	}

	return fmt.Errorf("%w: unrecognised file type (not PDF or DOCX)", bolsatrabajo.ErrInvalidFile)
}

// isPDF checks for the %PDF magic header (25 50 44 46).
func isPDF(b []byte) bool {
	return len(b) >= 4 &&
		b[0] == 0x25 && b[1] == 0x50 && b[2] == 0x44 && b[3] == 0x46
}

// isDOCX checks for the PK zip magic header used by DOCX (50 4B 03 04).
func isDOCX(b []byte) bool {
	return len(b) >= 4 &&
		b[0] == 0x50 && b[1] == 0x4B && b[2] == 0x03 && b[3] == 0x04
}
