package gateway

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Job represents a queued work item waiting to be assigned to a session.
type Job struct {
	ID        string // unique job ID (UUID)
	ProjectID string
	AgentType string
	TaskID    string
	EventRef  string // reference to the event that created this job
	CreatedAt time.Time
}

// JobQueue manages FIFO queues per (projectID, agentType) pair.
// It enforces concurrency limits — when a session slot is free,
// the next matching job can be dequeued.
type JobQueue struct {
	mu          sync.RWMutex
	queues      map[string][]*Job // key: "projectID:agentType" -> FIFO queue
	concurrency map[string]int    // key: "projectID:agentType" -> max concurrency
	active      map[string]int    // key: "projectID:agentType" -> current active count
}

// DefaultConcurrency returns the default maximum concurrent sessions per
// agent type.
func DefaultConcurrency() map[string]int {
	return map[string]int{
		"executor": 2,
		"planner":  1,
		"builder":  1,
		"reviewer": 1,
	}
}

// queueKey builds the composite map key for a (projectID, agentType) pair.
func queueKey(projectID, agentType string) string {
	return fmt.Sprintf("%s:%s", projectID, agentType)
}

// NewJobQueue creates a new JobQueue pre-populated with the default
// concurrency limits.
func NewJobQueue() *JobQueue {
	return &JobQueue{
		queues:      make(map[string][]*Job),
		concurrency: DefaultConcurrency(),
		active:      make(map[string]int),
	}
}

// SetConcurrency sets the maximum number of concurrent sessions for a given
// agent type. This applies across all projects.
func (jq *JobQueue) SetConcurrency(agentType string, max int) {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	jq.concurrency[agentType] = max
}

// Enqueue creates a new Job with a unique UUID, appends it to the FIFO
// queue for the given (projectID, agentType) pair, and returns the job.
func (jq *JobQueue) Enqueue(projectID, agentType, taskID, eventRef string) *Job {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	job := &Job{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		AgentType: agentType,
		TaskID:    taskID,
		EventRef:  eventRef,
		CreatedAt: time.Now().UTC(),
	}

	k := queueKey(projectID, agentType)
	jq.queues[k] = append(jq.queues[k], job)

	return job
}

// Dequeue pops and returns the front of the FIFO queue for the given
// (projectID, agentType) pair. Returns nil if the queue is empty.
func (jq *JobQueue) Dequeue(projectID, agentType string) *Job {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	k := queueKey(projectID, agentType)
	queue := jq.queues[k]
	if len(queue) == 0 {
		return nil
	}

	job := queue[0]
	jq.queues[k] = queue[1:]
	return job
}

// TotalLen returns the total number of jobs across all queues.
func (jq *JobQueue) TotalLen() int {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	total := 0
	for _, q := range jq.queues {
		total += len(q)
	}
	return total
}

// IncrementActive increments the active session count for the given
// (projectID, agentType) pair.
func (jq *JobQueue) IncrementActive(projectID, agentType string) {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	k := queueKey(projectID, agentType)
	jq.active[k]++
}

// DecrementActive decrements the active session count for the given
// (projectID, agentType) pair. It never goes below zero.
func (jq *JobQueue) DecrementActive(projectID, agentType string) {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	k := queueKey(projectID, agentType)
	if jq.active[k] > 0 {
		jq.active[k]--
	}
}

// HasCapacity returns true if the number of active sessions for the given
// (agentType, projectID) pair is less than the configured concurrency limit.
func (jq *JobQueue) HasCapacity(projectID, agentType string) bool {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	k := queueKey(projectID, agentType)
	max, ok := jq.concurrency[agentType]
	if !ok {
		max = 1
	}
	return jq.active[k] < max
}

// ListAll returns all jobs across all queues. The returned slice is a copy
// and is safe to modify.
func (jq *JobQueue) ListAll() []*Job {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	var result []*Job
	for _, q := range jq.queues {
		result = append(result, q...)
	}
	return result
}

// Remove removes a job by its TaskID from any queue. Returns true if a
// job was found and removed.
func (jq *JobQueue) Remove(taskID string) bool {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	for k, q := range jq.queues {
		for i, job := range q {
			if job.TaskID == taskID {
				jq.queues[k] = append(q[:i], q[i+1:]...)
				return true
			}
		}
	}
	return false
}
