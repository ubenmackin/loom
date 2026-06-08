package gateway

import (
	"sync"

	"github.com/ubenmackin/loom/internal/dispatcher"
)

// ActionType represents a gateway action that can be triggered by a
// dispatcher event and agent type combination.
type ActionType string

const (
	ActionCreateSession ActionType = "create_session"
	ActionResumeSession ActionType = "resume_session"
	ActionAssignTask    ActionType = "assign_task"
	ActionNoOp          ActionType = "noop"
)

// Rule maps a dispatcher event type and agent type combination to a
// gateway action.
type Rule struct {
	EventType string
	AgentType string
	Action    ActionType
}

// RulesEngine evaluates dispatcher events against a set of rules to
// determine the appropriate gateway action. It is safe for concurrent use.
type RulesEngine struct {
	mu    sync.RWMutex
	rules []Rule
}

// DefaultRules returns the default set of event-to-action mapping rules.
func DefaultRules() []Rule {
	return []Rule{
		{
			EventType: dispatcher.EventTaskCompleted,
			AgentType: "planner",
			Action:    ActionCreateSession,
		},
		{
			EventType: dispatcher.EventTaskCompleted,
			AgentType: "executor",
			Action:    ActionCreateSession,
		},
		{
			EventType: dispatcher.EventTaskBlocked,
			AgentType: "executor",
			Action:    ActionNoOp,
		},
		{
			EventType: dispatcher.EventWorkRequested,
			AgentType: "*",
			Action:    ActionAssignTask,
		},
		{
			EventType: dispatcher.EventSessionRegistered,
			AgentType: "*",
			Action:    ActionNoOp,
		},
		{
			EventType: dispatcher.EventPeriodicTick,
			AgentType: "*",
			Action:    ActionNoOp,
		},
		{
			EventType: dispatcher.EventTaskCompleted,
			AgentType: "reviewer",
			Action:    ActionCreateSession,
		},
		{
			EventType: dispatcher.EventTaskCompleted,
			AgentType: "builder",
			Action:    ActionCreateSession,
		},
		{
			EventType: dispatcher.EventGateTaskCreated,
			AgentType: "*",
			Action:    ActionCreateSession,
		},
	}
}

// NewRulesEngine creates a new RulesEngine pre-populated with the
// default set of rules.
func NewRulesEngine() *RulesEngine {
	return &RulesEngine{
		rules: DefaultRules(),
	}
}

// Evaluate returns the action for a given event type and agent type.
// Matching proceeds in order:
//  1. Exact match on (eventType, agentType)
//  2. Wildcard match on (eventType, "*") — any agent
//
// If no rule matches, ActionNoOp is returned. It is thread-safe.
func (re *RulesEngine) Evaluate(eventType, agentType string) ActionType {
	re.mu.RLock()
	defer re.mu.RUnlock()

	for _, r := range re.rules {
		if r.EventType == eventType && r.AgentType == agentType {
			return r.Action
		}
	}

	// Fall back to wildcard agent match.
	for _, r := range re.rules {
		if r.EventType == eventType && r.AgentType == "*" {
			return r.Action
		}
	}

	return ActionNoOp
}

// SetRules replaces all rules with the provided slice. It is thread-safe.
func (re *RulesEngine) SetRules(rules []Rule) {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.rules = make([]Rule, len(rules))
	copy(re.rules, rules)
}
