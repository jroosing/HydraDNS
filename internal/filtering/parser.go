package filtering

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ListFormat represents the format of a blocklist file.
type ListFormat int

const (
	// FormatAuto attempts to auto-detect the format.
	FormatAuto ListFormat = iota
	// FormatDomains is a plain list of domains, one per line.
	FormatDomains
	// FormatHosts is the hosts file format (IP address followed by domain).
	FormatHosts
	// FormatAdblock is the Adblock Plus format (||domain^).
	FormatAdblock
)

// Parser provides methods to parse various blocklist formats.
type Parser struct {
	// IgnoreComments determines whether to skip comment lines.
	IgnoreComments bool
	// TrimWhitespace determines whether to trim whitespace from lines.
	TrimWhitespace bool
	// Timeout is the HTTP request timeout in milliseconds. Default is 60000 (60s).
	Timeout int
}

// NewParser creates a new parser with default settings.
func NewParser() *Parser {
	return &Parser{
		IgnoreComments: true,
		TrimWhitespace: true,
		Timeout:        60000, // 60 seconds default
	}
}

// SetTimeout sets the HTTP timeout in milliseconds.
func (p *Parser) SetTimeout(ms int) {
	p.Timeout = ms
}

// ParseFile parses a blocklist file and returns a trie containing the domains.
func (p *Parser) ParseFile(path string, format ListFormat) (*DomainTrie, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return p.Parse(file, format)
}

// ParseURL fetches and parses a blocklist from a URL.
func (p *Parser) ParseURL(url string, format ListFormat) (*DomainTrie, error) {
	timeout := time.Duration(p.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return p.Parse(resp.Body, format)
}

// Parse parses a blocklist from a reader.
func (p *Parser) Parse(r io.Reader, format ListFormat) (*DomainTrie, error) {
	trie := NewDomainTrie()
	scanner := bufio.NewScanner(r)

	// Increase buffer size for very long lines (some blocklists have them)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if p.TrimWhitespace {
			line = strings.TrimSpace(line)
		}

		if line == "" {
			continue
		}

		// Auto-detect format if needed
		if format == FormatAuto {
			format = p.detectFormat(line)
		}

		domain, wildcard := p.parseLine(line, format)
		if domain != "" {
			trie.Add(domain, wildcard)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	return trie, nil
}

// detectFormat attempts to determine the format from a sample line.
func (p *Parser) detectFormat(line string) ListFormat {
	line = strings.TrimSpace(line)

	// Skip comments for detection
	if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
		return FormatAuto // can't determine from comments
	}

	// Adblock format: starts with ||
	if strings.HasPrefix(line, "||") {
		return FormatAdblock
	}

	// Hosts format: starts with 0.0.0.0 or 127.0.0.1
	if strings.HasPrefix(line, "0.0.0.0") || strings.HasPrefix(line, "127.0.0.1") {
		return FormatHosts
	}

	// Default to domains format
	return FormatDomains
}

// parseLine extracts a domain from a line based on the format.
// Returns the domain and whether it should be treated as a wildcard (block subdomains).
func (p *Parser) parseLine(line string, format ListFormat) (string, bool) {
	// Skip empty lines
	if line == "" {
		return "", false
	}

	// Handle comments
	if p.IgnoreComments {
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			return "", false
		}
	}

	switch format {
	case FormatAdblock:
		return p.parseAdblockLine(line)
	case FormatHosts:
		return p.parseHostsLine(line)
	case FormatDomains:
		return p.parseDomainsLine(line)
	default:
		return p.parseDomainsLine(line)
	}
}

// parseAdblockLine parses an Adblock Plus format line.
// Format: ||domain^ or ||domain^$options
// Also handles: ||domain (without ^)
func (p *Parser) parseAdblockLine(line string) (string, bool) {
	// Skip non-blocking rules
	if strings.HasPrefix(line, "@@") {
		// This is a whitelist rule in Adblock, we handle it separately
		return "", false
	}

	// Skip lines that don't start with ||
	if !strings.HasPrefix(line, "||") {
		return "", false
	}

	// Remove the || prefix
	domain := strings.TrimPrefix(line, "||")

	// Remove ^ suffix and everything after it (options)
	if idx := strings.Index(domain, "^"); idx >= 0 {
		domain = domain[:idx]
	}

	// Remove $ and everything after it (options)
	if idx := strings.Index(domain, "$"); idx >= 0 {
		domain = domain[:idx]
	}

	// Skip if it contains path separators (it's a URL rule, not a domain rule)
	if strings.Contains(domain, "/") {
		return "", false
	}

	// Skip if it contains wildcards in the middle (we handle subdomain wildcards differently)
	if strings.Contains(domain, "*") {
		return "", false
	}

	domain = normalizeDomain(domain)
	if domain == "" || !isValidDomain(domain) {
		return "", false
	}

	// In Adblock format, domain blocking typically blocks all subdomains
	return domain, true
}

// parseHostsLine parses a hosts file format line.
// Format: 0.0.0.0 domain or 127.0.0.1 domain
func (p *Parser) parseHostsLine(line string) (string, bool) {
	// Remove inline comments
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}

	// Split by whitespace
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", false
	}

	// First field should be an IP address (0.0.0.0 or 127.0.0.1)
	ip := fields[0]
	if ip != "0.0.0.0" && ip != "127.0.0.1" {
		return "", false
	}

	// Second field is the domain
	domain := normalizeDomain(fields[1])
	if domain == "" || !isValidDomain(domain) {
		return "", false
	}

	// Skip localhost entries
	if domain == "localhost" || domain == "localhost.localdomain" {
		return "", false
	}

	// Hosts format blocks exact domain only by default
	return domain, true
}

// parseDomainsLine parses a simple domains list format.
// Format: one domain per line
func (p *Parser) parseDomainsLine(line string) (string, bool) {
	// Remove inline comments
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}

	domain := normalizeDomain(strings.TrimSpace(line))
	if domain == "" || !isValidDomain(domain) {
		return "", false
	}

	// Domain list format typically means exact domain match + subdomains
	return domain, true
}

// isValidDomain performs basic validation of a domain name.
func isValidDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}

	// Must have at least one dot (TLD)
	if !strings.Contains(domain, ".") {
		return false
	}

	// Check each label
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}

		// Labels must start and end with alphanumeric
		if !isAlphaNum(label[0]) || !isAlphaNum(label[len(label)-1]) {
			return false
		}

		// Labels can contain alphanumeric and hyphens
		for _, c := range label {
			if !isAlphaNum(byte(c)) && c != '-' {
				return false
			}
		}
	}

	return true
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// ParseDomainsSlice parses a slice of domain strings.
func (p *Parser) ParseDomainsSlice(domains []string) *DomainTrie {
	trie := NewDomainTrie()
	for _, domain := range domains {
		domain = normalizeDomain(domain)
		if domain != "" && isValidDomain(domain) {
			trie.Add(domain, true)
		}
	}
	return trie
}
