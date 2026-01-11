// Package models defines request and response types for the HydraDNS REST API.
// All types are JSON-serializable and include validation tags where appropriate.
package models

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// StatusResponse represents a simple status response.
type StatusResponse struct {
	Status string `json:"status"`
}
