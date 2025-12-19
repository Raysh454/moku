package assessor

import (
	"bytes"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// BuildAttackSurfaceFromHTML constructs an AttackSurface from HTML content and metadata.
// It extracts security-relevant attributes like forms, inputs, scripts, cookies, and parameters.
func BuildAttackSurfaceFromHTML(
	rawURL, snapshotID string,
	statusCode int,
	headers map[string]string,
	body []byte,
) (*AttackSurface, error) {
	as := &AttackSurface{
		URL:         rawURL,
		SnapshotID:  snapshotID,
		CollectedAt: time.Now().UTC(),
		StatusCode:  statusCode,
		Headers:     headers,
	}

	// Extract content type from headers
	// Normalize header lookup to lowercase for consistency
	for k, v := range headers {
		if strings.ToLower(k) == "content-type" {
			as.ContentType = v
			break
		}
	}

	// Parse query parameters from URL
	if rawURL != "" {
		parsedURL, err := url.Parse(rawURL)
		if err == nil && parsedURL.RawQuery != "" {
			queryParams := parsedURL.Query()
			for name := range queryParams {
				as.GetParams = append(as.GetParams, Param{
					Name:   name,
					Origin: "query",
				})
			}
		}
	}

	// Extract cookies from Set-Cookie headers
	for key, value := range headers {
		if strings.ToLower(key) == "set-cookie" {
			cookie := parseCookie(value)
			if cookie != nil {
				as.Cookies = append(as.Cookies, *cookie)
			}
		}
	}

	// Parse HTML to extract forms, inputs, and scripts
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		// Return what we have so far even if HTML parsing fails
		return as, nil
	}

	// Extract forms and their inputs
	doc.Find("form").Each(func(i int, form *goquery.Selection) {
		formData := Form{
			Action: getAttr(form, "action"),
			Method: strings.ToUpper(getAttr(form, "method")),
		}
		if formData.Method == "" {
			formData.Method = "GET"
		}

		// Extract form inputs
		form.Find("input, textarea, select").Each(func(j int, input *goquery.Selection) {
			inputName := getAttr(input, "name")
			if inputName == "" {
				return
			}

			inputType := getAttr(input, "type")
			if inputType == "" {
				inputType = "text" // default for <input> without type
			}

			_, required := input.Attr("required")

			formData.Inputs = append(formData.Inputs, FormInput{
				Name:     inputName,
				Type:     inputType,
				Required: required,
			})

			// Track as param
			as.PostParams = append(as.PostParams, Param{
				Name:   inputName,
				Origin: "form",
			})
		})

		as.Forms = append(as.Forms, formData)
	})

	// Extract scripts
	doc.Find("script").Each(func(i int, script *goquery.Selection) {
		src := getAttr(script, "src")
		if src != "" {
			as.Scripts = append(as.Scripts, ScriptInfo{
				Src:    src,
				Inline: false,
			})
		} else {
			// Inline script
			as.Scripts = append(as.Scripts, ScriptInfo{
				Inline: true,
			})
		}
	})

	// Event handlers are harder to detect reliably without JS execution
	// For now, we'll leave this as a stub for future enhancement
	// Common event attributes: onclick, onload, onsubmit, etc.

	return as, nil
}

// parseCookie parses a Set-Cookie header value into CookieInfo.
// This is a simplified parser for the most common attributes.
func parseCookie(setCookieHeader string) *CookieInfo {
	parts := strings.Split(setCookieHeader, ";")
	if len(parts) == 0 {
		return nil
	}

	// First part is name=value
	nameValue := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	if len(nameValue) < 1 || nameValue[0] == "" {
		return nil
	}

	cookie := &CookieInfo{
		Name: nameValue[0],
	}

	// Parse attributes
	for i := 1; i < len(parts); i++ {
		attr := strings.TrimSpace(parts[i])
		attrLower := strings.ToLower(attr)

		if strings.HasPrefix(attrLower, "domain=") {
			attrParts := strings.SplitN(attr, "=", 2)
			if len(attrParts) == 2 {
				cookie.Domain = attrParts[1]
			}
		} else if strings.HasPrefix(attrLower, "path=") {
			attrParts := strings.SplitN(attr, "=", 2)
			if len(attrParts) == 2 {
				cookie.Path = attrParts[1]
			}
		} else if attrLower == "secure" {
			cookie.Secure = true
		} else if attrLower == "httponly" {
			cookie.HttpOnly = true
		} else if strings.HasPrefix(attrLower, "samesite=") {
			attrParts := strings.SplitN(attr, "=", 2)
			if len(attrParts) == 2 {
				cookie.SameSite = attrParts[1]
			}
		}
	}

	return cookie
}

// getAttr safely retrieves an attribute value from a goquery selection.
func getAttr(sel *goquery.Selection, attrName string) string {
	val, exists := sel.Attr(attrName)
	if exists {
		return strings.TrimSpace(val)
	}
	return ""
}
