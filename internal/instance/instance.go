package instance

import "sync"

var (
	once sync.Once
	id   string
)

// Set stores the instance project ID. Call once at startup.
func Set(projectID string) {
	once.Do(func() { id = projectID })
}

// ID returns the single instance project ID.
func ID() string { return id }
