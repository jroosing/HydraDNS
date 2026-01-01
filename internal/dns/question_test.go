package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuestionMarshal(t *testing.T) {
	q := Question{
		Name:  "example.com",
		Type:  uint16(TypeA),
		Class: 1, // IN
	}

	b, err := q.Marshal()
	require.NoError(t, err)

	// Expected: encoded name (13 bytes) + type (2) + class (2) = 17 bytes
	// Name: 7 + 'example' + 3 + 'com' + 0 = 1+7+1+3+1 = 13
	expectedMinLen := 13 + 4
	assert.GreaterOrEqual(t, len(b), expectedMinLen)

	// Last 4 bytes should be type and class
	typeVal := int(b[len(b)-4])<<8 | int(b[len(b)-3])
	classVal := int(b[len(b)-2])<<8 | int(b[len(b)-1])

	assert.Equal(t, int(TypeA), typeVal)
	assert.Equal(t, 1, classVal)
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
	assert.Error(t, err, "expected error for invalid name")
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
	require.NoError(t, err)

	assert.Equal(t, "www.example.com", q.Name)
	assert.Equal(t, uint16(TypeA), q.Type)
	assert.Equal(t, uint16(1), q.Class)
	assert.Equal(t, len(msg), off)
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
	assert.Error(t, err, "expected error for truncated question")
}

func TestQuestionRoundTrip(t *testing.T) {
	original := Question{
		Name:  "test.example.com",
		Type:  uint16(TypeAAAA),
		Class: 1,
	}

	b, err := original.Marshal()
	require.NoError(t, err, "Marshal failed")

	off := 0
	parsed, err := ParseQuestion(b, &off)
	require.NoError(t, err, "ParseQuestion failed")

	assert.Equal(t, original.Name, parsed.Name)
	assert.Equal(t, original.Type, parsed.Type)
	assert.Equal(t, original.Class, parsed.Class)
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
	require.NoError(t, err, "failed to parse question 1")
	assert.Equal(t, "example.com", q1.Name)
	assert.Equal(t, uint16(TypeA), q1.Type)

	q2, err := ParseQuestion(msg, &off)
	require.NoError(t, err, "failed to parse question 2")
	assert.Equal(t, "test.com", q2.Name)
	assert.Equal(t, uint16(TypeAAAA), q2.Type)
}
