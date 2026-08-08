// Package proxyx provides outbound proxy dialing for the proxy pool.
//
// It supports the following proxy URL schemes:
//
//   - http:// / https://  — HTTP(S) proxy (CONNECT tunneling)
//   - socks5://           — SOCKS5 proxy
//   - ss://               — Shadowsocks (AEAD methods, SIP002 URI)
//   - vmess://            — VMess (v2ray share-link JSON)
//   - vless://            — VLESS (share-link URI)
//   - trojan://           — Trojan (share-link URI)
//
// The main entry point is NewDialContext, which returns a dial function
// compatible with http.Transport.DialContext.
package proxyx

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ProxyScheme is the normalized scheme of a proxy URL.
type ProxyScheme string

const (
	SchemeHTTP   ProxyScheme = "http"
	SchemeHTTPS  ProxyScheme = "https"
	SchemeSOCKS5 ProxyScheme = "socks5"
	SchemeSS     ProxyScheme = "ss"
	SchemeVMess  ProxyScheme = "vmess"
	SchemeVLESS  ProxyScheme = "vless"
	SchemeTrojan ProxyScheme = "trojan"
)

// SupportedSchemes lists every scheme accepted by NormalizeURL.
var SupportedSchemes = []ProxyScheme{
	SchemeHTTP, SchemeHTTPS, SchemeSOCKS5,
	SchemeSS, SchemeVMess, SchemeVLESS, SchemeTrojan,
}

// IsSupportedScheme reports whether s is a supported proxy scheme.
func IsSupportedScheme(s string) bool {
	for _, scheme := range SupportedSchemes {
		if string(scheme) == strings.ToLower(strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}

// NormalizeURL validates a proxy URL and normalizes the scheme/host casing.
// Legacy "socks://" URLs are normalized to "socks5://". The raw value is
// preserved as-is (except for the socks alias) because base64 payloads inside
// ss:// / vmess:// links must not be re-encoded by url.URL.String().
func NormalizeURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("proxy url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid proxy url: %w", err)
	}
	rawScheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	switch rawScheme {
	case "socks":
		// Legacy alias: rewrite the prefix to socks5:// and keep the rest
		// byte-for-byte identical (host, payloads, credentials). Match the
		// scheme prefix case-insensitively, then append the remainder. The
		// host is still validated below.
		idx := strings.Index(trimmed, "://")
		if idx < 0 {
			return "", fmt.Errorf("invalid proxy url: missing '://'")
		}
		trimmed = "socks5://" + trimmed[idx+3:]
		parsed, err = url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("invalid proxy url: %w", err)
		}
		// Fall through to the shared host check.
	case "http", "https", "socks5", "ss", "vmess", "vless", "trojan":
		// Supported as-is.
	default:
		return "", fmt.Errorf("unsupported proxy scheme: %s", rawScheme)
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return "", fmt.Errorf("proxy url must have a host")
	}
	return trimmed, nil
}

// ssParsed is the decoded Shadowsocks share link.
type ssParsed struct {
	Method   string
	Password string
	Server   string
	Port     int
}

