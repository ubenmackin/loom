package models

// ValidTransitions defines the allowed status transitions.
var ValidTransitions = map[Status][]Status{
	StatusNew:        {StatusReady, StatusInProgress, StatusCancelled},
	StatusDraft:      {StatusPlanning, StatusCancelled},
	StatusPlanning:   {StatusReady, StatusCancelled},
	StatusReady:      {StatusInProgress, StatusBlocked, StatusCancelled, StatusFailed},
	StatusInProgress: {StatusDone, StatusCompleted, StatusBlocked, StatusCancelled, StatusFailed},
	StatusBlocked:    {StatusReady, StatusInProgress, StatusCancelled, StatusFailed},
	StatusDone:       {StatusArchived, StatusCancelled, StatusFailed},
	StatusCompleted:  {StatusDone, StatusCancelled},
	StatusCancelled:  {StatusNew, StatusDraft},
	StatusArchived:   {},
	StatusFailed:     {StatusReady, StatusCancelled},
}

// IsValidTransition checks whether moving from current to next is allowed.
func IsValidTransition(current, next Status) bool {
	allowed, ok := ValidTransitions[current]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == next {
			return true
		}
	}
	return false
}
