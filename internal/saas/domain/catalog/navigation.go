package catalog

import (
	"net/url"
	"strings"
	"unicode"
)

// AppType accepts the misspelling documented by the legacy model.
func AppType(value string) string {
	switch value {
	case "", "self":
		return "self"
	case "third", "thrid":
		return "third"
	default:
		return value
	}
}

// SafeWebURL is a browser destination, never a backend service address or an
// authenticated request. Credentials and browser-ambiguous forms are rejected.
func SafeWebURL(value string) bool {
	if len(value) > 2048 || strings.Contains(value, "\\") || strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false
	}
	u, err := url.Parse(value)
	return err == nil && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")) && u.Hostname() != "" && u.User == nil && u.Opaque == ""
}

func LocalAppPath(appCode, value string) bool {
	return (value == "/"+appCode || strings.HasPrefix(value, "/"+appCode+"/")) && !strings.ContainsAny(value, "\\?#") && !strings.Contains(value, "..") && !strings.ContainsFunc(value, unicode.IsControl)
}

// ComponentReference validates references without coupling the core service to
// a frontend filesystem. Deployment manifests additionally verify file existence.
func ComponentReference(appCode, value string) bool {
	if value == "" || value == "LAYOUT" {
		return true
	}
	value = strings.TrimPrefix(value, "/") // Legacy bundled component syntax.
	if strings.Contains(value, "..") || strings.ContainsAny(value, "\\?#%:") || strings.HasPrefix(value, "/") || strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false
	}
	return appCode == "basic" || appCode == "system" || strings.HasPrefix(value, "apps/"+appCode+"/")
}
