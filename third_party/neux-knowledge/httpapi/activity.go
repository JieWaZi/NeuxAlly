// 来源：Tencent/WeKnora 8776a2d internal/handler/audit_log.go 的知识库活动查询与游标规则。
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/JieWaZi/neux-knowledge/core"
)

// activityListResponse 保留上游成功标识、活动数组与末条游标。
type activityListResponse struct {
	// Success 表示查询是否成功。
	Success bool `json:"success"`
	// Data 是当前库本页实际持久活动。
	Data []*core.AuditLog `json:"data"`
	// NextCursor 是本页最后事件标识，空页为零。
	NextCursor uint64 `json:"next_cursor"`
}

// parseAuditCursor 沿用上游对非法游标与页大小容错的规则。
func parseAuditCursor(r *http.Request) (uint64, int) {
	var afterID uint64
	if raw := r.URL.Query().Get("after_id"); raw != "" {
		if v, err := strconv.ParseUint(raw, 10, 64); err == nil {
			afterID = v
		}
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	return afterID, limit
}

func (h *Handler) activityRoute(w http.ResponseWriter, r *http.Request, parts []string, lib string) bool {
	if len(parts) != 3 || parts[2] != "activity" || r.Method != http.MethodGet {
		return false
	}
	afterID, limit := parseAuditCursor(r)
	query := core.ActivityQuery{AfterID: afterID, Limit: limit, Action: core.AuditAction(r.URL.Query().Get("action")), Outcome: core.AuditOutcome(r.URL.Query().Get("outcome")), ActorUserID: r.URL.Query().Get("actor")}
	rows, cursor, err := h.engine.ListLibraryActivity(r.Context(), lib, query)
	respond(w, activityListResponse{Success: err == nil, Data: rows, NextCursor: cursor}, err)
	return true
}
