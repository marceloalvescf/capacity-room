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

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
)

const (
	minSessionSeconds = 60
	maxSessionSeconds = 2592000
	minSecretBytes    = 32
	dsnVariable       = "KCR_DATABASE_DSN"

	defaultCredentialDir = "/run/secrets/metrics"
	defaultCABundleDir   = "/run/config/ca"
)

// metadataAddresses are the cloud instance metadata endpoints that no
// allowlisted block may cover, except as a single address.
var metadataAddresses = []netip.Addr{
	netip.MustParseAddr("169.254.169.254"),
	netip.MustParseAddr("fd00:ec2::254"),
}

var (
	errTTLRange         = errors.New("session_ttl_seconds must be between 60 and 2592000")
	errIdleRange        = errors.New("session_idle_seconds must be between 60 and 2592000")
	errIdleOverTTL      = errors.New("session_idle_seconds must not exceed session_ttl_seconds")
	errProfileOverLimit = errors.New("admission_profile_limit must not exceed admission_limit")
	errSecretRequired   = errors.New("server_secret_path is required")
	errSecretRelative   = errors.New("server_secret_path must be absolute")
	errProxies          = errors.New("trusted_proxies must hold CIDR ranges")
	errDSNUnset         = errors.New(dsnVariable + " is not set")
	errDSNScheme        = errors.New(dsnVariable + " must be a postgres:// or postgresql:// URL")
	errSecretShort      = errors.New("the server secret must hold at least 32 bytes")
)

// Settings is the deployment file of spec 11.2, read once at start by serve
// and admin, plus the DSN of the environment.
type Settings struct {
	SessionTTL       time.Duration  // session_ttl_seconds
	SessionIdle      time.Duration  // session_idle_seconds
	ServerSecretPath string         // server_secret_path
	TrustedProxies   []netip.Prefix // trusted_proxies
	DatabaseDSN      string         // KCR_DATABASE_DSN; never written to a message

	CredentialDir            string         // credential_dir
	CABundleDir              string         // ca_bundle_dir
	AllowedEndpointHostnames []string       // allowed_endpoint_hostnames, normalised and unique
	AllowedEndpointCIDRs     []netip.Prefix // allowed_endpoint_cidrs, unique

	WorkerSlots           int           // worker_slots
	WorkerDrain           time.Duration // worker_drain_seconds
	CollectTimeout        time.Duration // collect_timeout_seconds
	RenderTimeout         time.Duration // render_timeout_seconds
	JobMaxAttempts        int           // job_max_attempts
	ReportRetention       time.Duration // report_retention_days
	ExpiredReportKept     time.Duration // expired_report_days
	AdmissionLimit        int           // admission_limit
	AdmissionProfileLimit int           // admission_profile_limit
	AdmissionRetry        time.Duration // admission_retry_seconds
}

// tomlSettings mirrors the keys of the deployment file. A key the file does
// not name keeps the default it was decoded over.
type tomlSettings struct {
	SessionTTLSeconds  int64    `toml:"session_ttl_seconds"`
	SessionIdleSeconds int64    `toml:"session_idle_seconds"`
	ServerSecretPath   string   `toml:"server_secret_path"`
	TrustedProxies     []string `toml:"trusted_proxies"`

	CredentialDir            string   `toml:"credential_dir"`
	CABundleDir              string   `toml:"ca_bundle_dir"`
	AllowedEndpointHostnames []string `toml:"allowed_endpoint_hostnames"`
	AllowedEndpointCIDRs     []string `toml:"allowed_endpoint_cidrs"`

	WorkerSlots           int64 `toml:"worker_slots"`
	WorkerDrainSeconds    int64 `toml:"worker_drain_seconds"`
	CollectTimeoutSeconds int64 `toml:"collect_timeout_seconds"`
	RenderTimeoutSeconds  int64 `toml:"render_timeout_seconds"`
	JobMaxAttempts        int64 `toml:"job_max_attempts"`
	ReportRetentionDays   int64 `toml:"report_retention_days"`
	ExpiredReportDays     int64 `toml:"expired_report_days"`
	AdmissionLimit        int64 `toml:"admission_limit"`
	AdmissionProfileLimit int64 `toml:"admission_profile_limit"`
	AdmissionRetrySeconds int64 `toml:"admission_retry_seconds"`
}

