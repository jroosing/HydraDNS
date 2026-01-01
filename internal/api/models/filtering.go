package models

// FilteringStatsResponse contains filtering statistics.
type FilteringStatsResponse struct {
	Enabled        bool   `json:"enabled"`
	QueriesTotal   uint64 `json:"queries_total"`
	QueriesBlocked uint64 `json:"queries_blocked"`
	QueriesAllowed uint64 `json:"queries_allowed"`
	WhitelistSize  int    `json:"whitelist_size"`
	BlacklistSize  int    `json:"blacklist_size"`
}

// DomainListResponse contains a list of domains.
type DomainListResponse struct {
	Domains []string `json:"domains"`
	Count   int      `json:"count"`
}

// DomainRequest is used to add/remove domains from lists.
type DomainRequest struct {
	Domains []string `json:"domains" binding:"required,min=1"`
}

// DomainDeleteRequest is used to remove domains from lists.
type DomainDeleteRequest struct {
	Domains []string `json:"domains" binding:"required,min=1"`
}

// FilteringEnabledRequest toggles filtering on/off.
type FilteringEnabledRequest struct {
	Enabled bool `json:"enabled"`
}
