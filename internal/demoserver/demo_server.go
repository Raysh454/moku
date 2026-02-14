package demoserver

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"sync"
)

// DemoServer is a simple HTTP server for demonstrating tracking capabilities.
type DemoServer struct {
	cfg      Config
	pages    map[string]PageDefinition
	versions map[string]int // path -> current version
	mu       sync.RWMutex
}

// NewDemoServer creates a new demo server instance.
func NewDemoServer(cfg Config) *DemoServer {
	pages := GetAllPages()
	pageMap := make(map[string]PageDefinition)
	versions := make(map[string]int)

	for _, p := range pages {
		pageMap[p.Path] = p
		versions[p.Path] = cfg.InitialVersion
	}

	return &DemoServer{
		cfg:      cfg,
		pages:    pageMap,
		versions: versions,
	}
}

// Start starts the demo server.
func (s *DemoServer) Start() error {
	mux := http.NewServeMux()

	// Register page handlers
	for path := range s.pages {
		p := path // capture for closure
		mux.HandleFunc(p, s.pageHandler(p))
	}

	// Control panel for version switching
	mux.HandleFunc("/demo/control", s.controlPanelHandler)
	mux.HandleFunc("/demo/set-version", s.setVersionHandler)
	mux.HandleFunc("/demo/get-versions", s.getVersionsHandler)
	mux.HandleFunc("/demo/bump-all", s.bumpAllVersionsHandler)
	mux.HandleFunc("/demo/reset", s.resetVersionsHandler)

	// Static file placeholder
	mux.HandleFunc("/static/", s.staticHandler)

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	fmt.Printf("Demo server starting on http://localhost%s\n", addr)
	fmt.Printf("Control panel at http://localhost%s/demo/control\n", addr)
	return http.ListenAndServe(addr, mux)
}

// pageHandler returns a handler for a specific page path.
func (s *DemoServer) pageHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		pageDef, ok := s.pages[path]
		version := s.versions[path]
		s.mu.RUnlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		// Get the specific version, fall back to closest available
		pageVersion, ok := pageDef.Versions[version]
		if !ok {
			// Find the closest available version
			for v := version; v >= 1; v-- {
				if pv, exists := pageDef.Versions[v]; exists {
					pageVersion = pv
					break
				}
			}
		}

		// Set headers
		for k, v := range pageVersion.Headers {
			w.Header().Set(k, v)
		}

		// Set cookies
		for _, c := range pageVersion.Cookies {
			cookie := &http.Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Path:     c.Path,
				HttpOnly: c.HttpOnly,
				Secure:   c.Secure,
			}
			if c.SameSite != "" {
				switch c.SameSite {
				case "Strict":
					cookie.SameSite = http.SameSiteStrictMode
				case "Lax":
					cookie.SameSite = http.SameSiteLaxMode
				case "None":
					cookie.SameSite = http.SameSiteNoneMode
				}
			}
			http.SetCookie(w, cookie)
		}

		// Set content type
		contentType := pageVersion.ContentType
		if contentType == "" {
			contentType = "text/html"
		}
		w.Header().Set("Content-Type", contentType)

		// Write the page content
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(pageVersion.HTML))
	}
}

// staticHandler serves placeholder static files.
func (s *DemoServer) staticHandler(w http.ResponseWriter, r *http.Request) {
	// Return a minimal JS file for any static request
	w.Header().Set("Content-Type", "application/javascript")
	_, _ = w.Write([]byte(`// Demo static file: ` + r.URL.Path + `
console.log("Loaded: ` + r.URL.Path + `");
`))
}

// controlPanelHandler serves the control panel for version management.
func (s *DemoServer) controlPanelHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tmpl := template.Must(template.New("control").Parse(controlPanelHTML))
	data := struct {
		Pages    map[string]PageDefinition
		Versions map[string]int
		Port     int
	}{
		Pages:    s.pages,
		Versions: s.versions,
		Port:     s.cfg.Port,
	}
	w.Header().Set("Content-Type", "text/html")
	_ = tmpl.Execute(w, data)
}

// setVersionHandler sets the version for a specific page.
func (s *DemoServer) setVersionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.FormValue("path")
	versionStr := r.FormValue("version")

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if _, ok := s.pages[path]; ok {
		s.versions[path] = version
	}
	s.mu.Unlock()

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    path,
		"version": version,
	})
}

// getVersionsHandler returns the current versions of all pages.
func (s *DemoServer) getVersionsHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type PageInfo struct {
		Path              string `json:"path"`
		Description       string `json:"description"`
		CurrentVersion    int    `json:"current_version"`
		AvailableVersions []int  `json:"available_versions"`
	}

	var pages []PageInfo
	for path, pageDef := range s.pages {
		var versions []int
		for v := range pageDef.Versions {
			versions = append(versions, v)
		}
		pages = append(pages, PageInfo{
			Path:              path,
			Description:       pageDef.Description,
			CurrentVersion:    s.versions[path],
			AvailableVersions: versions,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pages)
}

// bumpAllVersionsHandler increments the version of all pages.
func (s *DemoServer) bumpAllVersionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	for path := range s.versions {
		s.versions[path]++
		// Cap at max available version
		maxV := 1
		for v := range s.pages[path].Versions {
			if v > maxV {
				maxV = v
			}
		}
		if s.versions[path] > maxV {
			s.versions[path] = maxV
		}
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "All versions bumped",
	})
}

