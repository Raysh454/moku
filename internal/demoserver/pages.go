package demoserver

// PageVersion represents a specific version of a page with its HTML content and headers.
type PageVersion struct {
	HTML        string
	ContentType string
	Headers     map[string]string
	Cookies     []CookieDef
}

// CookieDef defines a cookie to be set.
type CookieDef struct {
	Name     string
	Value    string
	Path     string
	HttpOnly bool
	Secure   bool
	SameSite string // "Strict", "Lax", "None", or ""
}

// PageDefinition holds all versions of a single page.
type PageDefinition struct {
	Path        string
	Description string
	Versions    map[int]PageVersion
}

// GetAllPages returns all demo page definitions.
func GetAllPages() []PageDefinition {
	return []PageDefinition{
		getHomePage(),
		getLoginPage(),
		getAdminPage(),
		getUploadPage(),
		getContactPage(),
		getProfilePage(),
	}
}

// ===== HOME PAGE =====
func getHomePage() PageDefinition {
	return PageDefinition{
		Path:        "/",
		Description: "Home page with basic navigation",
		Versions: map[int]PageVersion{
			1: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Demo Site - Home v1</title>
    <script src="/static/app.js"></script>
</head>
<body>
    <h1>Welcome to Demo Site</h1>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <p>Version 1 - Basic home page</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options": "DENY",
				},
				Cookies: []CookieDef{
					{Name: "visitor", Value: "true", Path: "/", HttpOnly: false, Secure: false},
				},
			},
			2: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Demo Site - Home v2</title>
    <script src="/static/app.js"></script>
    <script src="/static/analytics.js"></script>
</head>
<body>
    <h1>Welcome to Demo Site</h1>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <p>Version 2 - Added admin and upload links</p>
    <form action="/search" method="GET" id="search-form">
        <input type="text" name="q" placeholder="Search...">
        <button type="submit">Search</button>
    </form>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options":        "DENY",
					"X-Content-Type-Options": "nosniff",
				},
				Cookies: []CookieDef{
					{Name: "visitor", Value: "true", Path: "/", HttpOnly: false, Secure: false},
					{Name: "tracking_id", Value: "abc123", Path: "/", HttpOnly: false, Secure: false},
				},
			},
			3: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Demo Site - Home v3</title>
    <script src="/static/app.js"></script>
    <script src="/static/analytics.js"></script>
    <script>
        // Inline script for quick actions
        function quickSearch() {
            document.getElementById('search-form').submit();
        }
    </script>
</head>
<body>
    <h1>Welcome to Demo Site</h1>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <p>Version 3 - Added inline script and profile link</p>
    <form action="/search" method="GET" id="search-form" class="search-box">
        <input type="text" name="q" placeholder="Search..." required>
        <input type="hidden" name="source" value="homepage">
        <button type="submit" onclick="quickSearch()">Search</button>
    </form>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options":           "SAMEORIGIN",
					"X-Content-Type-Options":    "nosniff",
					"Content-Security-Policy":   "default-src 'self'; script-src 'self' 'unsafe-inline'",
					"Strict-Transport-Security": "max-age=31536000",
				},
				Cookies: []CookieDef{
					{Name: "visitor", Value: "true", Path: "/", HttpOnly: true, Secure: true, SameSite: "Lax"},
					{Name: "session_id", Value: "sess_xyz789", Path: "/", HttpOnly: true, Secure: true, SameSite: "Strict"},
				},
			},
		},
	}
}

