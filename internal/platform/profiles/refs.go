package profiles

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// locate returns the resolved path of the regular file name under dir, after
// every link is followed, refusing a name or a link that leaves dir.
func locate(dir, name string, invalid, unreadable RefCode) (string, error) {
	if !ValidReferenceName(name) {
		return "", &RefError{Code: invalid}
	}
	base, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", &RefError{Code: unreadable}
	}
	full, err := filepath.EvalSymlinks(filepath.Join(base, name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &RefError{Code: invalid}
		}
		return "", &RefError{Code: unreadable}
	}
	if full == base || !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", &RefError{Code: invalid}
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", &RefError{Code: unreadable}
	}
	if !info.Mode().IsRegular() {
		return "", &RefError{Code: invalid}
	}
	return full, nil
}

func readReference(path string, unreadable, tooLarge RefCode) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, &RefError{Code: unreadable}
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
	path, err := locate(dir, name, CodeCredentialInvalid, CodeCredentialUnreadable)
	if err != nil {
		return "", err
	}
	data, err := readReference(path, CodeCredentialUnreadable, CodeCredentialTooLarge)
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
	path, err := locate(dir, name, CodeCABundleInvalid, CodeCABundleUnreadable)
	if err != nil {
		return nil, err
	}
	data, err := readReference(path, CodeCABundleUnreadable, CodeCABundleTooLarge)
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