// parseSS parses a Shadowsocks share link.
//
// Accepted formats:
//
//	ss://method:password@host:port
//	ss://base64(method:password@host:port)[#tag]
//	ss://base64url(method:password)@host:port[?plugin=...][#tag]
//
// The credentials part is split manually on the last '@' instead of relying on
// url.Parse: base64 payloads may contain ':', '/', '+', '=' which url.Parse
// would misinterpret as userinfo/host separators (e.g. standard base64 with
// '/' truncates the host at the slash).
func parseSS(raw string) (*ssParsed, error) {
	schemeIdx := strings.Index(raw, "://")
	if schemeIdx < 0 {
		return nil, fmt.Errorf("invalid ss url: missing '://'")
	}
	rest := raw[schemeIdx+3:]
	if fragIdx := strings.Index(rest, "#"); fragIdx >= 0 {
		rest = rest[:fragIdx]
	}
	at := strings.LastIndex(rest, "@")

	var cred, serverPart string
	if at >= 0 {
		// Modern form: credentials before '@', host:port after.
		cred = rest[:at]
		serverPart = rest[at+1:]
	} else {
		// Legacy whole-link form: ss://base64(method:password@host:port). The
		// '@' only appears after decoding. Decode first, then re-split.
		decodedBytes, err := decodeBase64URL(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid ss url: %w", err)
		}
		decoded := string(decodedBytes)
		innerAt := strings.LastIndex(decoded, "@")
		if innerAt < 0 {
			return nil, fmt.Errorf("invalid ss url: missing '@'")
		}
		cred = decoded[:innerAt]
		serverPart = decoded[innerAt+1:]
	}

	host, portStr, err := net.SplitHostPort(serverPart)
	if err != nil {
		return nil, fmt.Errorf("invalid ss server: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid ss port: %q", portStr)
	}
	if host == "" {
		return nil, fmt.Errorf("ss server is required")
	}

	var method, password string
	if isPlainSSMethod(cred) {
		sep := strings.Index(cred, ":")
		method = cred[:sep]
		password = cred[sep+1:]
	} else {
		decodedBytes, err := decodeBase64URL(cred)
		if err != nil {
			return nil, fmt.Errorf("invalid ss credentials: %w", err)
		}
		decoded := string(decodedBytes)
		// Legacy whole-link form decoded with an embedded '@': strip it.
		if innerAt := strings.LastIndex(decoded, "@"); innerAt >= 0 {
			decoded = decoded[:innerAt]
		}
		sep := strings.Index(decoded, ":")
		if sep < 0 {
			return nil, fmt.Errorf("invalid ss credentials: missing ':'")
		}
		method = decoded[:sep]
		password = decoded[sep+1:]
	}

	if method == "" {
		return nil, fmt.Errorf("ss method is required")
	}
	if password == "" {
		return nil, fmt.Errorf("ss password is required")
	}
	return &ssParsed{Method: method, Password: password, Server: host, Port: port}, nil
}

// knownSSMethods is the set of method names sing-shadowsocks accepts, used to
// distinguish a plain "method:password" credential from a base64 blob.
var knownSSMethods = func() map[string]bool {
	names := []string{
		// shadowaead
		"aes-128-gcm", "aes-192-gcm", "aes-256-gcm",
		"chacha20-ietf-poly1305", "xchacha20-ietf-poly1305",
		// shadowstream (stream ciphers)
		"rc4-md5", "aes-128-cfb", "aes-192-cfb", "aes-256-cfb",
		"aes-128-ctr", "aes-192-ctr", "aes-256-ctr",
		"chacha20-ietf", "xchacha20", "salsa20", "chacha20", "rc4",
		// shadowaead_2022
		"2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm",
		"2022-blake3-chacha20-poly1305",
		"none", "plain", "dummy",
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}()

// isPlainSSMethod reports whether cred looks like a plain "method:password"
// pair with a recognized method name, rather than a base64 blob.
func isPlainSSMethod(cred string) bool {
	sep := strings.Index(cred, ":")
	if sep <= 0 {
		return false
	}
	return knownSSMethods[strings.ToLower(cred[:sep])]
}

// vmessParsed is the decoded VMess share link (v2ray JSON).
type vmessParsed struct {
	UUID          string
	AlterID       int
	Security      string
	Server        string
	Port          int
	TLS           bool
	SNI           string
	AllowInsecure bool
}

// parseVMess parses a VMess share link: vmess://base64(JSON).
func parseVMess(raw string) (*vmessParsed, error) {
	// Scheme 大小写不敏感：校验层（NormalizeProxyURL）与其余协议的解析
	// （url.Parse / parseSS 的 Index）都接受 VMESS://，此处保持一致。
	payload := raw
	if len(raw) >= len("vmess://") && strings.EqualFold(raw[:len("vmess://")], "vmess://") {
		payload = raw[len("vmess://"):]
	}
	// Some clients emit vmess:// with an extra slash or URL-encoded payload.
	payload = strings.TrimPrefix(payload, "/")
	if idx := strings.Index(payload, "#"); idx >= 0 {
		payload = payload[:idx]
	}
	decoded, err := decodeBase64URL(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid vmess base64 payload: %w", err)
	}
	var doc struct {
		V        string `json:"v"`
		Ps       string `json:"ps"`
		Add      string `json:"add"`
		Port     any    `json:"port"`
		ID       string `json:"id"`
		AID      any    `json:"aid"`
		Net      string `json:"net"`
		Type     string `json:"type"`
		Host     string `json:"host"`
		Path     string `json:"path"`
		TLS      string `json:"tls"`
		SNI      string `json:"sni"`
		Alpn     string `json:"alpn"`
		Scy      string `json:"scy"`
		Fp       string `json:"fp"`
		Insecure any    `json:"allowInsecure"`
	}
	if err := json.Unmarshal(decoded, &doc); err != nil {
		return nil, fmt.Errorf("invalid vmess json: %w", err)
	}
	server := strings.TrimSpace(doc.Add)
	if server == "" {
		return nil, fmt.Errorf("vmess server (add) is required")
	}
	port, err := jsonNumberToInt(doc.Port)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid vmess port")
	}
	uuid := strings.TrimSpace(doc.ID)
	if uuid == "" {
		return nil, fmt.Errorf("vmess id is required")
	}
	alterID, _ := jsonNumberToInt(doc.AID)
	if alterID < 0 {
		alterID = 0
	}
	security := strings.ToLower(strings.TrimSpace(doc.Scy))
	if security == "" {
		security = "auto"
	}
	tlsEnabled := strings.EqualFold(strings.TrimSpace(doc.TLS), "tls")
	sni := strings.TrimSpace(doc.SNI)
	if sni == "" && tlsEnabled {
		sni = server
	}
	allowInsecure := false
	if insecure, ok := doc.Insecure.(bool); ok {
		allowInsecure = insecure
	}
	return &vmessParsed{
		UUID:          uuid,
		AlterID:       alterID,
		Security:      security,
		Server:        server,
		Port:          port,
		TLS:           tlsEnabled,
		SNI:           sni,
		AllowInsecure: allowInsecure,
	}, nil
}

