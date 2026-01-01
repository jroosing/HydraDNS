package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeName(t *testing.T) {
	b, err := EncodeName("google.com")
	require.NoError(t, err)
	exp := []byte{6, 'g', 'o', 'o', 'g', 'l', 'e', 3, 'c', 'o', 'm', 0}
	assert.Equal(t, exp, b)
}

func TestDecodeName_Uncompressed(t *testing.T) {
	msg := []byte{3, 'w', 'w', 'w', 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0}
	off := 0
	n, err := DecodeName(msg, &off)
	require.NoError(t, err)
	assert.Equal(t, "www.example.com", n)
	assert.Equal(t, len(msg), off)
}
