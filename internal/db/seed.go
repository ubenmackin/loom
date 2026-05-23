package db

import (
	"context"
	"database/sql"
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

// SeedDefaults populates the prompt_templates table with built-in templates
// if the table is empty. This is called after migrations on server startup.
func SeedDefaults(ctx context.Context, db *sql.DB) error {
	// Check if any templates already exist.
	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM prompt_templates").Scan(&count)
	if err != nil {
		return fmt.Errorf("check prompt_templates count: %w", err)
	}

	if count > 0 {
		log.Printf("Templates already exist (%d), skipping seed", count)
		return nil
	}

	log.Println("No templates found, seeding default prompt templates...")

	now := time.Now().UTC()

	for _, dt := range defaultTemplateList {
		content, err := defaultTemplatesFS.ReadFile(dt.filename)
		if err != nil {
			return fmt.Errorf("read default template %s: %w", dt.filename, err)
		}

		id := uuid.New().String()
		templateText := string(content)

		_, err = db.ExecContext(ctx,
			`INSERT INTO prompt_templates (id, task_type, template, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			id, dt.taskType, templateText, now, now,
		)
		if err != nil {
			return fmt.Errorf("seed template %q: %w", dt.taskType, err)
		}

		log.Printf("Seeded default template: %s", dt.taskType)
	}

	return nil
}
