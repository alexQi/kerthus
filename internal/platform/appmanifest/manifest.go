// Package appmanifest defines the versioned, non-executable application catalog.
package appmanifest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"kerthus/internal/saas/domain/catalog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Manifest struct {
	SchemaVersion      int         `yaml:"schema_version"`
	AppCode            string      `yaml:"app_code"`
	Name               string      `yaml:"name"`
	ReleaseVersion     string      `yaml:"release_version"`
	PlatformAPIVersion string      `yaml:"platform_api_version"`
	ResourceVersion    int64       `yaml:"resource_version"`
	HTTPContract       string      `yaml:"http_contract"`
	ServiceKey         string      `yaml:"service_key"`
	RoutePrefix        string      `yaml:"route_prefix"`
	FrontendEntry      string      `yaml:"frontend_entry"`
	Home               string      `yaml:"home"`
	Resources          []Resource  `yaml:"resources"`
	Operations         []Operation `yaml:"-"`
}
type Resource struct {
	OpenWith   string         `yaml:"open_with,omitempty" json:",omitempty"`
	Redirect   string         `yaml:"redirect,omitempty" json:",omitempty"`
	Meta       map[string]any `yaml:"meta,omitempty"`
	Code       string         `yaml:"code"`
	ParentCode string         `yaml:"parent_code"`
	Name       string         `yaml:"name"`
	Type       string         `yaml:"type"`
	Path       string         `yaml:"path"`
	Component  string         `yaml:"component"`
	Icon       string         `yaml:"icon"`
	Sort       int32          `yaml:"sort"`
	DataScope  bool           `yaml:"data_scope"`
}
type Operation struct {
	ID           string `yaml:"operationId" json:"operation_id"`
	AppCode      string `yaml:"x-app-code" json:"app_code"`
	ResourceCode string `yaml:"x-resource-code" json:"resource_code"`
	Action       string `yaml:"x-action" json:"action"`
	Group        string `yaml:"x-group" json:"group"`
	Public       bool   `yaml:"x-public" json:"public"`
	Method       string `yaml:"-" json:"method"`
	Path         string `yaml:"-" json:"path"`
}
type Spec struct {
	OpenAPI string                          `yaml:"openapi"`
	Paths   map[string]map[string]Operation `yaml:"paths"`
}

