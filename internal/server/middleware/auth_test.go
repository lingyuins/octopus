package middleware

import "testing"

func TestIsIPAllowed_SupportsHostPortAndCIDR(t *testing.T) {
	if !isIPAllowed("127.0.0.1:54321", "127.0.0.1") {
		t.Fatal("expected host:port IP to match exact allowed IP")
	}
	if !isIPAllowed("10.1.2.3:443", "10.0.0.0/8") {
		t.Fatal("expected host:port IP to match CIDR")
	}
	if isIPAllowed("invalid:ip", "127.0.0.1") {
		t.Fatal("expected invalid client IP to be rejected")
	}
}
