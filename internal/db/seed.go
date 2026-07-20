package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ubenmackin/loom/internal/models"
)

// defaultTemplatesFS embeds the default prompt template files.
//
//go:embed default-templates/*.md
var defaultTemplatesFS embed.FS

// defaultTemplate defines a prompt template to seed into the database.
type defaultTemplate struct {
	taskType models.TaskType
	filename string
}

// defaultTemplateList defines the built-in templates that are seeded on first run.
var defaultTemplateList = []defaultTemplate{
	{taskType: models.TaskTypeCode, filename: "default-templates/code.md"},
	{taskType: models.TaskTypeBuild, filename: "default-templates/build.md"},
	{taskType: models.TaskTypeReview, filename: "default-templates/review.md"},
}

// TemplateSeeder is the minimal interface needed by SeedDefaults.
// It is satisfied by *store.TemplateStore.
type TemplateSeeder interface {
	Create(ctx context.Context, t *models.PromptTemplate) error
	List(ctx context.Context) ([]*models.PromptTemplate, error)
}

// ProjectSeeder is the minimal interface needed by SeedDefaultProjects.
// It is satisfied by *store.ProjectStore.
type ProjectSeeder interface {
	Create(ctx context.Context, p *models.Project) error
	List(ctx context.Context) ([]*models.Project, error)
}

// SeedDefaultProjects creates a default "Loom" project if no projects exist.
// This is called after migrations on server startup, before SeedDefaults.
func SeedDefaultProjects(ctx context.Context, projectStore ProjectSeeder) error {
	projects, err := projectStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list existing projects: %w", err)
	}

	if len(projects) > 0 {
		log.Printf("Projects already exist (%d), skipping default project seed", len(projects))
		return nil
	}

	log.Println("No projects found, seeding default 'Loom' project...")

	now := time.Now().UTC()
	p := &models.Project{
		ID:          uuid.New().String(),
		Name:        "Loom",
		Description: "Default Loom project — agent-first JIT Kanban board",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := projectStore.Create(ctx, p); err != nil {
		return fmt.Errorf("seed default project: %w", err)
	}

	log.Printf("Seeded default project: %s (%s)", p.Name, p.ID)
	return nil
}

// AgentProfileSeeder is the minimal interface needed by SeedDefaultAgentProfiles.
// It is satisfied by the profiles store.
type AgentProfileSeeder interface {
	Create(ctx context.Context, p *models.AgentProfile) error
	List(ctx context.Context) ([]*models.AgentProfile, error)
	// Update is used by the post-migration-016 backfill pass to stamp a
	// default prompt onto pre-existing profiles whose prompt column is
	// still empty. Only rows whose name matches a known built-in default
	// profile AND whose prompt is blank are touched, so re-runs are safe.
	Update(ctx context.Context, p *models.AgentProfile) error
}

