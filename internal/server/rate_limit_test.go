package server

import "testing"

func TestPrefixKey(t *testing.T) {
	if got := prefixKey("203.0.113.9"); got != "v4:203.0.113.0/24" {
		t.Fatalf("got %q", got)
	}
	if got := prefixKey("2001:db8::1"); got != "v6:2001:db8::/64" {
		t.Fatalf("got %q", got)
	}
}