// ===== LOGIN PAGE =====
func getLoginPage() PageDefinition {
	return PageDefinition{
		Path:        "/login",
		Description: "Login page with authentication form",
		Versions: map[int]PageVersion{
			1: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Login v1</title>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Login</h1>
    <form action="/auth/login" method="POST" id="login-form">
        <label>Username: <input type="text" name="username"></label><br>
        <label>Password: <input type="password" name="password"></label><br>
        <button type="submit">Login</button>
    </form>
    <p>Version 1 - Basic login form (no CSRF)</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers:     map[string]string{},
				Cookies:     []CookieDef{},
			},
			2: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Login v2</title>
    <script src="/static/validation.js"></script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Login</h1>
    <form action="/auth/login" method="POST" id="login-form" class="auth-form">
        <input type="hidden" name="csrf_token" value="token_abc123">
        <label>Username: <input type="text" name="username" required></label><br>
        <label>Password: <input type="password" name="password" required></label><br>
        <label><input type="checkbox" name="remember"> Remember me</label><br>
        <button type="submit">Login</button>
    </form>
    <p><a href="/forgot-password">Forgot password?</a></p>
    <p>Version 2 - Added CSRF token and remember me</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options": "DENY",
				},
				Cookies: []CookieDef{
					{Name: "csrf", Value: "token_abc123", Path: "/", HttpOnly: true, Secure: false},
				},
			},
			3: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Login v3</title>
    <script src="/static/validation.js"></script>
    <script src="/static/captcha.js"></script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Secure Login</h1>
    <form action="/auth/signin" method="POST" id="login-form" class="auth-form secure-form">
        <input type="hidden" name="csrf_token" value="token_xyz789">
        <input type="hidden" name="form_id" value="login_v3">
        <label>Email: <input type="email" name="email" required></label><br>
        <label>Password: <input type="password" name="password" required minlength="8"></label><br>
        <label><input type="checkbox" name="remember"> Remember me</label><br>
        <div class="captcha-container">
            <input type="text" name="captcha" placeholder="Enter CAPTCHA" required>
        </div>
        <button type="submit">Sign In</button>
    </form>
    <p><a href="/forgot-password">Forgot password?</a> | <a href="/register">Register</a></p>
    <p>Version 3 - Email login, captcha, enhanced security</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options":           "DENY",
					"X-Content-Type-Options":    "nosniff",
					"Content-Security-Policy":   "default-src 'self'; form-action 'self'",
					"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
					"Referrer-Policy":           "strict-origin-when-cross-origin",
				},
				Cookies: []CookieDef{
					{Name: "csrf", Value: "token_xyz789", Path: "/", HttpOnly: true, Secure: true, SameSite: "Strict"},
					{Name: "session_id", Value: "sess_secure", Path: "/", HttpOnly: true, Secure: true, SameSite: "Strict"},
				},
			},
		},
	}
}

// ===== ADMIN PAGE =====
func getAdminPage() PageDefinition {
	return PageDefinition{
		Path:        "/admin",
		Description: "Admin panel with sensitive forms",
		Versions: map[int]PageVersion{
			1: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Admin Panel v1</title>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Admin Panel</h1>
    <form action="/admin/settings" method="POST" id="settings-form">
        <h2>Site Settings</h2>
        <label>Site Name: <input type="text" name="site_name"></label><br>
        <label>Admin Email: <input type="email" name="admin_email"></label><br>
        <button type="submit">Save</button>
    </form>
    <p>Version 1 - Basic admin form</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers:     map[string]string{},
				Cookies:     []CookieDef{},
			},
			2: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Admin Panel v2</title>
    <script src="/static/admin.js"></script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Admin Panel</h1>
    <form action="/admin/settings" method="POST" id="settings-form">
        <input type="hidden" name="csrf_token" value="admin_csrf_123">
        <h2>Site Settings</h2>
        <label>Site Name: <input type="text" name="site_name" required></label><br>
        <label>Admin Email: <input type="email" name="admin_email" required></label><br>
        <button type="submit">Save</button>
    </form>
    
    <form action="/admin/users" method="POST" id="user-form">
        <input type="hidden" name="csrf_token" value="admin_csrf_123">
        <h2>User Management</h2>
        <label>Username: <input type="text" name="username"></label><br>
        <label>Role: 
            <select name="role">
                <option value="user">User</option>
                <option value="admin">Admin</option>
            </select>
        </label><br>
        <button type="submit">Add User</button>
    </form>
    <p>Version 2 - Added user management form</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options":        "DENY",
					"X-Content-Type-Options": "nosniff",
				},
				Cookies: []CookieDef{
					{Name: "admin_session", Value: "admin_sess_abc", Path: "/admin", HttpOnly: true, Secure: false},
				},
			},
			3: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Admin Panel v3</title>
    <script src="/static/admin.js"></script>
    <script src="/static/audit.js"></script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Admin Control Panel</h1>
    <form action="/admin/config" method="POST" id="config-form" class="admin-form">
        <input type="hidden" name="csrf_token" value="admin_csrf_xyz">
        <input type="hidden" name="form_version" value="3">
        <h2>System Configuration</h2>
        <label>Site Name: <input type="text" name="site_name" required></label><br>
        <label>Admin Email: <input type="email" name="admin_email" required></label><br>
        <label>Debug Mode: <input type="checkbox" name="debug_mode"></label><br>
        <label>Maintenance: <input type="checkbox" name="maintenance_mode"></label><br>
        <button type="submit">Save Configuration</button>
    </form>
    
    <form action="/admin/users/create" method="POST" id="user-form" class="admin-form">
        <input type="hidden" name="csrf_token" value="admin_csrf_xyz">
        <h2>Create User</h2>
        <label>Username: <input type="text" name="username" required></label><br>
        <label>Email: <input type="email" name="email" required></label><br>
        <label>Password: <input type="password" name="password" required></label><br>
        <label>Role: 
            <select name="role">
                <option value="viewer">Viewer</option>
                <option value="editor">Editor</option>
                <option value="admin">Admin</option>
                <option value="superadmin">Super Admin</option>
            </select>
        </label><br>
        <button type="submit">Create User</button>
    </form>
    
    <form action="/admin/database" method="POST" id="db-form" class="admin-form danger">
        <input type="hidden" name="csrf_token" value="admin_csrf_xyz">
        <h2>Database Operations</h2>
        <label>SQL Query: <textarea name="sql_query" rows="4" cols="50"></textarea></label><br>
        <button type="submit">Execute</button>
    </form>
    <p>Version 3 - Full admin panel with database access</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options":           "DENY",
					"X-Content-Type-Options":    "nosniff",
					"Content-Security-Policy":   "default-src 'self'; form-action 'self'",
					"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
					"Referrer-Policy":           "no-referrer",
					"X-XSS-Protection":          "1; mode=block",
				},
				Cookies: []CookieDef{
					{Name: "admin_session", Value: "admin_sess_secure", Path: "/admin", HttpOnly: true, Secure: true, SameSite: "Strict"},
					{Name: "admin_csrf", Value: "admin_csrf_xyz", Path: "/admin", HttpOnly: true, Secure: true, SameSite: "Strict"},
				},
			},
		},
	}
}

