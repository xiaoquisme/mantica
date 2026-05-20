package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/xiaoquisme/mantica/server/internal/cli"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration for mantica",
	RunE:  runConfigShow,
	Args:  cobra.NoArgs,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current CLI configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a CLI configuration value",
	Long:  "Supported keys: server_url, app_url, workspace_id",
	Args:  exactArgs(2),
	RunE:  runConfigSet,
}

var configApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply agent_config.yaml to the workspace (upsert skills and agents by name)",
	RunE:  runConfigApply,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configApplyCmd)

	configApplyCmd.Flags().String("file", "agent_config.yaml", "Path to agent config YAML file")
	configApplyCmd.Flags().String("output", "table", "Output format: table or json")
}

func runConfigShow(cmd *cobra.Command, _ []string) error {
	profile := resolveProfile(cmd)
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return err
	}

	path, _ := cli.CLIConfigPathForProfile(profile)
	fmt.Fprintf(os.Stdout, "Config file: %s\n", path)
	if profile != "" {
		fmt.Fprintf(os.Stdout, "Profile:      %s\n", profile)
	}
	fmt.Fprintf(os.Stdout, "server_url:   %s\n", valueOrDefault(cfg.ServerURL, "(not set)"))
	fmt.Fprintf(os.Stdout, "app_url:      %s\n", valueOrDefault(cfg.AppURL, "(not set)"))
	fmt.Fprintf(os.Stdout, "workspace_id: %s\n", valueOrDefault(cfg.WorkspaceID, "(not set)"))
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	profile := resolveProfile(cmd)
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return err
	}

	switch key {
	case "server_url":
		cfg.ServerURL = value
	case "app_url":
		cfg.AppURL = value
	case "workspace_id":
		cfg.WorkspaceID = value
	default:
		return fmt.Errorf("unknown config key %q (supported: server_url, app_url, workspace_id)", key)
	}

	if err := cli.SaveCLIConfigForProfile(cfg, profile); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Set %s = %s\n", key, value)
	return nil
}

func valueOrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ---------------------------------------------------------------------------
// config apply
// ---------------------------------------------------------------------------

// configYAML mirrors the top-level structure of agent_config.yaml.
type configYAML struct {
	Skills []configSkillYAML `yaml:"skills"`
	Agents []configAgentYAML `yaml:"agents"`
}

type configSkillYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"content"`
}

type configAgentYAML struct {
	Name               string   `yaml:"name"`
	Provider           string   `yaml:"provider"`
	Instructions       string   `yaml:"instructions"`
	MaxConcurrentTasks int32    `yaml:"max_concurrent_tasks"`
	Visibility         string   `yaml:"visibility"`
	Skills             []string `yaml:"skills"`
}

// applyConfigRequest is the JSON body sent to POST /api/config/apply.
// Field names match the handler's ApplyConfigRequest.
type applyConfigRequest struct {
	Skills []applySkillDef `json:"skills"`
	Agents []applyAgentDef `json:"agents"`
}

type applySkillDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

type applyAgentDef struct {
	Name               string   `json:"name"`
	Provider           string   `json:"provider"`
	Instructions       string   `json:"instructions"`
	MaxConcurrentTasks int32    `json:"max_concurrent_tasks"`
	Visibility         string   `json:"visibility"`
	Skills             []string `json:"skills"`
}

type applyItemResult struct {
	Name   string `json:"name"`
	Action string `json:"action"`
}

type applyConfigResult struct {
	Skills []applyItemResult `json:"skills"`
	Agents []applyItemResult `json:"agents"`
}

func runConfigApply(cmd *cobra.Command, _ []string) error {
	filePath, _ := cmd.Flags().GetString("file")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", filePath, err)
	}

	var cfg configYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config file %q: %w", filePath, err)
	}

	// Convert YAML structs to JSON request body.
	body := applyConfigRequest{
		Skills: make([]applySkillDef, 0, len(cfg.Skills)),
		Agents: make([]applyAgentDef, 0, len(cfg.Agents)),
	}
	for _, s := range cfg.Skills {
		body.Skills = append(body.Skills, applySkillDef{
			Name:        s.Name,
			Description: s.Description,
			Content:     s.Content,
		})
	}
	for _, a := range cfg.Agents {
		body.Agents = append(body.Agents, applyAgentDef{
			Name:               a.Name,
			Provider:           a.Provider,
			Instructions:       a.Instructions,
			MaxConcurrentTasks: a.MaxConcurrentTasks,
			Visibility:         a.Visibility,
			Skills:             a.Skills,
		})
	}

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var result applyConfigResult
	if err := client.PostJSON(ctx, "/api/config/apply", body, &result); err != nil {
		return fmt.Errorf("config apply: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}

	// Table output.
	if len(result.Skills) > 0 {
		fmt.Fprintln(os.Stdout, "Skills:")
		headers := []string{"NAME", "ACTION"}
		rows := make([][]string, 0, len(result.Skills))
		for _, s := range result.Skills {
			rows = append(rows, []string{s.Name, s.Action})
		}
		cli.PrintTable(os.Stdout, headers, rows)
	} else {
		fmt.Fprintln(os.Stdout, "Skills: (none)")
	}

	fmt.Fprintln(os.Stdout)

	if len(result.Agents) > 0 {
		fmt.Fprintln(os.Stdout, "Agents:")
		headers := []string{"NAME", "ACTION"}
		rows := make([][]string, 0, len(result.Agents))
		for _, a := range result.Agents {
			rows = append(rows, []string{a.Name, a.Action})
		}
		cli.PrintTable(os.Stdout, headers, rows)
	} else {
		fmt.Fprintln(os.Stdout, "Agents: (none)")
	}

	return nil
}
