package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/models"
)

// ListZones godoc
// @Summary List all zones
// @Description Returns a list of all configured DNS zones
// @Tags zones
// @Produce json
// @Success 200 {object} models.ZoneListResponse
// @Security ApiKeyAuth
// @Router /zones [get]
func (h *Handler) ListZones(c *gin.Context) {
	h.mu.RLock()
	zones := h.zones
	h.mu.RUnlock()

	summaries := make([]models.ZoneSummary, 0, len(zones))
	for _, z := range zones {
		summaries = append(summaries, models.ZoneSummary{
			Name:        z.Origin,
			RecordCount: len(z.Records),
		})
	}

	c.JSON(http.StatusOK, models.ZoneListResponse{
		Zones: summaries,
		Count: len(summaries),
	})
}

// CreateZone godoc
// @Summary Create a new zone
// @Description Creates a new DNS zone with the specified records
// @Tags zones
// @Accept json
// @Produce json
// @Param zone body models.ZoneCreateRequest true "Zone to create"
// @Success 201 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 501 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /zones [post]
func (h *Handler) CreateZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: "zone creation not yet implemented"})
}

// GetZone godoc
// @Summary Get zone details
// @Description Returns detailed information about a specific zone
// @Tags zones
// @Produce json
// @Param name path string true "Zone name"
// @Success 200 {object} models.ZoneDetailResponse
// @Failure 404 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /zones/{name} [get]
func (h *Handler) GetZone(c *gin.Context) {
	name := c.Param("name")

	h.mu.RLock()
	zones := h.zones
	h.mu.RUnlock()

	for _, z := range zones {
		if z.Origin == name || z.Origin == name+"." {
			records := make([]models.ZoneRecord, 0, len(z.Records))
			for _, rr := range z.Records {
				records = append(records, models.ZoneRecord{
					Name:  rr.Name,
					TTL:   rr.TTL,
					Type:  formatRecordType(rr.Type),
					Value: formatRData(rr.RData),
				})
			}
			c.JSON(http.StatusOK, models.ZoneDetailResponse{
				Name:    z.Origin,
				Records: records,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "zone not found"})
}

// UpdateZone godoc
// @Summary Update a zone
// @Description Updates an existing DNS zone
// @Tags zones
// @Accept json
// @Produce json
// @Param name path string true "Zone name"
// @Param zone body models.ZoneCreateRequest true "Zone update"
// @Success 200 {object} models.StatusResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 501 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /zones/{name} [put]
func (h *Handler) UpdateZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: "zone updates not yet implemented"})
}

// DeleteZone godoc
// @Summary Delete a zone
// @Description Deletes an existing DNS zone
// @Tags zones
// @Produce json
// @Param name path string true "Zone name"
// @Success 200 {object} models.StatusResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 501 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /zones/{name} [delete]
func (h *Handler) DeleteZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: "zone deletion not yet implemented"})
}