// ===== UPLOAD PAGE =====
func getUploadPage() PageDefinition {
	return PageDefinition{
		Path:        "/upload",
		Description: "File upload page",
		Versions: map[int]PageVersion{
			1: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Upload v1</title>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>File Upload</h1>
    <form action="/files/upload" method="POST" enctype="multipart/form-data" id="upload-form">
        <label>Select file: <input type="file" name="file"></label><br>
        <button type="submit">Upload</button>
    </form>
    <p>Version 1 - Basic file upload</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers:     map[string]string{},
				Cookies:     []CookieDef{},
			},
			2: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Upload v2</title>
    <script src="/static/upload.js"></script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>File Upload</h1>
    <form action="/files/upload" method="POST" enctype="multipart/form-data" id="upload-form">
        <input type="hidden" name="csrf_token" value="upload_csrf_123">
        <label>Select file: <input type="file" name="file" required accept=".jpg,.png,.pdf"></label><br>
        <label>Description: <input type="text" name="description"></label><br>
        <button type="submit">Upload</button>
    </form>
    <p>Version 2 - Added CSRF and file type restrictions</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options": "DENY",
				},
				Cookies: []CookieDef{
					{Name: "upload_session", Value: "upload_sess", Path: "/upload", HttpOnly: true, Secure: false},
				},
			},
			3: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Upload v3</title>
    <script src="/static/upload.js"></script>
    <script src="/static/validation.js"></script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Secure File Upload</h1>
    <form action="/api/v2/files/upload" method="POST" enctype="multipart/form-data" id="upload-form" class="upload-form">
        <input type="hidden" name="csrf_token" value="upload_csrf_xyz">
        <input type="hidden" name="max_size" value="10485760">
        <label>Select files: <input type="file" name="files" required accept=".jpg,.png,.gif,.pdf,.doc,.docx" multiple></label><br>
        <label>Category: 
            <select name="category">
                <option value="documents">Documents</option>
                <option value="images">Images</option>
                <option value="other">Other</option>
            </select>
        </label><br>
        <label>Description: <textarea name="description" rows="3" cols="40"></textarea></label><br>
        <label><input type="checkbox" name="public" value="1"> Make public</label><br>
        <button type="submit">Upload Files</button>
    </form>
    <p>Version 3 - Multi-file upload with categories</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options":           "DENY",
					"X-Content-Type-Options":    "nosniff",
					"Content-Security-Policy":   "default-src 'self'",
					"Strict-Transport-Security": "max-age=31536000",
				},
				Cookies: []CookieDef{
					{Name: "upload_session", Value: "upload_sess_secure", Path: "/", HttpOnly: true, Secure: true, SameSite: "Lax"},
				},
			},
		},
	}
}

