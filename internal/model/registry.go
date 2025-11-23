package model

// Project is a logical grouping of websites.
type Project struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	Meta        string `json:"meta,omitempty"`
}

// Website is a single website belonging to a project.
type Website struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Slug        string `json:"slug"`
	Name        string `json:"name,omitempty"`
	Origin      string `json:"origin"`
	StoragePath string `json:"storage_path"`
	CreatedAt   int64  `json:"created_at"`
	LastSeenAt  int64  `json:"last_seen_at,omitempty"`
	Config      string `json:"config,omitempty"`
}
