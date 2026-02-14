package server

// CreateProjectRequest represents the payload required to create a project.
type CreateProjectRequest struct {
	Slug        string `json:"slug" example:"ibm"`
	Name        string `json:"name" example:"IBM"`
	Description string `json:"description" example:"Project for IBM's public website"`
}

// CreateWebsiteRequest represents the payload for creating a website within a project.
type CreateWebsiteRequest struct {
	Slug   string `json:"slug" example:"demo"`
	Origin string `json:"origin" example:"http://localhost:9999"`
}

// AddWebsiteEndpointsRequest contains URLs to add to the endpoint index.
type AddWebsiteEndpointsRequest struct {
	URLs   []string `json:"urls" example:"[\"http://localhost:9999/index\"]"`
	Source string   `json:"source" example:"manual"`
}

// AddedEndpointsResponse reports how many endpoints were inserted.
type AddedEndpointsResponse struct {
	Added int `json:"added" example:"42"`
}

// StartFetchJobRequest optionally scopes a fetch job by endpoint status and limit.
type StartFetchJobRequest struct {
	Status string `json:"status" example:"*"`
	Limit  int    `json:"limit" example:"100"`
}

// ErrorResponse is a uniform error payload returned by the API.
type ErrorResponse struct {
	Error string `json:"error" example:"not found"`
}
