// Package seed handles workspace seed data — default agents and skills created
// when a runtime is first registered for a workspace.
package seed

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"gopkg.in/yaml.v3"
)

// skillDef mirrors the YAML skill definition.
type skillDef struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"content"`
}

// agentDef mirrors the YAML agent definition.
type agentDef struct {
	Name               string   `yaml:"name"`
	Provider           string   `yaml:"provider"`
	Instructions       string   `yaml:"instructions"`
	MaxConcurrentTasks int32    `yaml:"max_concurrent_tasks"`
	Visibility         string   `yaml:"visibility"`
	Skills             []string `yaml:"skills"`
}

type seedData struct {
	Skills []skillDef `yaml:"skills"`
	Agents []agentDef `yaml:"agents"`
}

func parseSeedData(data []byte) (*seedData, error) {
	var sd seedData
	if err := yaml.Unmarshal(data, &sd); err != nil {
		return nil, fmt.Errorf("parse agent config yaml: %w", err)
	}
	return &sd, nil
}

// Queries is the subset of db.Queries used by the seed package.
type Queries interface {
	ListAgents(ctx context.Context, workspaceID pgtype.UUID) ([]db.Agent, error)
	CreateAgent(ctx context.Context, arg db.CreateAgentParams) (db.Agent, error)
	ListSkillsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.Skill, error)
	CreateSkill(ctx context.Context, arg db.CreateSkillParams) (db.Skill, error)
	AddAgentSkill(ctx context.Context, arg db.AddAgentSkillParams) error
}

// WorkspaceIfEmpty seeds default skills and agents for a workspace if it has
// no agents yet. configData is the raw content of agent_config.yaml.
// It is idempotent: skills are matched by name (skipped if they exist) and
// agents are only created when the workspace has zero agents.
//
// runtimesByProvider maps provider name → runtime ID for runtimes that were
// just registered. Only agents whose provider appears in this map are created.
func WorkspaceIfEmpty(
	ctx context.Context,
	q Queries,
	workspaceID pgtype.UUID,
	ownerID pgtype.UUID,
	runtimesByProvider map[string]pgtype.UUID,
	configData []byte,
) error {
	if len(configData) == 0 {
		slog.Debug("seed: no agent config data, skipping")
		return nil
	}

	data, err := parseSeedData(configData)
	if err != nil {
		return err
	}

	// Check if workspace already has agents — if so, skip.
	existing, err := q.ListAgents(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}
	if len(existing) > 0 {
		slog.Debug("seed: workspace already has agents, skipping", "workspace_id", workspaceID)
		return nil
	}

	// Ensure all skills exist, building a name→ID map.
	skillIDByName, err := ensureSkills(ctx, q, workspaceID, ownerID, data.Skills)
	if err != nil {
		return err
	}

	// Create agents whose provider has an online runtime.
	for _, ag := range data.Agents {
		runtimeID, ok := runtimesByProvider[ag.Provider]
		if !ok {
			slog.Debug("seed: no runtime for provider, skipping agent",
				"agent", ag.Name, "provider", ag.Provider)
			continue
		}

		visibility := ag.Visibility
		if visibility == "" {
			visibility = "public"
		}
		maxTasks := ag.MaxConcurrentTasks
		if maxTasks == 0 {
			maxTasks = 6
		}

		created, err := q.CreateAgent(ctx, db.CreateAgentParams{
			WorkspaceID:        workspaceID,
			Name:               ag.Name,
			Description:        "",
			Instructions:       ag.Instructions,
			AvatarUrl:          pgtype.Text{},
			RuntimeMode:        "local",
			RuntimeConfig:      []byte("{}"),
			RuntimeID:          runtimeID,
			Visibility:         visibility,
			MaxConcurrentTasks: maxTasks,
			OwnerID:            ownerID,
		})
		if err != nil {
			return fmt.Errorf("create agent %q: %w", ag.Name, err)
		}
		slog.Info("seed: agent created", "agent", created.Name, "provider", ag.Provider)

		// Attach skills.
		for _, skillName := range ag.Skills {
			skillID, ok := skillIDByName[skillName]
			if !ok {
				slog.Warn("seed: skill not found for agent", "skill", skillName, "agent", ag.Name)
				continue
			}
			if err := q.AddAgentSkill(ctx, db.AddAgentSkillParams{
				AgentID: created.ID,
				SkillID: skillID,
			}); err != nil {
				return fmt.Errorf("attach skill %q to agent %q: %w", skillName, ag.Name, err)
			}
		}
	}

	return nil
}

// ensureSkills upserts skills by name, returning a name→UUID map.
func ensureSkills(
	ctx context.Context,
	q Queries,
	workspaceID pgtype.UUID,
	ownerID pgtype.UUID,
	defs []skillDef,
) (map[string]pgtype.UUID, error) {
	// Load existing skills once.
	existing, err := q.ListSkillsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	byName := make(map[string]pgtype.UUID, len(existing))
	for _, s := range existing {
		byName[s.Name] = s.ID
	}

	for _, def := range defs {
		if _, ok := byName[def.Name]; ok {
			slog.Debug("seed: skill already exists, skipping", "skill", def.Name)
			continue
		}
		created, err := q.CreateSkill(ctx, db.CreateSkillParams{
			WorkspaceID: workspaceID,
			Name:        def.Name,
			Description: def.Description,
			Content:     def.Content,
			Config:      []byte("{}"),
			CreatedBy:   ownerID,
		})
		if err != nil {
			return nil, fmt.Errorf("create skill %q: %w", def.Name, err)
		}
		byName[created.Name] = created.ID
		slog.Info("seed: skill created", "skill", created.Name)
	}

	return byName, nil
}
