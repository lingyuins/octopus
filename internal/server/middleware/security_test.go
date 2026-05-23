package middleware

import "testing"

func TestBuildContentSecurityPolicy_AllowsRequestOriginInConnectSrc(t *testing.T) {
	policy := buildContentSecurityPolicy("https://console.example.com")
	if want := "connect-src 'self' https://console.example.com"; !contains(policy, want) {
		t.Fatalf("policy = %q, want substring %q", policy, want)
	}
}

func TestNormalizeCSPOrigin_StripsPathAndRejectsInvalidValues(t *testing.T) {
	if got := normalizeCSPOrigin(" https://console.example.com/app?x=1 "); got != "https://console.example.com" {
		t.Fatalf("normalizeCSPOrigin() = %q", got)
	}
	if got := normalizeCSPOrigin("null"); got != "" {
		t.Fatalf("normalizeCSPOrigin(null) = %q, want empty", got)
	}
	if got := normalizeCSPOrigin("not a url"); got != "" {
		t.Fatalf("normalizeCSPOrigin(invalid) = %q, want empty", got)
	}
}

func contains(text, part string) bool {
	return len(part) == 0 || (len(text) >= len(part) && (text == part || contains(text[1:], part) || text[:len(part)] == part))
}
