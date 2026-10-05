package netpolicy

import (
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// MaxURLLength bounds a URL.
const MaxURLLength = 2048

var hostPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$`)

// Origin is a validated endpoint.
type Origin struct {
	Scheme  string // http or https
	Host    string // lower-case, no trailing dot; an IP literal without zone
	Port    int    // the explicit port or the scheme's default
	Path    string // the canonical path, "/" when empty
	Literal bool   // Host is an IP literal
}

func (o Origin) defaultPort() int {
	if o.Scheme == "https" {
		return 443
	}
	return 80
}

// Authority is host, or host:port when the port is not the default, the host
// bracketed when IPv6.
func (o Origin) Authority() string {
	host := o.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if o.Port == o.defaultPort() {
		return host
	}
	return host + ":" + strconv.Itoa(o.Port)
}

// URL is the canonical text: scheme, authority, and the path ("" for "/").
func (o Origin) URL() string {
	text := o.Scheme + "://" + o.Authority()
	if o.Path != "/" {
		text += o.Path
	}
	return text
}

// ValidateURL applies the static rules; a literal host must be ClassPermitted.
func (p Policy) ValidateURL(raw string) (Origin, error) {
	reject := refuse(CodeURLRejected)
	if raw == "" || len(raw) > MaxURLLength {
		return Origin{}, reject
	}
	for _, r := range raw {
		if r < 0x21 || r == 0x7f || r == '\\' || r == '?' || r == '#' {
			return Origin{}, reject
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Origin{}, reject
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return Origin{}, reject
	}
	if u.User != nil || u.Host == "" {
		return Origin{}, reject
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return Origin{}, reject
	}

	var literal netip.Addr
	if addr, err := netip.ParseAddr(host); err == nil {
		literal = addr.WithZone("")
		host = literal.String()
	} else if !hostPattern.MatchString(host) {
		return Origin{}, reject
	}

	o := Origin{Scheme: scheme, Host: host, Literal: literal.IsValid()}
	o.Port = o.defaultPort()
	if text := u.Port(); text != "" {
		port, err := strconv.Atoi(text)
		if err != nil || port < 1 || port > 65535 {
			return Origin{}, reject
		}
		o.Port = port
	}

	// The path as written: the text after the authority, which holds no query
	// or fragment.
	rest := raw[strings.Index(raw, "://")+3:]
	path := ""
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		path = rest[slash:]
	}
	o.Path, err = canonicalPath(path)
	if err != nil {
		return Origin{}, err
	}

	if o.Literal && !p.ClassPermitted(literal) {
		return Origin{}, refuse(CodeAddressRejected)
	}
	return o, nil
}

func canonicalPath(path string) (string, error) {
	segments := strings.Split(path, "/")
	if segments[0] == "" {
		segments = segments[1:]
	}
	if n := len(segments); n > 0 && segments[n-1] == "" {
		segments = segments[:n-1]
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || !segmentValid(segment) {
			return "", refuse(CodeURLRejected)
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil || decoded == "" || decoded == "." || decoded == ".." ||
			strings.ContainsAny(decoded, `/\`) {
			return "", refuse(CodeURLRejected)
		}
	}
	if len(segments) == 0 {
		return "/", nil
	}
	return "/" + strings.Join(segments, "/"), nil
}

const segmentChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~!$&'()*+,;=:@%"

func segmentValid(segment string) bool {
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		if !strings.ContainsRune(segmentChars, rune(c)) {
			return false
		}
		if c == '%' && (i+2 >= len(segment) || !isHex(segment[i+1]) || !isHex(segment[i+2])) {
			return false
		}
	}
	return true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
