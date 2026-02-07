package models

// CustomDNSRecordsResponse is the response for GET /custom-dns.
type CustomDNSRecordsResponse struct {
	Hosts  map[string][]string     `json:"hosts"`
	CNAMEs map[string]string       `json:"cnames"`
	Count  CustomDNSCountsResponse `json:"count"`
}

// CustomDNSCountsResponse contains counts of custom DNS entries.
type CustomDNSCountsResponse struct {
	Hosts  int `json:"hosts"`
	CNAMEs int `json:"cnames"`
	Total  int `json:"total"`
}

// HostRecord represents a single host entry for custom DNS.
type HostRecord struct {
	Name string   `json:"name" binding:"required"`
	IPs  []string `json:"ips"  binding:"required,min=1"`
}

// CNAMERecord represents a single CNAME entry for custom DNS.
type CNAMERecord struct {
	Alias  string `json:"alias"  binding:"required"`
	Target string `json:"target" binding:"required"`
}

// AddHostRequest is the request body for POST /custom-dns/hosts.
type AddHostRequest struct {
	Name string   `json:"name" binding:"required"`
	IPs  []string `json:"ips"  binding:"required,min=1"`
}

// UpdateHostRequest is the request body for PUT /custom-dns/hosts/{name}.
type UpdateHostRequest struct {
	IPs []string `json:"ips" binding:"required,min=1"`
}

// AddCNAMERequest is the request body for POST /custom-dns/cnames.
type AddCNAMERequest struct {
	Alias  string `json:"alias"  binding:"required"`
	Target string `json:"target" binding:"required"`
}

// UpdateCNAMERequest is the request body for PUT /custom-dns/cnames/{alias}.
type UpdateCNAMERequest struct {
	Target string `json:"target" binding:"required"`
}

// CustomDNSOperationResponse is the generic success response for custom DNS operations.
type CustomDNSOperationResponse struct {
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
