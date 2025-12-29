package dns

import "testing"

func TestEncodeName(t *testing.T) {
	b, err := EncodeName("google.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	exp := []byte{6, 'g', 'o', 'o', 'g', 'l', 'e', 3, 'c', 'o', 'm', 0}
	if string(b) != string(exp) {
		t.Fatalf("got %v want %v", b, exp)
	}
}

func TestDecodeName_Uncompressed(t *testing.T) {
	msg := []byte{3, 'w', 'w', 'w', 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0}
	off := 0
	n, err := DecodeName(msg, &off)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != "www.example.com" {
		t.Fatalf("got %q", n)
	}
	if off != len(msg) {
		t.Fatalf("off=%d", off)
	}
}
