package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/settings"
	"github.com/marceloalvescf/capacity-room/internal/redact"
)

func TestReadCredentialsAddsAValidPasswordToTheRedactor(t *testing.T) {
	password := "Correct-Horse-9-Battery"
	redactor := redact.New()
	source := strings.NewReader(password + "\n")
	if _, _, err := readCredentials(source, redactor, "root"); err != nil {
		t.Fatal(err)
	}
	if got := redactor.Text("the password is " + password); strings.Contains(got, password) {
		t.Fatalf("Text() = %q, want the password removed", got)
	}
}

func TestReadCredentialsLeavesARefusalWhole(t *testing.T) {
	redactor := redact.New()
	source := strings.NewReader("short\n")
	_, _, err := readCredentials(source, redactor, "root")
	if err == nil {
		t.Fatal("want a refusal")
	}
	const message = "the password is too short"
	if got := redactor.Text(message); got != message {
		t.Fatalf("Text() = %q, want %q", got, message)
	}
}

func TestReadServerSecretAddsTheSecretToTheRedactor(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef-secret"
	path := filepath.Join(t.TempDir(), "server-secret")
	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	redactor := redact.New()
	got, err := readServerSecret(settings.Settings{ServerSecretPath: path}, redactor)
	if err != nil || string(got) != secret {
		t.Fatalf("readServerSecret() = %q, %v", got, err)
	}
	if text := redactor.Text("leaked " + secret); strings.Contains(text, secret) {
		t.Fatalf("Text() = %q, want the secret removed", text)
	}
}
