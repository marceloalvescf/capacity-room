package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

func TestReadCredentialsAddsThePasswordToTheRedactor(t *testing.T) {
	// The first password is too short and the second is valid: both enter
	// the redactor, since it is added before any check.
	for _, password := range []string{"short", "Correct-Horse-9"} {
		redactor := redact.New()
		env := Env{Stdin: strings.NewReader(password + "\n")}
		_, _, _ = readCredentials(env, redactor, "root")
		if got := redactor.Text("the password is " + password); strings.Contains(got, password) {
			t.Fatalf("Text() = %q, want the password removed", got)
		}
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
