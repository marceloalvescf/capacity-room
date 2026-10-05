package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	minSessionSeconds = 60
	maxSessionSeconds = 2592000
	minSecretBytes    = 32
	dsnVariable       = "KCR_DATABASE_DSN"
)

var (
	errTTLRange       = errors.New("session_ttl_seconds must be between 60 and 2592000")
	errIdleRange      = errors.New("session_idle_seconds must be between 60 and 2592000")
	errIdleOverTTL    = errors.New("session_idle_seconds must not exceed session_ttl_seconds")
	errSecretRequired = errors.New("server_secret_path is required")
	errSecretRelative = errors.New("server_secret_path must be absolute")
	errProxies        = errors.New("trusted_proxies must hold CIDR ranges")
	errDSNUnset       = errors.New(dsnVariable + " is not set")
	errDSNScheme      = errors.New(dsnVariable + " must be a postgres:// or postgresql:// URL")
	errSecretShort    = errors.New("the server secret must hold at least 32 bytes")
)

// Settings is the deployment file of spec 11.2, read once at start by serve
// and admin, plus the DSN of the environment.
type Settings struct {
	SessionTTL       time.Duration  // session_ttl_seconds
	SessionIdle      time.Duration  // session_idle_seconds
	ServerSecretPath string         // server_secret_path
	TrustedProxies   []netip.Prefix // trusted_proxies
	DatabaseDSN      string         // KCR_DATABASE_DSN; never written to a message
}

// tomlSettings mirrors the keys of the deployment file. A key the file does
// not name keeps the default it was decoded over.
type tomlSettings struct {
	SessionTTLSeconds  int64    `toml:"session_ttl_seconds"`
	SessionIdleSeconds int64    `toml:"session_idle_seconds"`
	ServerSecretPath   string   `toml:"server_secret_path"`
	TrustedProxies     []string `toml:"trusted_proxies"`
}

// Load reads the TOML file at path, then the environment through getenv,
// and validates the result. An unknown key or section is an error that
// names the key and not its value. The DSN is read from KCR_DATABASE_DSN
// and is required.
func Load(path string, getenv func(string) string) (Settings, error) {
	file := tomlSettings{SessionTTLSeconds: 43200, SessionIdleSeconds: 3600}
	md, err := toml.DecodeFile(path, &file)
	if err != nil {
		return Settings{}, decodeError(path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return Settings{}, fmt.Errorf("%s: unknown key %s", path, undecoded[0])
	}
	return build(file, getenv(dsnVariable))
}

// decodeError words a failure of toml.DecodeFile without repeating any value
// of the file.
func decodeError(path string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("cannot read %s: %s", path, pathErr.Err)
	}
	var parseErr toml.ParseError
	if errors.As(err, &parseErr) {
		return fmt.Errorf("%s is not valid TOML: %d:%d", path, parseErr.Position.Line, parseErr.Position.Col)
	}
	return fmt.Errorf("%s has a value of the wrong type", path)
}

// build validates the decoded file and the DSN, reporting the first failure,
// and returns the Settings.
func build(file tomlSettings, dsn string) (Settings, error) {
	if file.SessionTTLSeconds < minSessionSeconds || file.SessionTTLSeconds > maxSessionSeconds {
		return Settings{}, errTTLRange
	}
	if file.SessionIdleSeconds < minSessionSeconds || file.SessionIdleSeconds > maxSessionSeconds {
		return Settings{}, errIdleRange
	}
	if file.SessionIdleSeconds > file.SessionTTLSeconds {
		return Settings{}, errIdleOverTTL
	}
	if file.ServerSecretPath == "" {
		return Settings{}, errSecretRequired
	}
	if !filepath.IsAbs(file.ServerSecretPath) {
		return Settings{}, errSecretRelative
	}
	var proxies []netip.Prefix
	for _, entry := range file.TrustedProxies {
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			return Settings{}, errProxies
		}
		proxies = append(proxies, prefix)
	}
	if dsn == "" {
		return Settings{}, errDSNUnset
	}
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return Settings{}, errDSNScheme
	}
	return Settings{
		SessionTTL:       time.Duration(file.SessionTTLSeconds) * time.Second,
		SessionIdle:      time.Duration(file.SessionIdleSeconds) * time.Second,
		ServerSecretPath: file.ServerSecretPath,
		TrustedProxies:   proxies,
		DatabaseDSN:      dsn,
	}, nil
}

// ReadServerSecret reads the file named by ServerSecretPath whole and
// returns its bytes. A file shorter than 32 bytes is refused.
func (s Settings) ReadServerSecret() ([]byte, error) {
	data, err := os.ReadFile(s.ServerSecretPath)
	if err != nil {
		cause := err.Error()
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			cause = pathErr.Err.Error()
		}
		return nil, fmt.Errorf("cannot read the server secret at %s: %s", s.ServerSecretPath, cause)
	}
	if len(data) < minSecretBytes {
		return nil, errSecretShort
	}
	return data, nil
}
