package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/xiaoquisme/mantica/server/pkg/db/generated"
)

// ── Skill Governance Endpoints ──

type SkillQualityInfo struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	QualityScore float64 `json:"quality_score"`
	UsageCount   int32   `json:"usage_count"`
	SuccessCount int32   `json:"success_count"`
	FailureCount int32   `json:"failure_count"`
	LastUsedAt   string  `json:"last_used_at"`
	Pinned       bool    `json:"pinned"`
	Archived     bool    `json:"archived"`
}

type GovernanceOverview struct {
	Total       int32             `json:"total"`
	Active      int32             `json:"active"`
	Archived    int32             `json:"archived"`
	Pinned      int32             `json:"pinned"`
	AvgQuality  float64           `json:"avg_quality"`
	StaleSkills []SkillQualityInfo `json:"stale_skills"`
	TopSkills   []SkillQualityInfo `json:"top_skills"`
}

// GetSkillGovernance returns an overview of skill quality and governance status.
func (h *Handler) GetSkillGovernance(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.Header.Get("X-Workspace-ID")
	wsUUID := parseUUID(workspaceID)

	// Get all skills
	allSkills, err := h.Queries.ListSkillsByWorkspace(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skills")
		return
	}

	// Get stale skills
	staleSkills, _ := h.Queries.ListStaleSkills(r.Context())

	// Get top skills
	topSkills, _ := h.Queries.ListSkillsByQuality(r.Context(), db.ListSkillsByQualityParams{
		WorkspaceID: wsUUID,
		Limit:       10,
	})

	// Compute stats
	var total, active, archived, pinned int32
	var totalQuality float64
	var activeCount int32

	for _, s := range allSkills {
		total++
		if s.ArchivedAt.Valid {
			archived++
		} else {
			active++
			totalQuality += s.QualityScore.Float64
			activeCount++
		}
		if s.Pinned.Bool {
			pinned++
		}
	}

	avgQuality := 0.0
	if activeCount > 0 {
		avgQuality = totalQuality / float64(activeCount)
	}

	// Convert stale skills
	var staleInfos []SkillQualityInfo
	for _, s := range staleSkills {
		if s.ArchivedAt.Valid {
			continue
		}
		staleInfos = append(staleInfos, SkillQualityInfo{
			ID:           uuidToString(s.ID),
			Name:         s.Name,
			QualityScore: s.QualityScore.Float64,
			UsageCount:   s.UsageCount.Int32,
			SuccessCount: s.SuccessCount.Int32,
			FailureCount: s.FailureCount.Int32,
			LastUsedAt:   formatTSPtr(s.LastUsedAt),
			Pinned:       s.Pinned.Bool,
		})
	}

	// Convert top skills
	var topInfos []SkillQualityInfo
	for _, s := range topSkills {
		topInfos = append(topInfos, SkillQualityInfo{
			ID:           uuidToString(s.ID),
			Name:         s.Name,
			QualityScore: s.QualityScore.Float64,
			UsageCount:   s.UsageCount.Int32,
			SuccessCount: s.SuccessCount.Int32,
			FailureCount: s.FailureCount.Int32,
			Pinned:       s.Pinned.Bool,
		})
	}

	if staleInfos == nil {
		staleInfos = []SkillQualityInfo{}
	}
	if topInfos == nil {
		topInfos = []SkillQualityInfo{}
	}

	writeJSON(w, http.StatusOK, GovernanceOverview{
		Total:       total,
		Active:      active,
		Archived:    archived,
		Pinned:      pinned,
		AvgQuality:  avgQuality,
		StaleSkills: staleInfos,
		TopSkills:   topInfos,
	})
}

// PinSkill marks a skill as pinned (protected from auto-archive).
func (h *Handler) PinSkill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Queries.PinSkill(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to pin skill")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "pinned"})
}

// UnpinSkill removes the pin from a skill.
func (h *Handler) UnpinSkill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Queries.UnpinSkill(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unpin skill")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unpinned"})
}

// ArchiveSkill archives a low-quality skill (only if not pinned).
func (h *Handler) ArchiveSkill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Queries.ArchiveSkill(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive skill")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// AutoArchiveStaleSkills runs the auto-archive logic for stale skills.
// Called periodically or on demand.
func (h *Handler) AutoArchiveStaleSkills(w http.ResponseWriter, r *http.Request) {
	staleSkills, err := h.Queries.ListStaleSkills(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list stale skills")
		return
	}

	archived := 0
	for _, s := range staleSkills {
		if s.Pinned.Bool || s.ArchivedAt.Valid {
			continue
		}
		if err := h.Queries.ArchiveSkill(r.Context(), s.ID); err == nil {
			archived++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"archived": archived,
	})
}

// RecordSkillUsage records that a skill was used in a task.
func (h *Handler) RecordSkillUsage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Queries.RecordSkillUsage(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record usage")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func formatTSPtr(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format(time.RFC3339)
}