// resetVersionsHandler resets all pages to version 1.
func (s *DemoServer) resetVersionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	for path := range s.versions {
		s.versions[path] = 1
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "All versions reset to 1",
	})
}

const controlPanelHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Demo Server Control Panel</title>
    <style>
        body { font-family: system-ui, -apple-system, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; background: #f5f5f5; }
        h1 { color: #333; border-bottom: 2px solid #007bff; padding-bottom: 10px; }
        .page-card { background: white; border-radius: 8px; padding: 20px; margin: 15px 0; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
        .page-path { font-size: 1.2em; font-weight: bold; color: #007bff; text-decoration: none; }
        .page-path:hover { text-decoration: underline; }
        .page-desc { color: #666; margin: 5px 0; }
        .version-controls { display: flex; gap: 10px; align-items: center; margin-top: 10px; }
        .version-btn { padding: 8px 16px; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; }
        .version-btn:hover { opacity: 0.9; }
        .version-btn.active { background: #007bff; color: white; }
        .version-btn.inactive { background: #e9ecef; color: #333; }
        .current-version { font-weight: bold; color: #28a745; }
        .global-controls { background: #fff3cd; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .global-controls h2 { margin-top: 0; color: #856404; }
        .global-btn { padding: 10px 20px; margin-right: 10px; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; }
        .bump-btn { background: #28a745; color: white; }
        .reset-btn { background: #dc3545; color: white; }
        .status { margin-top: 10px; padding: 10px; border-radius: 4px; display: none; }
        .status.success { background: #d4edda; color: #155724; display: block; }
        .status.error { background: #f8d7da; color: #721c24; display: block; }
        .info-box { background: #e7f3ff; padding: 15px; border-radius: 8px; margin-bottom: 20px; border-left: 4px solid #007bff; }
    </style>
</head>
<body>
    <h1>Demo Server Control Panel</h1>
    
    <div class="info-box">
        <strong>How to use:</strong> Change page versions to simulate website updates. 
        Use the Moku tracker to monitor these changes and see how it detects security-relevant modifications.
    </div>
    
    <div class="global-controls">
        <h2>Global Controls</h2>
        <button class="global-btn bump-btn" onclick="bumpAllVersions()">Bump All Versions</button>
        <button class="global-btn reset-btn" onclick="resetAllVersions()">Reset All to v1</button>
        <div id="global-status" class="status"></div>
    </div>
    
    <h2>Pages</h2>
    {{range $path, $page := .Pages}}
    <div class="page-card">
        <div class="page-header">
            <a href="{{$path}}" target="_blank" class="page-path">{{$path}}</a>
            <span class="current-version">Current: v{{index $.Versions $path}}</span>
        </div>
        <div class="page-desc">{{$page.Description}}</div>
        <div class="version-controls">
            <span>Set version:</span>
            {{range $v, $_ := $page.Versions}}
            <button class="version-btn {{if eq (index $.Versions $path) $v}}active{{else}}inactive{{end}}" 
                    onclick="setVersion('{{$path}}', {{$v}}, this)">
                v{{$v}}
            </button>
            {{end}}
        </div>
    </div>
    {{end}}
    
    <script>
        function setVersion(path, version, btn) {
            fetch('/demo/set-version', {
                method: 'POST',
                headers: {'Content-Type': 'application/x-www-form-urlencoded'},
                body: 'path=' + encodeURIComponent(path) + '&version=' + version
            })
            .then(r => r.json())
            .then(data => {
                if (data.success) {
                    // Update button states
                    const card = btn.closest('.page-card');
                    card.querySelectorAll('.version-btn').forEach(b => {
                        b.classList.remove('active');
                        b.classList.add('inactive');
                    });
                    btn.classList.remove('inactive');
                    btn.classList.add('active');
                    card.querySelector('.current-version').textContent = 'Current: v' + version;
                }
            });
        }
        
        function bumpAllVersions() {
            fetch('/demo/bump-all', {method: 'POST'})
            .then(r => r.json())
            .then(data => {
                showGlobalStatus(data.success, data.message);
                if (data.success) location.reload();
            });
        }
        
        function resetAllVersions() {
            fetch('/demo/reset', {method: 'POST'})
            .then(r => r.json())
            .then(data => {
                showGlobalStatus(data.success, data.message);
                if (data.success) location.reload();
            });
        }
        
        function showGlobalStatus(success, message) {
            const el = document.getElementById('global-status');
            el.textContent = message;
            el.className = 'status ' + (success ? 'success' : 'error');
        }
    </script>
</body>
</html>`
