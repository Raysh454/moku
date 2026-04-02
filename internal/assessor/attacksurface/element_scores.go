package attacksurface

// SaturationConfig controls how element counts are capped when computing exposure scores.
// When Enabled is true, each element type count is capped at Caps[type] before scoring.
type SaturationConfig struct {
	Enabled bool               `json:"enabled"`
	Caps    map[string]float64 `json:"caps"`
}

// DefaultSaturationConfig returns a SaturationConfig with sensible defaults.
func DefaultSaturationConfig() SaturationConfig {
	return SaturationConfig{
		Enabled: true,
		Caps: map[string]float64{
			"script": 10,
			"input":  20,
			"param":  15,
			"cookie": 10,
		},
	}
}

// ElementScores maps element type keys to their default exposure score contribution.
var ElementScores = map[string]float64{
	"form":           0.10,
	"form_admin":     0.30,
	"form_auth":      0.30,
	"form_upload":    0.30,
	"input":          0.05,
	"input_file":     0.35,
	"input_password": 0.35,
	"input_hidden":   0.02,

	"cookie":             0.02,
	"cookie_no_httponly": 0.12,
	"cookie_no_secure":   0.12,
	"cookie_session":     0.10,

	"script":        0.02,
	"script_inline": 0.03,

	"param":            0.01,
	"param_suspicious": 0.08,
}

// SeverityForElement returns a coarse severity label for an element type.
func SeverityForElement(elementType string) string {
	switch elementType {
	case "form_admin", "form_auth", "form_upload",
		"input_file", "input_password",
		"cookie_session", "cookie_no_httponly", "cookie_no_secure":
		return "high"
	case "form", "script_inline", "param_suspicious":
		return "medium"
	default:
		return "low"
	}
}