// ===== CONTACT PAGE =====
func getContactPage() PageDefinition {
	return PageDefinition{
		Path:        "/contact",
		Description: "Contact form page",
		Versions: map[int]PageVersion{
			1: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Contact v1</title>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Contact Us</h1>
    <form action="/contact/send" method="POST" id="contact-form">
        <label>Name: <input type="text" name="name"></label><br>
        <label>Email: <input type="email" name="email"></label><br>
        <label>Message: <textarea name="message" rows="5" cols="40"></textarea></label><br>
        <button type="submit">Send</button>
    </form>
    <p>Version 1 - Basic contact form</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers:     map[string]string{},
				Cookies:     []CookieDef{},
			},
			2: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Contact v2</title>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Contact Us</h1>
    <form action="/contact/submit" method="POST" id="contact-form">
        <input type="hidden" name="csrf_token" value="contact_csrf">
        <label>Name: <input type="text" name="name" required></label><br>
        <label>Email: <input type="email" name="email" required></label><br>
        <label>Subject: <input type="text" name="subject"></label><br>
        <label>Message: <textarea name="message" rows="5" cols="40" required></textarea></label><br>
        <button type="submit">Send Message</button>
    </form>
    <p>Version 2 - Added subject field and CSRF</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options": "SAMEORIGIN",
				},
				Cookies: []CookieDef{},
			},
			3: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Contact v3</title>
    <script src="/static/validation.js"></script>
    <script>
        function validateForm() {
            var email = document.getElementById('email').value;
            return email.includes('@');
        }
    </script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>Get In Touch</h1>
    <form action="/api/contact" method="POST" id="contact-form" class="contact-form" onsubmit="return validateForm()">
        <input type="hidden" name="csrf_token" value="contact_csrf_v3">
        <input type="hidden" name="honeypot" value="">
        <label>Full Name: <input type="text" name="full_name" required></label><br>
        <label>Email: <input type="email" name="email" id="email" required></label><br>
        <label>Phone: <input type="tel" name="phone"></label><br>
        <label>Subject: 
            <select name="subject">
                <option value="general">General Inquiry</option>
                <option value="support">Technical Support</option>
                <option value="sales">Sales</option>
                <option value="feedback">Feedback</option>
            </select>
        </label><br>
        <label>Priority: 
            <input type="radio" name="priority" value="low"> Low
            <input type="radio" name="priority" value="medium" checked> Medium
            <input type="radio" name="priority" value="high"> High
        </label><br>
        <label>Message: <textarea name="message" rows="5" cols="40" required></textarea></label><br>
        <label><input type="checkbox" name="subscribe" value="1"> Subscribe to newsletter</label><br>
        <label>Attachment: <input type="file" name="attachment" accept=".pdf,.doc,.docx,.txt"></label><br>
        <button type="submit">Submit</button>
    </form>
    <p>Version 3 - Full contact form with honeypot, attachment</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options":        "DENY",
					"X-Content-Type-Options": "nosniff",
					"Referrer-Policy":        "strict-origin-when-cross-origin",
				},
				Cookies: []CookieDef{
					{Name: "form_submitted", Value: "false", Path: "/contact", HttpOnly: false, Secure: false},
				},
			},
		},
	}
}