// SeedDefaultAgentProfiles creates the built-in agent profiles (planner,
// executor, builder, reviewer, security-auditor, release-manager,
// workspace-setup) if no agent profiles exist yet. It also runs a
// post-migration-016 backfill pass that stamps the default role prompts
// onto any existing profile rows whose prompt column is still empty (and
// whose name matches a known built-in default), so existing installs pick
// up the new prompt column without needing to drop and re-create their
// profiles.
func SeedDefaultAgentProfiles(ctx context.Context, profileStore AgentProfileSeeder) error {
	profiles, err := profileStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list existing agent profiles: %w", err)
	}

	now := time.Now().UTC()

	// defaultPromptsByRole maps a profile's role-key (the value stored in
	// the agent_role column — or the profile name when agent_role is
	// blank) to the canonical role-prompt const declared in
	// seed_prompts.go. These are the SOURCE OF TRUTH for the per-role
	// prompt text; seed.go is the new home for that expression (it was
	// previously hard-coded as unexported consts in
	// internal/gateway/prompts.go).
	defaultPromptsByRole := map[string]string{
		"planner":          PlannerPrompt,
		"executor":         ExecutorPrompt,
		"builder":          BuilderPrompt,
		"reviewer":         ReviewerPrompt,
		"security-auditor": SecurityAuditorPrompt,
		"release-manager":  ReleaseManagerPrompt,
		"workspace-setup":  WorkspaceSetupPrompt,
		"git-manager":      GitManagerPrompt,
	}

	// promptForProfile returns the default prompt text for the given
	// profile's role. The role key resolves to agent_role if non-empty,
	// otherwise the profile name — the same role-key resolution that
	// ProfilePrompt in internal/gateway/prompts.go applies at runtime.
	promptForProfile := func(p *models.AgentProfile) string {
		role := p.AgentRole
		if role == "" {
			role = p.Name
		}
		if prompt, ok := defaultPromptsByRole[role]; ok {
			return prompt
		}
		return DefaultPrompt
	}

	// Backfill pass: existing rows whose prompt is empty get stamped with
	// the default for their role. Idempotent — only rows with an empty
	// prompt are touched, so re-runs are safe. This runs unconditionally
	// so installs that already had profiles before migration 016 land
	// pick up the new prompt column on their next startup.
	if len(profiles) > 0 {
		backfilled := 0
		for _, p := range profiles {
			if p == nil {
				continue
			}
			if strings.TrimSpace(p.Prompt) != "" {
				continue
			}
			// Only backfill rows whose role maps to a known built-in
			// default — custom profiles inserted by the user keep an
			// empty prompt (and the gateway falls back to the static
			// DefaultPrompt via ProfilePrompt at session-build time).
			role := p.AgentRole
			if role == "" {
				role = p.Name
			}
			if _, known := defaultPromptsByRole[role]; !known {
				continue
			}
			p.Prompt = promptForProfile(p)
			if err := profileStore.Update(ctx, p); err != nil {
				return fmt.Errorf("backfill prompt for agent profile %q: %w", p.Name, err)
			}
			backfilled++
			log.Printf("Backfilled agent profile prompt: %s (%s)", p.Name, p.ID)
		}
		if backfilled > 0 {
			log.Printf("Backfilled %d agent profile prompt(s) after migration 016", backfilled)
		} else {
			log.Printf("Agent profiles already exist (%d) and have prompts, skipping seed", len(profiles))
		}
		return nil
	}

	log.Println("No agent profiles found, seeding default profiles...")

	defaultProfiles := []*models.AgentProfile{
		{
			ID:             uuid.New().String(),
			Name:           "planner",
			Description:    "Story planning and task decomposition",
			MaxConcurrency: 2,
			AgentRole:      "planner",
			TaskTypes:      []string{"planning"},
			Prompt:         PlannerPrompt,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "executor",
			Description:    "Code implementation and execution",
			MaxConcurrency: 5,
			AgentRole:      "executor",
			TaskTypes:      []string{"code"},
			Prompt:         ExecutorPrompt,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "builder",
			Description:    "Build verification and syntax checking",
			MaxConcurrency: 2,
			AgentRole:      "builder",
			TaskTypes:      []string{"build"},
			Prompt:         BuilderPrompt,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "reviewer",
			Description:    "Code review and quality verification",
			MaxConcurrency: 3,
			AgentRole:      "reviewer",
			TaskTypes:      []string{"review"},
			Prompt:         ReviewerPrompt,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "security-auditor",
			Description:    "Security audit verification and vulnerability scanning",
			MaxConcurrency: 2,
			AgentRole:      "security-auditor",
			TaskTypes:      []string{"security"},
			Prompt:         SecurityAuditorPrompt,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "release-manager",
			Description:    "Release management — commit, push, and create PRs",
			MaxConcurrency: 1,
			AgentRole:      "release-manager",
			TaskTypes:      []string{"release"},
			Prompt:         ReleaseManagerPrompt,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "workspace-setup",
			Description:    "Workspace isolation via git worktree for executor sessions",
			MaxConcurrency: 1,
			AgentRole:      "workspace-setup",
			TaskTypes:      []string{}, // No task type — ephemeral, spawned by Gateway
			Prompt:         WorkspaceSetupPrompt,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	for _, p := range defaultProfiles {
		if err := profileStore.Create(ctx, p); err != nil {
			return fmt.Errorf("seed agent profile %q: %w", p.Name, err)
		}
		log.Printf("Seeded default agent profile: %s (%s)", p.Name, p.ID)
	}

	return nil
}

// SeedDefaults populates the prompt_templates table with built-in templates
// if the table is empty. This is called after migrations on server startup.
func SeedDefaults(ctx context.Context, templateStore TemplateSeeder) error {
	// Check if any templates already exist.
	templates, err := templateStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list existing templates: %w", err)
	}

	if len(templates) > 0 {
		log.Printf("Templates already exist (%d), skipping seed", len(templates))
		return nil
	}

	log.Println("No templates found, seeding default prompt templates...")

	now := time.Now().UTC()

	for _, dt := range defaultTemplateList {
		content, err := defaultTemplatesFS.ReadFile(dt.filename)
		if err != nil {
			return fmt.Errorf("read default template %s: %w", dt.filename, err)
		}

		t := &models.PromptTemplate{
			ID:        uuid.New().String(),
			TaskType:  dt.taskType,
			Template:  string(content),
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := templateStore.Create(ctx, t); err != nil {
			return fmt.Errorf("seed template %q: %w", dt.taskType, err)
		}

		log.Printf("Seeded default template: %s", dt.taskType)
	}

	return nil
}

// StorySeeder is the minimal interface needed by SeedDefaultStory.
// It is satisfied by *store.StoryStore.
type StorySeeder interface {
	Create(ctx context.Context, story *models.Story) error
	ListAll(ctx context.Context) ([]*models.Story, error)
}

// SeedDefaultStory creates a single default story with Status "draft" if no
// stories exist. This provides a starting point for the new story lifecycle.
func SeedDefaultStory(ctx context.Context, storyStore StorySeeder) error {
	stories, err := storyStore.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list existing stories: %w", err)
	}

	if len(stories) > 0 {
		log.Printf("Stories already exist (%d), skipping default story seed", len(stories))
		return nil
	}

	log.Println("No stories found, seeding default story...")

	now := time.Now().UTC()
	s := &models.Story{
		ID:          uuid.New().String(),
		Title:       "Welcome to Loom",
		Description: "This is the first story. Use it to explore the Kanban board and understand the workflow.",
		Status:      models.Status("draft"),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := storyStore.Create(ctx, s); err != nil {
		return fmt.Errorf("seed default story: %w", err)
	}

	log.Printf("Seeded default story: %s (%s)", s.Title, s.ID)
	return nil
}

// SettingSeeder is the minimal interface needed by SeedDefaultSettings.
type SettingSeeder interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// SeedDefaultSettings creates default application settings if the settings table is empty.
func SeedDefaultSettings(ctx context.Context, settingStore SettingSeeder) error {
	// Check if settings already exist by trying to read a known key.
	if _, err := settingStore.Get(ctx, "acp_command"); err == nil {
		log.Println("Settings already exist, skipping default settings seed")
		return nil
	}

	log.Println("No settings found, seeding default settings...")

	defaults := map[string]string{
		"acp_command":            "opencode acp",
		"global_max_concurrency": "0",
	}

	for key, value := range defaults {
		if err := settingStore.Set(ctx, key, value); err != nil {
			return fmt.Errorf("seed setting %q: %w", key, err)
		}
		log.Printf("Seeded default setting: %s = %s", key, value)
	}

	return nil
}
