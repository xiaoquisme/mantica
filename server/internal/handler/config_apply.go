package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ApplyConfigRequest is the JSON body for POST /api/config/apply.
type ApplyConfigRequest struct {
	Skills []ApplySkillDef `json:"skills"`
	Agents []ApplyAgentDef `json:"agents"`
}

type ApplySkillDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

type ApplyAgentDef struct {
	Name               string   `json:"name"`
	Provider           string   `json:"provider"`
	Instructions       string   `json:"instructions"`
	MaxConcurrentTasks int32    `json:"max_concurrent_tasks"`
	Visibility         string   `json:"visibility"`
	Skills             []string `json:"skills"`
}

type ApplyConfigResult struct {
	Skills []ApplyItemResult `json:"skills"`
	Agents []ApplyItemResult `json:"agents"`
}

type ApplyItemResult struct {
	Name   string `json:"name"`
	Action string `json:"action"` // "created" or "updated"
}

// ApplyConfig handles POST /api/config/apply.
// It upserts skills and agents from the request body, matching by name.
// Skills: always upserted (create or update description+content).
// Agents: created if new; existing agents have instructions+max_concurrent_tasks updated.
// Agent skill assignments are replaced on every apply.
// If an agent references a provider with no runtime, the request fails immediately.
func (h *Handler) ApplyConfig(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req ApplyConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	wsUUID := parseUUID(workspaceID)
	ownerUUID := parseUUID(userID)

	result := ApplyConfigResult{
		Skills: make([]ApplyItemResult, 0),
		Agents: make([]ApplyItemResult, 0),
	}

	// --- Skills: upsert by name ---

	existingSkills, err := h.Queries.ListSkillsByWorkspace(ctx, wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skills")
		return
	}

	// Seed the name→ID map with all pre-existing skills so agents can reference
	// skills that aren't listed in this config's skills section.
	skillIDByName := make(map[string]pgtype.UUID, len(existingSkills))
	existingSkillByName := make(map[string]db.Skill, len(existingSkills))
	for _, s := range existingSkills {
		skillIDByName[s.Name] = s.ID
		existingSkillByName[s.Name] = s
	}

	for _, def := range req.Skills {
		if def.Name == "" {
			writeError(w, http.StatusBadRequest, "skill name is required")
			return
		}
		if existing, found := existingSkillByName[def.Name]; found {
			updated, err := h.Queries.UpdateSkill(ctx, db.UpdateSkillParams{
				ID:          existing.ID,
				Name:        pgtype.Text{String: def.Name, Valid: true},
				Description: pgtype.Text{String: def.Description, Valid: true},
				Content:     pgtype.Text{String: def.Content, Valid: true},
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update skill %q: %s", def.Name, err))
				return
			}
			skillIDByName[def.Name] = updated.ID
			result.Skills = append(result.Skills, ApplyItemResult{Name: def.Name, Action: "updated"})
			slog.Info("config apply: skill updated", "skill", def.Name)
		} else {
			created, err := h.Queries.CreateSkill(ctx, db.CreateSkillParams{
				WorkspaceID: wsUUID,
				Name:        def.Name,
				Description: def.Description,
				Content:     def.Content,
				Config:      []byte("{}"),
				CreatedBy:   ownerUUID,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create skill %q: %s", def.Name, err))
				return
			}
			skillIDByName[def.Name] = created.ID
			result.Skills = append(result.Skills, ApplyItemResult{Name: def.Name, Action: "created"})
			slog.Info("config apply: skill created", "skill", def.Name)
		}
	}

	// --- Runtimes: build provider → runtime map ---
	// Prefer online runtimes; fall back to any runtime for that provider.
	runtimes, err := h.Queries.ListAgentRuntimes(ctx, wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runtimes")
		return
	}
	runtimeByProvider := make(map[string]db.AgentRuntime)
	for _, rt := range runtimes {
		prev, already := runtimeByProvider[rt.Provider]
		if !already || (rt.Status == "online" && prev.Status != "online") {
			runtimeByProvider[rt.Provider] = rt
		}
	}

	// --- Agents: upsert by name ---

	existingAgents, err := h.Queries.ListAgents(ctx, wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}
	agentByName := make(map[string]db.Agent, len(existingAgents))
	for _, a := range existingAgents {
		agentByName[a.Name] = a
	}

	for _, def := range req.Agents {
		if def.Name == "" {
			writeError(w, http.StatusBadRequest, "agent name is required")
			return
		}
		if def.Provider == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("provider is required for agent %q", def.Name))
			return
		}

		rt, found := runtimeByProvider[def.Provider]
		if !found {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("no runtime found for provider %q (required by agent %q)", def.Provider, def.Name))
			return
		}

		visibility := def.Visibility
		if visibility == "" {
			visibility = "public"
		}
		maxTasks := def.MaxConcurrentTasks
		if maxTasks == 0 {
			maxTasks = 6
		}

		var agentID pgtype.UUID

		if existing, found := agentByName[def.Name]; found {
			// Update instructions and max_concurrent_tasks only; leave runtime/visibility untouched.
			updated, err := h.Queries.UpdateAgent(ctx, db.UpdateAgentParams{
				ID:                 existing.ID,
				Instructions:       pgtype.Text{String: def.Instructions, Valid: true},
				MaxConcurrentTasks: pgtype.Int4{Int32: maxTasks, Valid: true},
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError,
					fmt.Sprintf("failed to update agent %q: %s", def.Name, err))
				return
			}
			agentID = updated.ID
			result.Agents = append(result.Agents, ApplyItemResult{Name: def.Name, Action: "updated"})
			slog.Info("config apply: agent updated", "agent", def.Name)
		} else {
			created, err := h.Queries.CreateAgent(ctx, db.CreateAgentParams{
				WorkspaceID:        wsUUID,
				Name:               def.Name,
				Description:        "",
				Instructions:       def.Instructions,
				AvatarUrl:          pgtype.Text{},
				RuntimeMode:        rt.RuntimeMode,
				RuntimeConfig:      []byte("{}"),
				RuntimeID:          rt.ID,
				Visibility:         visibility,
				MaxConcurrentTasks: maxTasks,
				OwnerID:            ownerUUID,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError,
					fmt.Sprintf("failed to create agent %q: %s", def.Name, err))
				return
			}
			agentID = created.ID
			result.Agents = append(result.Agents, ApplyItemResult{Name: def.Name, Action: "created"})
			slog.Info("config apply: agent created", "agent", def.Name, "provider", def.Provider)
		}

		// Sync skill assignments: replace all existing assignments.
		if err := h.Queries.RemoveAllAgentSkills(ctx, agentID); err != nil {
			writeError(w, http.StatusInternalServerError,
				fmt.Sprintf("failed to clear skills for agent %q", def.Name))
			return
		}
		for _, skillName := range def.Skills {
			skillID, ok := skillIDByName[skillName]
			if !ok {
				writeError(w, http.StatusBadRequest,
					fmt.Sprintf("skill %q referenced by agent %q not found in workspace", skillName, def.Name))
				return
			}
			if err := h.Queries.AddAgentSkill(ctx, db.AddAgentSkillParams{
				AgentID: agentID,
				SkillID: skillID,
			}); err != nil {
				writeError(w, http.StatusInternalServerError,
					fmt.Sprintf("failed to attach skill %q to agent %q: %s", skillName, def.Name, err))
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, result)
}
