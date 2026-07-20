package gateway

import (
	"strings"

	"github.com/ubenmackin/loom/internal/db"
	"github.com/ubenmackin/loom/internal/models"
)

// ProfilePrompt returns the Loom-defined system prompt for the given agent
// profile. It is the production source of the prepended system-prompt text
// sent to ACP sessions by Gateway.buildACPContext (see internal/gateway/loop.go).
//
// Resolution order:
//  1. If profile is non-nil and its prompt column (added in migration 016) is
//     non-empty, that text wins — profiles with a custom prompt opt out of the
//     static defaults entirely.
//  2. Otherwise fall back to the static role-keyed default declared in
//     internal/db/seed_prompts.go (PlannerPrompt, ExecutorPrompt, etc.). The
//     role key resolves to profile.AgentRole if non-empty, otherwise to
//     profile.Name — the same resolution the DB seeder applies when
//     stamping the default prompt onto the agent_profiles row.
//  3. If the role is not one of the 8 known built-in roles, db.DefaultPrompt
//     (a generic one-liner) is returned.
//
// Keeping the const text in internal/db (rather than in this package) means the
// DB seeder can stamp the default prompts onto the agent_profiles.prompt
// column without importing the heavyweight gateway package, and the gateway
// can fall back to the same consts at runtime — single source of truth.
func ProfilePrompt(profile *models.AgentProfile) string {
	if profile != nil && strings.TrimSpace(profile.Prompt) != "" {
		return profile.Prompt
	}

	role := ""
	if profile != nil {
		role = profile.AgentRole
		if role == "" {
			role = profile.Name
		}
	}

	switch role {
	case "planner":
		return db.PlannerPrompt
	case "executor":
		return db.ExecutorPrompt
	case "builder":
		return db.BuilderPrompt
	case "reviewer":
		return db.ReviewerPrompt
	case "security-auditor":
		return db.SecurityAuditorPrompt
	case "release-manager":
		return db.ReleaseManagerPrompt
	case "workspace-setup":
		return db.WorkspaceSetupPrompt
	case "git-manager":
		return db.GitManagerPrompt
	default:
		return db.DefaultPrompt
	}
}
