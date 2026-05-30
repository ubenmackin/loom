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
