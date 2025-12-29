package dns

import "testing"

func TestParseRequestBoundedRejectsResponse(t *testing.T) {
	// header with QR=1
	msg := make([]byte, 12)
	msg[2] = 0x80
	msg[5] = 1 // qdcount=1
	_, err := ParseRequestBounded(msg)
	if err == nil {
		t.Fatalf("expected error")
	}
}