var appCode = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,62}$`)
var paramSegment = regexp.MustCompile(`^\{[a-zA-Z][a-zA-Z0-9_]*\}$`)

func safeFile(root, ref string) (string, error) {
	if filepath.IsAbs(ref) || ref == "" {
		return "", fmt.Errorf("reference must be relative")
	}
	full := filepath.Join(root, ref)
	rel, e := filepath.Rel(root, full)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reference escapes project root")
	}
	resolved, e := filepath.EvalSymlinks(full)
	if e != nil {
		return "", e
	}
	rootResolved, e := filepath.EvalSymlinks(root)
	if e != nil {
		return "", e
	}
	rel, e = filepath.Rel(rootResolved, resolved)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("symlink escapes project root")
	}
	return resolved, nil
}
func LoadSpec(path string) ([]Operation, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var spec Spec
	if e = yaml.Unmarshal(b, &spec); e != nil {
		return nil, e
	}
	if !strings.HasPrefix(spec.OpenAPI, "3.") {
		return nil, fmt.Errorf("OpenAPI 3 is required")
	}
	ops := []Operation{}
	for path, methods := range spec.Paths {
		for method, op := range methods {
			switch strings.ToUpper(method) {
			case "GET", "POST", "PUT", "PATCH", "DELETE":
			default:
				continue
			}
			op.Path = path
			op.Method = strings.ToUpper(method)
			if op.ID == "" || op.Action == "" || op.ResourceCode == "" {
				return nil, fmt.Errorf("operation metadata missing: %s %s", method, path)
			}
			ops = append(ops, op)
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	return ops, nil
}
func Load(root, manifestFile string) (*Manifest, error) {
	root, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	path, e := safeFile(root, manifestFile)
	if e != nil {
		return nil, e
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var m Manifest
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if e = dec.Decode(&m); e != nil {
		return nil, e
	}
	if m.SchemaVersion != 1 || m.PlatformAPIVersion != "v1" || m.ResourceVersion < 1 || !appCode.MatchString(m.AppCode) || m.Name == "" || m.ReleaseVersion == "" {
		return nil, fmt.Errorf("invalid application metadata or unsupported version")
	}
	builtin := m.AppCode == "basic" || m.AppCode == "system"
	if !builtin {
		if m.RoutePrefix != "/api/apps/"+m.AppCode+"/v1" || m.ServiceKey == "" {
			return nil, fmt.Errorf("application route prefix/service key is invalid")
		}
	}
	if m.FrontendEntry != "bundled" {
		return nil, fmt.Errorf("only bundled frontends are supported in schema v1")
	}
	if !strings.HasPrefix(m.Home, "/"+m.AppCode+"/") {
		return nil, fmt.Errorf("home must belong to application")
	}
	resources := map[string]Resource{}
	hasHome := false
	for _, r := range m.Resources {
		if r.Code == "" || r.Name == "" {
			return nil, fmt.Errorf("resource code/name is required")
		}
		if _, ok := resources[r.Code]; ok {
			return nil, fmt.Errorf("duplicate resource %s", r.Code)
		}
		if !strings.HasPrefix(r.Code, m.AppCode+":") {
			return nil, fmt.Errorf("resource %s escapes application namespace", r.Code)
		}
		switch r.Type {
		case "menu", "view", "action", "field":
		default:
			return nil, fmt.Errorf("invalid resource type")
		}
		switch r.OpenWith {
		case "", "component", "route":
			if r.Component != "" && r.Component != "LAYOUT" {
				component := strings.TrimPrefix(r.Component, "/")
				if !catalog.ComponentReference(m.AppCode, r.Component) {
					return nil, fmt.Errorf("component escapes application namespace")
				}
				if _, e = safeFile(root, filepath.Join("web/admin/src/views", component+".vue")); e != nil {
					return nil, fmt.Errorf("component %s: %w", component, e)
				}
			}
		case "inside":
			if (r.Type != "menu" && r.Type != "view") || !catalog.SafeWebURL(r.Component) || !catalog.LocalAppPath(m.AppCode, r.Path) {
				return nil, fmt.Errorf("invalid embedded page")
			}
		case "outside":
			if (r.Type != "menu" && r.Type != "view") || !catalog.SafeWebURL(r.Path) || (r.Component != "" && r.Component != "LAYOUT") || r.Redirect != "" {
				return nil, fmt.Errorf("invalid external link")
			}
		default:
			return nil, fmt.Errorf("unsupported resource open_with")
		}
		if r.Path == m.Home && r.Component != "" && r.OpenWith != "outside" {
			hasHome = true
		}
		if r.OpenWith != "outside" && r.Path != "" && !catalog.LocalAppPath(m.AppCode, r.Path) {
			return nil, fmt.Errorf("resource route escapes application namespace")
		}
		if r.Redirect != "" && !catalog.LocalAppPath(m.AppCode, r.Redirect) {
			return nil, fmt.Errorf("redirect escapes application namespace")
		}
		resources[r.Code] = r
	}
	if !hasHome {
		return nil, fmt.Errorf("home component is not registered")
	}
	for _, r := range m.Resources {
		seen := map[string]bool{r.Code: true}
		parent := r.ParentCode
		for parent != "" {
			if seen[parent] {
				return nil, fmt.Errorf("resource cycle")
			}
			seen[parent] = true
			p, ok := resources[parent]
			if !ok || p.OpenWith == "outside" {
				return nil, fmt.Errorf("missing or external parent resource %s", parent)
			}
			parent = p.ParentCode
		}
	}
	specPath, e := safeFile(root, m.HTTPContract)
	if e != nil {
		return nil, e
	}
	ops, e := LoadSpec(specPath)
	if e != nil {
		return nil, e
	}
	ids := map[string]bool{}
	routes := map[string]bool{}
	for _, op := range ops {
		if !builtin && op.Public {
			return nil, fmt.Errorf("public application operations are not supported in schema v1")
		}
		if op.AppCode != m.AppCode {
			if !builtin {
				return nil, fmt.Errorf("contract contains another application's operation")
			}
			continue
		}
		if _, ok := resources[op.ResourceCode]; !ok {
			return nil, fmt.Errorf("operation references unknown resource %s", op.ResourceCode)
		}
		if ids[op.ID] {
			return nil, fmt.Errorf("duplicate operation ID")
		}
		ids[op.ID] = true
		if !builtin && !strings.HasPrefix(op.Path, m.RoutePrefix+"/") {
			return nil, fmt.Errorf("operation escapes route prefix")
		}
		if strings.ContainsAny(op.Path, "?#%") || strings.Contains(op.Path, "//") {
			return nil, fmt.Errorf("invalid route")
		}
		parts := strings.Split(op.Path, "/")
		for i, p := range parts {
			if p == "." || p == ".." {
				return nil, fmt.Errorf("invalid path segment")
			}
			if strings.ContainsAny(p, "{}") {
				if !paramSegment.MatchString(p) {
					return nil, fmt.Errorf("invalid path parameter")
				}
				parts[i] = "{}"
			}
		}
		key := op.Method + " " + strings.Join(parts, "/")
		if routes[key] {
			return nil, fmt.Errorf("duplicate or ambiguous route")
		}
		routes[key] = true
		for _, previous := range m.Operations {
			if routesOverlap(previous, op) {
				return nil, fmt.Errorf("ambiguous application routes")
			}
		}
		m.Operations = append(m.Operations, op)
	}
	return &m, nil
}
func (m Manifest) Fingerprint() (string, error) {
	b, e := json.Marshal(m)
	return fmt.Sprintf("%x", sha256.Sum256(b)), e
}

func routesOverlap(a, b Operation) bool {
	if a.Method != b.Method {
		return false
	}
	left, right := strings.Split(a.Path, "/"), strings.Split(b.Path, "/")
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] && !paramSegment.MatchString(left[i]) && !paramSegment.MatchString(right[i]) {
			return false
		}
	}
	return true
}