// Load reads the TOML file at path, then the environment through getenv,
// and validates the result. An unknown key or section is an error that
// names the key and not its value. The DSN is read from KCR_DATABASE_DSN
// and is required.
func Load(path string, getenv func(string) string) (Settings, error) {
	file := tomlSettings{
		SessionTTLSeconds:  43200,
		SessionIdleSeconds: 3600,
		CredentialDir:      defaultCredentialDir,
		CABundleDir:        defaultCABundleDir,

		WorkerSlots:           2,
		WorkerDrainSeconds:    30,
		CollectTimeoutSeconds: 900,
		RenderTimeoutSeconds:  300,
		JobMaxAttempts:        3,
		ReportRetentionDays:   7,
		ExpiredReportDays:     30,
		AdmissionLimit:        20,
		AdmissionProfileLimit: 5,
		AdmissionRetrySeconds: 5,
	}
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
	for _, k := range []struct {
		key        string
		value, min int64
		max        int64
	}{
		{"worker_slots", file.WorkerSlots, 1, 16},
		{"worker_drain_seconds", file.WorkerDrainSeconds, 1, 3600},
		{"collect_timeout_seconds", file.CollectTimeoutSeconds, 60, 3600},
		{"render_timeout_seconds", file.RenderTimeoutSeconds, 10, 3600},
		{"job_max_attempts", file.JobMaxAttempts, 1, 10},
		{"report_retention_days", file.ReportRetentionDays, 1, 365},
		{"expired_report_days", file.ExpiredReportDays, 1, 365},
		{"admission_limit", file.AdmissionLimit, 1, 1000},
		{"admission_profile_limit", file.AdmissionProfileLimit, 1, 1000},
		{"admission_retry_seconds", file.AdmissionRetrySeconds, 1, 300},
	} {
		if k.value < k.min || k.value > k.max {
			return Settings{}, fmt.Errorf("%s must be between %d and %d", k.key, k.min, k.max)
		}
	}
	if file.AdmissionProfileLimit > file.AdmissionLimit {
		return Settings{}, errProfileOverLimit
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
	if err := checkFolder("credential_dir", file.CredentialDir); err != nil {
		return Settings{}, err
	}
	if err := checkFolder("ca_bundle_dir", file.CABundleDir); err != nil {
		return Settings{}, err
	}
	hostnames, err := parseHostnames(file.AllowedEndpointHostnames)
	if err != nil {
		return Settings{}, err
	}
	cidrs, err := parseEndpointCIDRs(file.AllowedEndpointCIDRs)
	if err != nil {
		return Settings{}, err
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

		CredentialDir:            file.CredentialDir,
		CABundleDir:              file.CABundleDir,
		AllowedEndpointHostnames: hostnames,
		AllowedEndpointCIDRs:     cidrs,

		WorkerSlots:           int(file.WorkerSlots),
		WorkerDrain:           time.Duration(file.WorkerDrainSeconds) * time.Second,
		CollectTimeout:        time.Duration(file.CollectTimeoutSeconds) * time.Second,
		RenderTimeout:         time.Duration(file.RenderTimeoutSeconds) * time.Second,
		JobMaxAttempts:        int(file.JobMaxAttempts),
		ReportRetention:       time.Duration(file.ReportRetentionDays) * 24 * time.Hour,
		ExpiredReportKept:     time.Duration(file.ExpiredReportDays) * 24 * time.Hour,
		AdmissionLimit:        int(file.AdmissionLimit),
		AdmissionProfileLimit: int(file.AdmissionProfileLimit),
		AdmissionRetry:        time.Duration(file.AdmissionRetrySeconds) * time.Second,
	}, nil
}

// checkFolder judges the shape of a reference folder: absolute, with no ..
// segment. The folder is not touched.
func checkFolder(key, dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("%s must be absolute", key)
	}
	for _, segment := range strings.Split(dir, "/") {
		if segment == ".." {
			return fmt.Errorf("%s must not hold a .. segment", key)
		}
	}
	return nil
}

// parseHostnames normalises each entry and refuses a malformed or repeated
// one, naming its 1-based position and never its value.
func parseHostnames(entries []string) ([]string, error) {
	var names []string
	seen := make(map[string]bool)
	for i, entry := range entries {
		name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(entry), "."))
		if !netpolicy.ValidHostname(name) {
			return nil, fmt.Errorf("allowed_endpoint_hostnames entry %d is not a host name", i+1)
		}
		if seen[name] {
			return nil, fmt.Errorf("allowed_endpoint_hostnames repeats entry %d", i+1)
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, nil
}

// parseEndpointCIDRs refuses a malformed block, a block with host bits, an
// IPv4-mapped block, which never matches an address that is compared in its
// IPv4 form, a block that covers a metadata address along with others, and
// a repeat, naming its 1-based position and never its value.
func parseEndpointCIDRs(entries []string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	seen := make(map[netip.Prefix]bool)
	for i, entry := range entries {
		prefix, err := netip.ParsePrefix(entry)
		if err != nil || prefix != prefix.Masked() {
			return nil, fmt.Errorf("allowed_endpoint_cidrs entry %d is not a CIDR block", i+1)
		}
		if prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("allowed_endpoint_cidrs entry %d is an IPv4-mapped prefix; write the IPv4 form", i+1)
		}
		for _, metadata := range metadataAddresses {
			if prefix.Contains(metadata) && prefix.Bits() < metadata.BitLen() {
				return nil, fmt.Errorf("allowed_endpoint_cidrs entry %d covers a metadata address; list that single address instead", i+1)
			}
		}
		if seen[prefix] {
			return nil, fmt.Errorf("allowed_endpoint_cidrs repeats entry %d", i+1)
		}
		seen[prefix] = true
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
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
