package profiles

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
)

// RefCode is the code of a refused reference resolution.
type RefCode string

// The codes of a reference resolution.
const (
	CodeCredentialInvalid    RefCode = "CREDENTIAL_FILE_INVALID"
	CodeCredentialUnreadable RefCode = "CREDENTIAL_FILE_UNREADABLE"
	CodeCredentialTooLarge   RefCode = "CREDENTIAL_FILE_TOO_LARGE"
	CodeCABundleInvalid      RefCode = "CA_BUNDLE_INVALID"
	CodeCABundleUnreadable   RefCode = "CA_BUNDLE_UNREADABLE"
	CodeCABundleTooLarge     RefCode = "CA_BUNDLE_TOO_LARGE"
)

// RefError carries a code and nothing else.
type RefError struct{ Code RefCode }

func (e *RefError) Error() string { return string(e.Code) }

// MaxReferenceBytes bounds a referenced file.
const MaxReferenceBytes = 1 << 20

// openReference opens the file name inside dir once, through a root that
// refuses a name or a link leaving dir, and checks on the open file that it
// is a regular file. The open does not block on a FIFO.
func openReference(dir, name string, invalid, unreadable RefCode) (*os.File, error) {
	if !ValidReferenceName(name) {
		return nil, &RefError{Code: invalid}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, &RefError{Code: unreadable}
	}
	defer func() { _ = root.Close() }()
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, &RefError{Code: openErrorCode(err, invalid, unreadable)}
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, &RefError{Code: unreadable}
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, &RefError{Code: invalid}
	}
	return f, nil
}

// readReference reads the regular file name inside dir, at most
// MaxReferenceBytes.
func readReference(dir, name string, invalid, unreadable, tooLarge RefCode) ([]byte, error) {
	f, err := openReference(dir, name, invalid, unreadable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxReferenceBytes+1))
	if err != nil {
		return nil, &RefError{Code: unreadable}
	}
	if len(data) > MaxReferenceBytes {
		return nil, &RefError{Code: tooLarge}
	}
	return data, nil
}

// ResolveCredential reads the credential named under dir and returns the
// token: UTF-8, one trailing CRLF or LF removed, at least 8 characters, with
// no control character, no white space, and no leading byte order mark.
func ResolveCredential(dir, name string) (string, error) {
	data, err := readReference(dir, name, CodeCredentialInvalid, CodeCredentialUnreadable, CodeCredentialTooLarge)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", &RefError{Code: CodeCredentialInvalid}
	}
	token := string(data)
	if t, ok := strings.CutSuffix(token, "\r\n"); ok {
		token = t
	} else {
		token = strings.TrimSuffix(token, "\n")
	}
	if token == "" || !cleanToken(token) || config.CheckToken(token) != nil {
		return "", &RefError{Code: CodeCredentialInvalid}
	}
	return token, nil
}

// cleanToken reports whether token holds no control character and no white
// space, and does not start with a byte order mark.
func cleanToken(token string) bool {
	if strings.HasPrefix(token, "\uFEFF") {
		return false
	}
	return !strings.ContainsFunc(token, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	})
}

// ResolveCABundle checks the CA bundle named under dir holds at least one
// PEM certificate and returns the bytes it checked, for the TLS
// configuration.
func ResolveCABundle(dir, name string) ([]byte, error) {
	data, err := readReference(dir, name, CodeCABundleInvalid, CodeCABundleUnreadable, CodeCABundleTooLarge)
	if err != nil {
		return nil, err
	}
	for rest := data; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, &RefError{Code: CodeCABundleInvalid}
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		if _, err := x509.ParseCertificate(block.Bytes); err == nil {
			return data, nil
		}
	}
}

// openErrorCode maps the error of opening a reference file: a refused
// permission and a transient resource or I/O failure are unreadable, every
// other error is invalid.
func openErrorCode(err error, invalid, unreadable RefCode) RefCode {
	for _, transient := range []error{fs.ErrPermission, syscall.EMFILE, syscall.ENFILE, syscall.EIO, syscall.ENOMEM} {
		if errors.Is(err, transient) {
			return unreadable
		}
	}
	return invalid
}
