package attacksurface

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
	headers map[string][]string,
	body []byte,
) (*AttackSurface, error) {
	as := newAttackSurface(rawURL, snapshotID, statusCode, headers)

	setContentType(as, headers)
	parseQueryParams(as, rawURL)
	extractCookies(as, headers)

	doc, err := parseHTMLDocument(body)
	if err != nil {
		// Return what we have so far even if HTML parsing fails
		return as, nil
	}

	extractForms(as, doc)
	extractScripts(as, doc)

	// Event handlers are harder to detect reliably without JS execution
	// For now, we'll leave this as a stub for future enhancement, same for framework hints and error indicators.
	// Common event attributes: onclick, onload, onsubmit, etc.

	return as, nil
}

func newAttackSurface(rawURL, snapshotID string, statusCode int, headers map[string][]string) *AttackSurface {
	return &AttackSurface{
		URL:         rawURL,
		SnapshotID:  snapshotID,
		CollectedAt: time.Now().UTC(),
		StatusCode:  statusCode,
		Headers:     headers,
	}
}

func setContentType(as *AttackSurface, headers map[string][]string) {
	for k, values := range headers {
		if strings.ToLower(k) == "content-type" && len(values) > 0 {
			as.ContentType = values[0]
			break
		}
	}
}

func parseQueryParams(as *AttackSurface, rawURL string) {
	if rawURL == "" {
		return
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.RawQuery == "" {
		return
	}

	queryParams := parsedURL.Query()
	for name := range queryParams {
		as.GetParams = append(as.GetParams, Param{
			Name:   name,
			Origin: "query",
		})
	}
}

func extractCookies(as *AttackSurface, headers map[string][]string) {
	for key, values := range headers {
		if strings.ToLower(key) == "set-cookie" {
			for _, value := range values {
				cookie := parseCookie(value)
				if cookie != nil {
					as.Cookies = append(as.Cookies, *cookie)
				}
			}
		}
	}
}

func parseHTMLDocument(body []byte) (*goquery.Document, error) {
	// Parse HTML to extract forms, inputs, and scripts
	return goquery.NewDocumentFromReader(bytes.NewReader(body))
}

// helper to split class attribute into tokens
func splitClasses(classAttr string) []string {
	if classAttr == "" {
		return nil
	}
	parts := strings.Fields(classAttr)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func extractForms(as *AttackSurface, doc *goquery.Document) {
	// Each formIndex here is 0-based index into document.getElementsByTagName("form")
	doc.Find("form").Each(func(formIndex int, formSel *goquery.Selection) {
		method := strings.ToUpper(getAttr(formSel, "method"))
		if method == "" {
			method = "GET"
		}

		formData := Form{
			Action:     getAttr(formSel, "action"),
			Method:     method,
			DOMIndex:   formIndex, // 0-based
			DOMID:      getAttr(formSel, "id"),
			DOMClasses: splitClasses(getAttr(formSel, "class")),
		}

		// Extract form inputs
		// inputIndex here is 0-based index into formElement.getElementsByTagName("input")
		formSel.Find("input, textarea, select").Each(func(inputIndex int, inputSel *goquery.Selection) {
			inputName := getAttr(inputSel, "name")
			if inputName == "" {
				return
			}

			inputType := strings.ToLower(getAttr(inputSel, "type"))
			if inputType == "" {
				// default for <input> without type; textarea/select will have no type
				inputType = "text"
			}

			_, required := inputSel.Attr("required")

			formInput := FormInput{
				Name:       inputName,
				Type:       inputType,
				Required:   required,
				DOMIndex:   inputIndex, // 0-based
				DOMID:      getAttr(inputSel, "id"),
				DOMClasses: splitClasses(getAttr(inputSel, "class")),
			}

			formData.Inputs = append(formData.Inputs, formInput)

			// Track as param
			as.PostParams = append(as.PostParams, Param{
				Name:   inputName,
				Origin: "form",
			})
		})

		as.Forms = append(as.Forms, formData)
	})
}

func extractScripts(as *AttackSurface, doc *goquery.Document) {
	// scriptIndex here is 0-based index into document.getElementsByTagName("script")
	doc.Find("script").Each(func(scriptIndex int, scriptSel *goquery.Selection) {
		src := getAttr(scriptSel, "src")
		if src != "" {
			as.Scripts = append(as.Scripts, ScriptInfo{
				Src:      src,
				Inline:   false,
				DOMIndex: scriptIndex, // 0-based
			})
		} else {
			as.Scripts = append(as.Scripts, ScriptInfo{
				Inline:   true,
				DOMIndex: scriptIndex, // 0-based
			})
		}
	})
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
