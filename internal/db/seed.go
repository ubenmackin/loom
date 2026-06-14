package db

import (
	"context"
	"embed"
	"fmt"
	"log"
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
}

// SeedDefaultAgentProfiles creates the built-in agent profiles (planner,
// executor, builder, reviewer) if no agent profiles exist yet.
func SeedDefaultAgentProfiles(ctx context.Context, profileStore AgentProfileSeeder) error {
	profiles, err := profileStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list existing agent profiles: %w", err)
	}

	if len(profiles) > 0 {
		log.Printf("Agent profiles already exist (%d), skipping seed", len(profiles))
		return nil
	}

	log.Println("No agent profiles found, seeding default profiles...")

	now := time.Now().UTC()

	defaultProfiles := []*models.AgentProfile{
		{
			ID:             uuid.New().String(),
			Name:           "planner",
			Description:    "Story planning and task decomposition",
			MaxConcurrency: 2,
			TaskTypes:      []string{"planning"},
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "executor",
			Description:    "Code implementation and execution",
			MaxConcurrency: 5,
			TaskTypes:      []string{"code"},
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "builder",
			Description:    "Build verification and syntax checking",
			MaxConcurrency: 2,
			TaskTypes:      []string{"build"},
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             uuid.New().String(),
			Name:           "reviewer",
			Description:    "Code review and quality verification",
			MaxConcurrency: 3,
			TaskTypes:      []string{"review"},
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