// ===== PROFILE PAGE =====
func getProfilePage() PageDefinition {
	return PageDefinition{
		Path:        "/profile",
		Description: "User profile page",
		Versions: map[int]PageVersion{
			1: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Profile v1</title>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>User Profile</h1>
    <form action="/profile/update" method="POST" id="profile-form">
        <label>Display Name: <input type="text" name="display_name"></label><br>
        <label>Bio: <textarea name="bio" rows="3" cols="40"></textarea></label><br>
        <button type="submit">Update Profile</button>
    </form>
    <p>Version 1 - Basic profile</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers:     map[string]string{},
				Cookies:     []CookieDef{},
			},
			2: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Profile v2</title>
    <script src="/static/profile.js"></script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>User Profile</h1>
    <form action="/profile/update" method="POST" id="profile-form">
        <input type="hidden" name="csrf_token" value="profile_csrf">
        <input type="hidden" name="user_id" value="12345">
        <label>Display Name: <input type="text" name="display_name" required></label><br>
        <label>Email: <input type="email" name="email" required></label><br>
        <label>Bio: <textarea name="bio" rows="3" cols="40"></textarea></label><br>
        <button type="submit">Update Profile</button>
    </form>
    
    <form action="/profile/password" method="POST" id="password-form">
        <input type="hidden" name="csrf_token" value="profile_csrf">
        <h2>Change Password</h2>
        <label>Current Password: <input type="password" name="current_password" required></label><br>
        <label>New Password: <input type="password" name="new_password" required></label><br>
        <label>Confirm Password: <input type="password" name="confirm_password" required></label><br>
        <button type="submit">Change Password</button>
    </form>
    <p>Version 2 - Added password change form</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options": "DENY",
				},
				Cookies: []CookieDef{
					{Name: "user_session", Value: "user_sess_123", Path: "/", HttpOnly: true, Secure: false},
				},
			},
			3: {
				HTML: `<!DOCTYPE html>
<html>
<head>
    <title>Profile v3</title>
    <script src="/static/profile.js"></script>
    <script src="/static/avatar.js"></script>
</head>
<body>
    <nav class="main-nav">
        <a href="/">Home</a> | 
        <a href="/login">Login</a> | 
        <a href="/admin">Admin</a> | 
        <a href="/upload">Upload</a> | 
        <a href="/contact">Contact</a> | 
        <a href="/profile">Profile</a>
    </nav>
    <h1>My Account</h1>
    <form action="/api/profile/update" method="POST" id="profile-form" class="profile-form" enctype="multipart/form-data">
        <input type="hidden" name="csrf_token" value="profile_csrf_v3">
        <input type="hidden" name="user_id" value="12345">
        <input type="hidden" name="form_version" value="3">
        
        <h2>Profile Information</h2>
        <label>Avatar: <input type="file" name="avatar" accept="image/*"></label><br>
        <label>Display Name: <input type="text" name="display_name" required></label><br>
        <label>Email: <input type="email" name="email" required></label><br>
        <label>Phone: <input type="tel" name="phone"></label><br>
        <label>Website: <input type="url" name="website"></label><br>
        <label>Bio: <textarea name="bio" rows="3" cols="40" maxlength="500"></textarea></label><br>
        <label>Location: <input type="text" name="location"></label><br>
        <button type="submit">Save Profile</button>
    </form>
    
    <form action="/api/profile/password" method="POST" id="password-form" class="security-form">
        <input type="hidden" name="csrf_token" value="profile_csrf_v3">
        <h2>Security Settings</h2>
        <label>Current Password: <input type="password" name="current_password" required></label><br>
        <label>New Password: <input type="password" name="new_password" required minlength="8"></label><br>
        <label>Confirm Password: <input type="password" name="confirm_password" required></label><br>
        <label><input type="checkbox" name="enable_2fa"> Enable Two-Factor Authentication</label><br>
        <button type="submit">Update Security</button>
    </form>
    
    <form action="/api/profile/delete" method="POST" id="delete-form" class="danger-form">
        <input type="hidden" name="csrf_token" value="profile_csrf_v3">
        <h2>Delete Account</h2>
        <label>Confirm Password: <input type="password" name="confirm_delete_password" required></label><br>
        <label><input type="checkbox" name="confirm_delete" required> I understand this action is irreversible</label><br>
        <button type="submit" class="danger">Delete My Account</button>
    </form>
    <p>Version 3 - Full profile with avatar upload, 2FA, account deletion</p>
</body>
</html>`,
				ContentType: "text/html",
				Headers: map[string]string{
					"X-Frame-Options":           "DENY",
					"X-Content-Type-Options":    "nosniff",
					"Content-Security-Policy":   "default-src 'self'; img-src 'self' data:",
					"Strict-Transport-Security": "max-age=31536000",
					"Referrer-Policy":           "strict-origin-when-cross-origin",
				},
				Cookies: []CookieDef{
					{Name: "user_session", Value: "user_sess_secure", Path: "/", HttpOnly: true, Secure: true, SameSite: "Lax"},
					{Name: "csrf_token", Value: "profile_csrf_v3", Path: "/", HttpOnly: true, Secure: true, SameSite: "Strict"},
				},
			},
		},
	}
}
