package assessor

import "time"

// AttackSurface provides a structured, security-focused representation of a page/response.
// It is linked to a snapshot and contains security-relevant attributes extracted from HTTP
// responses and HTML content.
type AttackSurface struct {
	URL        string    `json:"url"`
	SnapshotID string    `json:"snapshot_id"`
	CollectedAt time.Time `json:"collected_at"`

	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Cookies    []CookieInfo      `json:"cookies,omitempty"`

	GetParams  []Param `json:"get_params,omitempty"`
	PostParams []Param `json:"post_params,omitempty"`

	Forms []Form `json:"forms,omitempty"`

	Scripts       []ScriptInfo       `json:"scripts,omitempty"`
	EventHandlers []EventHandlerInfo `json:"event_handlers,omitempty"`

	ContentType    string   `json:"content_type,omitempty"`
	FrameworkHints []string `json:"framework_hints,omitempty"`
	ErrorIndicators []string `json:"error_indicators,omitempty"`
}

// CookieInfo represents security-relevant cookie attributes.
type CookieInfo struct {
	Name     string `json:"name"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"http_only"`
	SameSite string `json:"same_site,omitempty"`
}

// Param represents a parameter extracted from query string, form body, or form.
type Param struct {
	Name   string `json:"name"`
	Origin string `json:"origin"` // "query", "body", "form"
}

// Form represents an HTML form element.
type Form struct {
	Action string      `json:"action"`
	Method string      `json:"method"`
	Inputs []FormInput `json:"inputs,omitempty"`
}

// FormInput represents an input field within a form.
type FormInput struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// ScriptInfo represents a script tag in the page.
type ScriptInfo struct {
	Src    string `json:"src,omitempty"`
	Inline bool   `json:"inline"`
}

// EventHandlerInfo represents a DOM event handler.
type EventHandlerInfo struct {
	ElementSelector string `json:"element_selector"`
	EventType       string `json:"event_type"` // e.g. "click", "submit"
}
