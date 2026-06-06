package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/platform/clientip"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuditHandler struct {
	audit *repository.AuditRepository
}

func NewAuditHandler(audit *repository.AuditRepository) *AuditHandler {
	return &AuditHandler{audit: audit}
}

func (h *AuditHandler) List(c *gin.Context) {
	if !requireOwner(c) {
		return
	}
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	filters, err := parseAuditListFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	entries, err := h.audit.List(c.Request.Context(), cursor, limit, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to list audit logs"})
		return
	}

	items := make([]gin.H, 0, len(entries))
	for i := range entries {
		items = append(items, auditEntryResponse(&entries[i]))
	}
	resp := gin.H{"items": items}
	if len(entries) == limit {
		resp["next_cursor"] = entries[len(entries)-1].ID
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuditHandler) Actions(c *gin.Context) {
	if !requireOwner(c) {
		return
	}
	actions, err := h.audit.ListDistinctActions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to list audit actions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"actions": actions})
}

func parseAuditListFilters(c *gin.Context) (repository.ListAuditFilters, error) {
	filters := repository.ListAuditFilters{
		Actions:      parseCSVQuery(c.Query("actions")),
		ActionPrefix: strings.TrimSpace(c.Query("action_prefix")),
	}

	if legacy := strings.TrimSpace(c.Query("action")); legacy != "" && len(filters.Actions) == 0 {
		filters.Actions = []string{legacy}
	}

	if actorRaw := strings.TrimSpace(c.Query("actor_id")); actorRaw != "" {
		id, err := strconv.ParseInt(actorRaw, 10, 64)
		if err != nil {
			return filters, errInvalidQuery("actor_id")
		}
		filters.ActorID = id
	}

	if actorPIDRaw := strings.TrimSpace(c.Query("actor_public_id")); actorPIDRaw != "" {
		pid, err := uuid.Parse(actorPIDRaw)
		if err != nil {
			return filters, errInvalidQuery("actor_public_id")
		}
		filters.ActorPublicID = &pid
	}

	if fromRaw := strings.TrimSpace(c.Query("from")); fromRaw != "" {
		t, err := parseAuditTime(fromRaw, false)
		if err != nil {
			return filters, errInvalidQuery("from")
		}
		filters.From = t
	}

	if toRaw := strings.TrimSpace(c.Query("to")); toRaw != "" {
		t, err := parseAuditTime(toRaw, true)
		if err != nil {
			return filters, errInvalidQuery("to")
		}
		filters.To = t
	}

	return filters, nil
}

func parseCSVQuery(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseAuditTime(raw string, endOfDay bool) (*time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			if layout == "2006-01-02" && endOfDay {
				end := t.Add(24*time.Hour - time.Nanosecond)
				return &end, nil
			}
			return &t, nil
		}
	}
	return nil, errInvalidQuery("time")
}

type invalidQueryError string

func (e invalidQueryError) Error() string {
	return "invalid " + string(e)
}

func errInvalidQuery(field string) error {
	return invalidQueryError(field)
}

func requireOwner(c *gin.Context) bool {
	u, ok := middleware.GetAuthUser(c)
	if !ok || strings.ToLower(u.Role) != "owner" {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "owner access required",
		})
		return false
	}
	return true
}

func auditEntryResponse(e *repository.AuditEntry) gin.H {
	out := gin.H{
		"id":         e.ID,
		"action":     e.Action,
		"metadata":   e.Metadata,
		"created_at": e.CreatedAt,
	}
	if e.TargetType != nil {
		out["target_type"] = *e.TargetType
	}
	if e.TargetID != nil {
		out["target_id"] = *e.TargetID
	}
	if e.IP != nil {
		out["ip"] = clientip.Normalize(*e.IP)
	}
	if e.UserAgent != nil {
		out["user_agent"] = *e.UserAgent
	}
	if e.ActorID != nil || e.ActorEmail != nil || e.ActorPublicID != nil {
		actor := gin.H{}
		if e.ActorID != nil {
			actor["id"] = *e.ActorID
		}
		if e.ActorEmail != nil {
			actor["email"] = *e.ActorEmail
		}
		if e.ActorPublicID != nil {
			actor["public_id"] = e.ActorPublicID.String()
		}
		out["actor"] = actor
	}
	return out
}
