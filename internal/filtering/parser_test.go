package filtering

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain sets a global timeout for all tests in this package.
func TestMain(m *testing.M) {
	// Set 30 second timeout for the entire test suite
	go func() {
		time.Sleep(30 * time.Second)
		os.Exit(1) // Force exit if tests take too long
	}()
	os.Exit(m.Run())
}

func TestParser_ParseDomainsList(t *testing.T) {
	content := `# Comment line
example.com
ads.example.com
tracker.example.org

# Another comment
malware.com
`

	file := createTempFile(t, content)
	defer os.Remove(file)

	parser := NewParser()
	trie, err := parser.ParseFile(file, FormatDomains)
	require.NoError(t, err, "ParseFile failed")
	assert.Equal(t, 4, trie.Size(), "Expected 4 domains")

	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", true},
		{"ads.example.com", true},
		{"tracker.example.org", true},
		{"malware.com", true},
		{"safe.com", false},
		{"sub.example.com", true}, // wildcard match
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, trie.Contains(tt.domain), "Contains(%q)", tt.domain)
	}
}

func TestParser_ParseHostsFile(t *testing.T) {
	content := `# StevenBlack hosts file format
127.0.0.1 localhost
::1 localhost
0.0.0.0 ads.example.com
0.0.0.0 tracker.example.org extra ignored
0.0.0.0 malware.com # with comment
`

	file := createTempFile(t, content)
	defer os.Remove(file)

	parser := NewParser()
	trie, err := parser.ParseFile(file, FormatHosts)
	require.NoError(t, err, "ParseFile failed")

	// Should only have 3 domains (not localhost entries)
	assert.Equal(t, 3, trie.Size(), "Expected 3 domains")

	tests := []struct {
		domain   string
		expected bool
	}{
		{"ads.example.com", true},
		{"tracker.example.org", true},
		{"malware.com", true},
		{"localhost", false}, // localhost should be ignored
		{"safe.com", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, trie.Contains(tt.domain), "Contains(%q)", tt.domain)
	}
}

func TestParser_ParseAdblockFormat(t *testing.T) {
	content := `! Adblock-style blocklist
! Title: Test List
||ads.example.com^
||tracker.example.org^
||malware.com^
! Some domains with wildcards
||*.doubleclick.net^
! Plain domains (should be parsed)
example-ad.com
`

	file := createTempFile(t, content)
	defer os.Remove(file)

	parser := NewParser()
	trie, err := parser.ParseFile(file, FormatAdblock)
	require.NoError(t, err, "ParseFile failed")
	assert.GreaterOrEqual(t, trie.Size(), 3, "Expected at least 3 domains")

	tests := []struct {
		domain   string
		expected bool
	}{
		{"ads.example.com", true},
		{"tracker.example.org", true},
		{"malware.com", true},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, trie.Contains(tt.domain), "Contains(%q)", tt.domain)
	}
}

func TestParser_AutoDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected ListFormat
	}{
		{
			name:     "adblock format",
			content:  "||ads.example.com^\n||tracker.com^\n",
			expected: FormatAdblock,
		},
		{
			name:     "hosts format",
			content:  "0.0.0.0 ads.example.com\n0.0.0.0 tracker.com\n",
			expected: FormatHosts,
		},
		{
			name:     "domains format",
			content:  "ads.example.com\ntracker.com\nmalware.org\n",
			expected: FormatDomains,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createTempFile(t, tt.content)
			defer os.Remove(file)

			parser := NewParser()
			trie, err := parser.ParseFile(file, FormatAuto)
			require.NoError(t, err, "ParseFile failed")
			assert.GreaterOrEqual(t, trie.Size(), 1, "Expected at least 1 domain")
		})
	}
}

func TestParser_ParseURL(t *testing.T) {
	content := `||ads.example.com^
||tracker.example.org^
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer server.Close()

	parser := NewParser()
	trie, err := parser.ParseURL(server.URL, FormatAdblock)
	require.NoError(t, err, "ParseURL failed")
	assert.Equal(t, 2, trie.Size(), "Expected 2 domains")
}

func TestParser_ParseURLTimeout(t *testing.T) {
	// Use a non-routable IP to trigger timeout without blocking
	parser := NewParser()
	parser.SetTimeout(50) // 50ms timeout

	// Use a non-routable IP that will timeout
	_, err := parser.ParseURL("http://10.255.255.1:12345/blocklist.txt", FormatAuto)
	assert.Error(t, err, "Expected timeout error")
}

func TestParser_InvalidFile(t *testing.T) {
	parser := NewParser()
	_, err := parser.ParseFile("/nonexistent/file.txt", FormatAuto)
	assert.Error(t, err, "Expected error for nonexistent file")
}

func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"a.b.c.example.com", true},
		{"example-site.com", true},
		{"example123.com", true},
		{"", false},
		{".", false},
		{"..", false},
		{"example", false}, // TLD-only is invalid (no dot)
		{"-example.com", false},
		{"example-.com", false},
		{"example..com", false},
		{"localhost", false}, // no dot
		{"local", false},     // no dot
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			assert.Equal(t, tt.expected, isValidDomain(tt.domain))
		})
	}
}

func TestParser_ParseDomainsSlice(t *testing.T) {
	parser := NewParser()

	domains := []string{
		"example.com",
		"test.org",
		"invalid", // no dot
		"valid.net",
		"", // empty
	}

	trie := parser.ParseDomainsSlice(domains)

	assert.Equal(t, 3, trie.Size(), "expected 3 valid domains")
	assert.True(t, trie.Contains("example.com"), "expected trie to contain example.com")
	assert.True(t, trie.Contains("test.org"), "expected trie to contain test.org")
	assert.True(t, trie.Contains("valid.net"), "expected trie to contain valid.net")
	assert.False(t, trie.Contains("invalid"), "expected trie NOT to contain invalid domain")
}

func TestParser_ParseDomainsSlice_Empty(t *testing.T) {
	parser := NewParser()
	trie := parser.ParseDomainsSlice(nil)
	assert.Equal(t, 0, trie.Size(), "expected empty trie")
}

// Helper function to create temporary test files.
func createTempFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	file := filepath.Join(dir, "blocklist.txt")
	err := os.WriteFile(file, []byte(content), 0644)
	require.NoError(t, err, "Failed to create temp file")
	return file
}