// vlessParsed is the decoded VLESS share link.
type vlessParsed struct {
	UUID          string
	Server        string
	Port          int
	TLS           bool
	SNI           string
	AllowInsecure bool
	Flow          string
}

// parseVLESS parses a VLESS share link: vless://uuid@host:port?params#tag.
func parseVLESS(raw string) (*vlessParsed, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid vless url: %w", err)
	}
	uuid := parsed.User.Username()
	if uuid == "" {
		return nil, fmt.Errorf("vless uuid is required")
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("vless server is required")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid vless port")
	}
	query := parsed.Query()
	security := strings.ToLower(query.Get("security"))
	tlsEnabled := security == "tls" || security == "reality"
	sni := strings.TrimSpace(query.Get("sni"))
	if sni == "" && tlsEnabled {
		sni = host
	}
	allowInsecure := strings.EqualFold(query.Get("allowInsecure"), "1") ||
		strings.EqualFold(query.Get("allow_insecure"), "true")
	flow := strings.TrimSpace(query.Get("flow"))
	return &vlessParsed{
		UUID:          uuid,
		Server:        host,
		Port:          port,
		TLS:           tlsEnabled,
		SNI:           sni,
		AllowInsecure: allowInsecure,
		Flow:          flow,
	}, nil
}

// trojanParsed is the decoded Trojan share link.
type trojanParsed struct {
	Password      string
	Server        string
	Port          int
	SNI           string
	AllowInsecure bool
}

// parseTrojan parses a Trojan share link: trojan://password@host:port?params#tag.
func parseTrojan(raw string) (*trojanParsed, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid trojan url: %w", err)
	}
	password := parsed.User.Username()
	if password == "" {
		return nil, fmt.Errorf("trojan password is required")
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("trojan server is required")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid trojan port")
	}
	query := parsed.Query()
	security := strings.ToLower(query.Get("security"))
	if security != "" && security != "tls" {
		return nil, fmt.Errorf("trojan only supports tls security, got %q", security)
	}
	sni := strings.TrimSpace(query.Get("sni"))
	if sni == "" {
		sni = host
	}
	allowInsecure := strings.EqualFold(query.Get("allowInsecure"), "1") ||
		strings.EqualFold(query.Get("allow_insecure"), "true")
	return &trojanParsed{
		Password:      password,
		Server:        host,
		Port:          port,
		SNI:           sni,
		AllowInsecure: allowInsecure,
	}, nil
}

// decodeBase64URL decodes base64 in standard, raw-standard, URL-safe, or
// raw-URL-safe encodings.
func decodeBase64URL(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, enc := range encodings {
		decoded, err := enc.DecodeString(s)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// jsonNumberToInt converts a JSON number (float64 or string) to an int.
func jsonNumberToInt(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(n))
	case nil:
		return 0, fmt.Errorf("missing number")
	default:
		return 0, fmt.Errorf("unsupported number type %T", v)
	}
}

// parseServerAddr builds a host:port target from a parsed struct.
func parseServerAddr(server string, port int) string {
	return net.JoinHostPort(server, strconv.Itoa(port))
}
