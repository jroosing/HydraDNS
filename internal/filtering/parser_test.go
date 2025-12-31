package filtering

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if trie.Size() != 4 {
		t.Errorf("Expected 4 domains, got %d", trie.Size())
	}

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
		if got := trie.Contains(tt.domain); got != tt.expected {
			t.Errorf("Contains(%q) = %v, want %v", tt.domain, got, tt.expected)
		}
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
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Should only have 3 domains (not localhost entries)
	if trie.Size() != 3 {
		t.Errorf("Expected 3 domains, got %d", trie.Size())
	}

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
		if got := trie.Contains(tt.domain); got != tt.expected {
			t.Errorf("Contains(%q) = %v, want %v", tt.domain, got, tt.expected)
		}
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
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if trie.Size() < 3 {
		t.Errorf("Expected at least 3 domains, got %d", trie.Size())
	}

	tests := []struct {
		domain   string
		expected bool
	}{
		{"ads.example.com", true},
		{"tracker.example.org", true},
		{"malware.com", true},
	}

	for _, tt := range tests {
		if got := trie.Contains(tt.domain); got != tt.expected {
			t.Errorf("Contains(%q) = %v, want %v", tt.domain, got, tt.expected)
		}
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
			if err != nil {
				t.Fatalf("ParseFile failed: %v", err)
			}

			if trie.Size() < 1 {
				t.Errorf("Expected at least 1 domain, got %d", trie.Size())
			}
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
	if err != nil {
		t.Fatalf("ParseURL failed: %v", err)
	}

	if trie.Size() != 2 {
		t.Errorf("Expected 2 domains, got %d", trie.Size())
	}
}

func TestParser_ParseURLTimeout(t *testing.T) {
	// Use a non-routable IP to trigger timeout without blocking
	parser := NewParser()
	parser.SetTimeout(50) // 50ms timeout

	// Use a non-routable IP that will timeout
	_, err := parser.ParseURL("http://10.255.255.1:12345/blocklist.txt", FormatAuto)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestParser_InvalidFile(t *testing.T) {
	parser := NewParser()
	_, err := parser.ParseFile("/nonexistent/file.txt", FormatAuto)
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
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
			if got := isValidDomain(tt.domain); got != tt.expected {
				t.Errorf("isValidDomain(%q) = %v, want %v", tt.domain, got, tt.expected)
			}
		})
	}
}

// Helper function to create temporary test files.
func createTempFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	file := filepath.Join(dir, "blocklist.txt")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return file
}
