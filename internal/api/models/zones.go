package models

// ZoneSummary is a brief zone description.
type ZoneSummary struct {
	Name        string `json:"name"`
	RecordCount int    `json:"record_count"`
	FilePath    string `json:"file_path,omitempty"`
}

// ZoneListResponse contains a list of zones.
type ZoneListResponse struct {
	Zones []ZoneSummary `json:"zones"`
	Count int           `json:"count"`
}

// ZoneDetailResponse contains full zone details.
type ZoneDetailResponse struct {
	Name    string       `json:"name"`
	Records []ZoneRecord `json:"records"`
}

// ZoneRecord represents a single DNS record in a zone.
type ZoneRecord struct {
	Name  string `json:"name"`
	TTL   uint32 `json:"ttl"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ZoneCreateRequest is used to create a new zone.
type ZoneCreateRequest struct {
	Name    string       `json:"name" binding:"required"`
	Records []ZoneRecord `json:"records"`
}
