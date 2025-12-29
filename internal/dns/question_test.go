package dns

import (
	"testing"
)

func TestQuestionMarshal(t *testing.T) {
	q := Question{
		Name:  "example.com",
		Type:  uint16(TypeA),
		Class: 1, // IN
	}

	b, err := q.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: encoded name (13 bytes) + type (2) + class (2) = 17 bytes
	// Name: 7 + 'example' + 3 + 'com' + 0 = 1+7+1+3+1 = 13
	expectedMinLen := 13 + 4
	if len(b) < expectedMinLen {
		t.Errorf("expected at least %d bytes, got %d", expectedMinLen, len(b))
	}

	// Last 4 bytes should be type and class
	typeVal := int(b[len(b)-4])<<8 | int(b[len(b)-3])
	classVal := int(b[len(b)-2])<<8 | int(b[len(b)-1])

	if typeVal != int(TypeA) {
		t.Errorf("expected type %d, got %d", TypeA, typeVal)
	}
	if classVal != 1 {
		t.Errorf("expected class 1, got %d", classVal)
	}
}

func TestQuestionMarshalInvalidName(t *testing.T) {
	// Create a name with a label too long
	longLabel := make([]byte, 70)
	for i := range longLabel {
		longLabel[i] = 'a'
	}
	q := Question{
		Name:  string(longLabel) + ".com",
		Type:  uint16(TypeA),
		Class: 1,
	}

	_, err := q.Marshal()
	if err == nil {
		t.Error("expected error for invalid name")
	}
}

func TestParseQuestion(t *testing.T) {
	// Build a question section
	// Name: www.example.com (3www7example3com0)
	msg := []byte{
		3, 'w', 'w', 'w',
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
		0, 1, // Type A
		0, 1, // Class IN
	}

	off := 0
	q, err := ParseQuestion(msg, &off)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.Name != "www.example.com" {
		t.Errorf("expected name www.example.com, got %s", q.Name)
	}
	if q.Type != uint16(TypeA) {
		t.Errorf("expected type %d, got %d", TypeA, q.Type)
	}
	if q.Class != 1 {
		t.Errorf("expected class 1, got %d", q.Class)
	}
	if off != len(msg) {
		t.Errorf("expected offset %d, got %d", len(msg), off)
	}
}

func TestParseQuestionTruncated(t *testing.T) {
	// Name without type/class
	msg := []byte{
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
		// Missing type and class
	}

	off := 0
	_, err := ParseQuestion(msg, &off)
	if err == nil {
		t.Error("expected error for truncated question")
	}
}

func TestQuestionRoundTrip(t *testing.T) {
	original := Question{
		Name:  "test.example.com",
		Type:  uint16(TypeAAAA),
		Class: 1,
	}

	b, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	off := 0
	parsed, err := ParseQuestion(b, &off)
	if err != nil {
		t.Fatalf("ParseQuestion failed: %v", err)
	}

	if parsed.Name != original.Name {
		t.Errorf("name: got %s, want %s", parsed.Name, original.Name)
	}
	if parsed.Type != original.Type {
		t.Errorf("type: got %d, want %d", parsed.Type, original.Type)
	}
	if parsed.Class != original.Class {
		t.Errorf("class: got %d, want %d", parsed.Class, original.Class)
	}
}

func TestParseQuestionMultiple(t *testing.T) {
	// Two questions back to back
	msg := []byte{
		// Question 1: example.com A
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
		0, 1, // Type A
		0, 1, // Class IN
		// Question 2: test.com AAAA
		4, 't', 'e', 's', 't',
		3, 'c', 'o', 'm',
		0,
		0, 28, // Type AAAA
		0, 1, // Class IN
	}

	off := 0

	q1, err := ParseQuestion(msg, &off)
	if err != nil {
		t.Fatalf("failed to parse question 1: %v", err)
	}
	if q1.Name != "example.com" {
		t.Errorf("q1 name: got %s, want example.com", q1.Name)
	}
	if q1.Type != uint16(TypeA) {
		t.Errorf("q1 type: got %d, want %d", q1.Type, TypeA)
	}

	q2, err := ParseQuestion(msg, &off)
	if err != nil {
		t.Fatalf("failed to parse question 2: %v", err)
	}
	if q2.Name != "test.com" {
		t.Errorf("q2 name: got %s, want test.com", q2.Name)
	}
	if q2.Type != uint16(TypeAAAA) {
		t.Errorf("q2 type: got %d, want %d", q2.Type, TypeAAAA)
	}
}
