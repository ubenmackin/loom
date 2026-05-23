package models

// ValidTransitions defines the allowed status transitions.
var ValidTransitions = map[Status][]Status{
	StatusNew:        {StatusReady, StatusInProgress, StatusCancelled},
	StatusReady:      {StatusInProgress, StatusBlocked, StatusCancelled},
	StatusInProgress: {StatusDone, StatusBlocked, StatusCancelled},
	StatusBlocked:    {StatusReady, StatusInProgress, StatusCancelled},
	StatusDone:       {StatusArchived, StatusCancelled},
	StatusCancelled:  {StatusNew},
	StatusArchived:   {},
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
