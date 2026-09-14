package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

const PageContextVersion = "page_context.v1"
const MaxPageContextBytes = 16 << 10

// PageContext is untrusted UI context. Identity fields are always replaced by
// the authenticated request context before it is given to an agent.
type PageContext struct {
	Version         string           `json:"version"`
	Route           string           `json:"route,omitempty"`
	Title           string           `json:"title,omitempty"`
	AppID           int64            `json:"app_id,omitempty"`
	TenantID        int64            `json:"tenant_id,omitempty"`
	SelectedIDs     []string         `json:"selected_ids,omitempty"`
	Filters         map[string]any   `json:"filters,omitempty"`
	VisibleData     []map[string]any `json:"visible_data,omitempty"`
	FormData        map[string]any   `json:"form_data,omitempty"`
	Locale          string           `json:"locale,omitempty"`
	AvailableRoutes []string         `json:"available_routes,omitempty"`
}

// TrustedContext is supplied by the authenticated gateway, never by the page.
type TrustedContext struct {
	UserID            int64  `json:"user_id"`
	TenantID          int64  `json:"tenant_id"`
	AppID             int64  `json:"app_id"`
	PermissionVersion string `json:"permission_version,omitempty"`
}

type ContextEnvelope struct {
	Trusted TrustedContext `json:"trusted"`
	Page    PageContext    `json:"untrusted_context"`
}

func NormalizePageContext(in PageContext, trusted TrustedContext) (ContextEnvelope, error) {
	in.Version = PageContextVersion
	in.Route = limit(strings.TrimSpace(in.Route), 256)
	in.Title = limit(strings.TrimSpace(in.Title), 256)
	in.Locale = limit(strings.TrimSpace(in.Locale), 32)
	in.AppID, in.TenantID = trusted.AppID, trusted.TenantID
	in.AvailableRoutes = limitStrings(in.AvailableRoutes, 200, 256)
	in.SelectedIDs = limitStrings(in.SelectedIDs, 100, 128)
	in.Filters = sanitizeMap(in.Filters)
	in.FormData = sanitizeMap(in.FormData)
	in.VisibleData = sanitizeRows(in.VisibleData, 50)
	out := ContextEnvelope{Trusted: trusted, Page: in}
	b, err := json.Marshal(out)
	if err != nil {
		return ContextEnvelope{}, err
	}
	if len(b) > MaxPageContextBytes {
		return ContextEnvelope{}, fmt.Errorf("agent page context exceeds %d bytes", MaxPageContextBytes)
	}
	return out, nil
}

func sanitizeRows(rows []map[string]any, max int) []map[string]any {
	if len(rows) > max {
		rows = rows[:max]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, sanitizeMap(row))
	}
	return out
}

func sanitizeMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		lk := strings.ToLower(k)
		// Page context is sent to an external model. Keep the deny-list broad
		// enough to cover common credential aliases, including camelCase keys
		// normalized above, without dropping ordinary IDs.
		if strings.Contains(lk, "password") || strings.Contains(lk, "token") || strings.Contains(lk, "secret") || strings.Contains(lk, "cookie") || strings.Contains(lk, "authorization") || strings.Contains(lk, "api_key") || strings.Contains(lk, "apikey") || strings.Contains(lk, "access_key") || strings.Contains(lk, "accesskey") || strings.Contains(lk, "private_key") || strings.Contains(lk, "privatekey") || strings.Contains(lk, "credential") {
			continue
		}
		out[k] = sanitizeValue(v)
	}
	return out
}

func sanitizeValue(v any) any {
	switch x := v.(type) {
	case string:
		return limit(x, 1024)
	case map[string]any:
		return sanitizeMap(x)
	case []any:
		if len(x) > 50 {
			x = x[:50]
		}
		for i := range x {
			x[i] = sanitizeValue(x[i])
		}
		return x
	default:
		return v
	}
}

func limit(v string, n int) string {
	r := []rune(v)
	if len(r) > n {
		return string(r[:n])
	}
	return v
}
func limitStrings(in []string, max, n int) []string {
	if len(in) > max {
		in = in[:max]
	}
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = limit(v, n)
	}
	return out
}
