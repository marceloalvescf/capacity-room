package profiles

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func write(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func wantRef(t *testing.T, err error, code RefCode, dir string) {
	t.Helper()
	re, ok := err.(*RefError)
	if !ok || re.Code != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
	if err.Error() != string(code) || (dir != "" && strings.Contains(err.Error(), dir)) {
		t.Fatalf("text %q is not only the code", err.Error())
	}
}

func TestResolveReferenceConfinement(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	write(t, dir, "good-token", []byte("token-one\n"))
	write(t, other, "outside", []byte("token-two\n"))
	if err := os.Symlink("good-token", filepath.Join(dir, "inside")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(other, "outside"), filepath.Join(dir, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(dir, "self")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o700); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"good-token", "inside"} {
		got, err := ResolveCredential(dir, name)
		if err != nil || got != "token-one" {
			t.Errorf("%s: got %q, %v", name, got, err)
		}
	}
	for _, name := range []string{"escape", "subdir", "self", "missing"} {
		_, err := ResolveCredential(dir, name)
		wantRef(t, err, CodeCredentialInvalid, dir)
	}
	_, err := ResolveCredential(filepath.Join(dir, "nope"), "good-token")
	wantRef(t, err, CodeCredentialUnreadable, dir)
	_, err = ResolveCABundle(filepath.Join(dir, "nope"), "good-token")
	wantRef(t, err, CodeCABundleUnreadable, dir)
	_, err = ResolveCABundle(dir, "escape")
	wantRef(t, err, CodeCABundleInvalid, dir)
}

func TestResolveCredentialText(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		data []byte
		want string
		code RefCode
	}{
		{[]byte("token-one\r\n"), "token-one", ""},
		{[]byte("token-one\n"), "token-one", ""},
		{[]byte("abcdefghij\n\n"), "", CodeCredentialInvalid},
		{[]byte("abcdefghij\r"), "", CodeCredentialInvalid},
		{[]byte("abc\ndefghijk\n"), "", CodeCredentialInvalid},
		{[]byte("\xef\xbb\xbfabcdefghij\n"), "", CodeCredentialInvalid},
		{[]byte("abcdefghij \n"), "", CodeCredentialInvalid},
		{[]byte("abcde fghij\n"), "", CodeCredentialInvalid},
		{[]byte("abcdefghij\t\n"), "", CodeCredentialInvalid},
		{[]byte("abcde\u00a0fghij\n"), "", CodeCredentialInvalid},
		{[]byte("token-\xff\xfe-one\n"), "", CodeCredentialInvalid},
		{[]byte("short\n"), "", CodeCredentialInvalid},
		{[]byte("\n"), "", CodeCredentialInvalid},
		{[]byte(""), "", CodeCredentialInvalid},
		{bytes.Repeat([]byte("a"), MaxReferenceBytes+1), "", CodeCredentialTooLarge},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			write(t, dir, "tok", c.data)
			got, err := ResolveCredential(dir, "tok")
			if c.code == "" {
				if err != nil || got != c.want {
					t.Errorf("got %q, %v", got, err)
				}
				return
			}
			wantRef(t, err, c.code, dir)
		})
	}
}

func certPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test root"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestResolveCABundle(t *testing.T) {
	dir := t.TempDir()
	bundle := certPEM(t)
	write(t, dir, "ca", bundle)
	got, err := ResolveCABundle(dir, "ca")
	if err != nil || !bytes.Equal(got, bundle) {
		t.Fatalf("got %q, %v, want the bytes of the file", got, err)
	}

	cases := []struct {
		data []byte
		code RefCode
	}{
		{[]byte("not a pem at all"), CodeCABundleInvalid},
		{pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("0123456789abcdef")}), CodeCABundleInvalid},
		{pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")}), CodeCABundleInvalid},
		{bytes.Repeat([]byte("a"), MaxReferenceBytes+1), CodeCABundleTooLarge},
	}
	for i, c := range cases {
		write(t, dir, "bad", c.data)
		_, err := ResolveCABundle(dir, "bad")
		if re, ok := err.(*RefError); !ok || re.Code != c.code {
			t.Errorf("case %d: err = %v, want %s", i, err, c.code)
		}
	}
}

func TestRefErrorCarriesOnlyTheCode(t *testing.T) {
	dir := t.TempDir()
	for _, code := range []RefCode{CodeCredentialInvalid, CodeCredentialUnreadable, CodeCredentialTooLarge, CodeCABundleInvalid, CodeCABundleUnreadable, CodeCABundleTooLarge} {
		if got := (&RefError{Code: code}).Error(); got != string(code) {
			t.Errorf("%s: text %q", code, got)
		}
	}
	_, err := ResolveCredential(dir, "../etc")
	wantRef(t, err, CodeCredentialInvalid, dir)
	_, err = ResolveCABundle(dir, "missing")
	wantRef(t, err, CodeCABundleInvalid, dir)
}

func TestResolveReferenceFIFOIsNotRegular(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ResolveCredential(dir, "pipe")
		done <- err
	}()
	select {
	case err := <-done:
		wantRef(t, err, CodeCredentialInvalid, dir)
	case <-time.After(time.Second):
		t.Fatal("the read of a FIFO did not return within 1s")
	}
}

func TestResolveReferenceFollowsRelativeLinkInsideFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "..2026_data"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "..2026_data"), "token", []byte("token-one\n"))
	if err := os.Symlink("..2026_data", filepath.Join(dir, "..data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..data/token", filepath.Join(dir, "token")); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveCredential(dir, "token")
	if err != nil || got != "token-one" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestResolveReferenceRefusesLinkOutsideFolder(t *testing.T) {
	dir, other := t.TempDir(), t.TempDir()
	write(t, other, "outside", []byte("token-two\n"))
	if err := os.Symlink(filepath.Join("..", filepath.Base(other), "outside"), filepath.Join(dir, "up")); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveCredential(dir, "up")
	wantRef(t, err, CodeCredentialInvalid, dir)
}
