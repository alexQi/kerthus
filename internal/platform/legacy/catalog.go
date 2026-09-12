// Package legacy maps a read-only legacy snapshot to the foundation schema.
package legacy

import (
	"encoding/json"
	"fmt"
	"kerthus/internal/platform/appmanifest"
	"kerthus/internal/saas/domain/catalog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Row accepts database/sql values without depending on a particular driver.
type Row = map[string]any

func String(row Row, key string) string {
	switch v := row[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func Int(row Row, key string) int64 {
	n, _ := strconv.ParseInt(String(row, key), 10, 64)
	return n
}

type CatalogResult struct {
	Apps       []catalog.App
	Resources  []catalog.Resource
	Operations []catalog.Operation
	Warnings   []string
	// Every source candidate must have equivalent effective tenant/role grants
	// before the caller may persist the canonical operation. The key is
	// "<app_id> <METHOD> <path>". Catalog intentionally does not inspect grants.
	BindingCandidates map[string][]int64
}

// Catalog preserves legacy identities and permissions. Only known foundation
// components and current HTTP operations are executable. It never assigns an
// old menu grant to a new, broader capability resource.
func Catalog(apps, resources, apis []Row, operations []appmanifest.Operation, projectRoot string) (CatalogResult, error) {
	out := CatalogResult{BindingCandidates: map[string][]int64{}}
	appByID := map[int64]catalog.App{}
	appCodes := map[string]bool{}
	for _, row := range apps {
		a := catalog.App{ID: Int(row, "id"), Code: String(row, "code"), Name: String(row, "name"), Version: String(row, "version"), Status: int32(Int(row, "status")), CreatedAt: Int(row, "created_at"), UpdatedAt: Int(row, "updated_at")}
		a.Icon, a.Description, a.Remark = String(row, "icon"), String(row, "desc"), String(row, "remark")
		a.Type, a.URL, a.IsPublic = catalog.AppType(String(row, "type")), String(row, "url"), Int(row, "is_public") == 1
		if a.ID <= 0 || a.Code == "" || appByID[a.ID].ID != 0 || appCodes[a.Code] {
			return out, fmt.Errorf("legacy app identity is invalid or duplicated: id=%d", a.ID)
		}
		appCodes[a.Code] = true
		if (a.Code == "system" || a.Code == "basic") && a.Type == "self" {
			a.ServiceKey, a.RoutePrefix = "kerthus.saas", "/system"
			a.Home, a.FrontendEntry = "/"+a.Code+"/dashboard", "bundled"
		} else if a.Type == "third" && catalog.SafeWebURL(a.URL) {
			// A link has no backend deployment or service route to migrate.
		} else {
			a.Status = 0
			out.Warnings = append(out.Warnings, fmt.Sprintf("app %d (%s): business application retained disabled", a.ID, a.Code))
		}
		if Int(row, "is_del") != 0 || a.Status != 1 {
			a.Status = 0
		}
		appByID[a.ID] = a
		out.Apps = append(out.Apps, a)
	}
	resourceByID := map[int64]catalog.Resource{}
	resourceByCode := map[string]catalog.Resource{}
	for _, row := range resources {
		r := catalog.Resource{ID: Int(row, "id"), AppID: Int(row, "app_id"), ParentID: Int(row, "parent_id"), Code: String(row, "code"), Name: String(row, "name"), Type: String(row, "type"), Path: String(row, "path"), Component: strings.TrimPrefix(String(row, "component"), "/"), Icon: String(row, "icon"), Redirect: String(row, "redirect"), OpenWith: String(row, "open_with"), Remark: String(row, "remark"), MetaJSON: String(row, "meta"), Status: int32(Int(row, "status")), Sort: int32(Int(row, "sort")), IsPublic: Int(row, "is_public") == 1, IsDataAccess: Int(row, "is_data_access") == 1, CreatedAt: Int(row, "created_at"), UpdatedAt: Int(row, "updated_at")}
		a, ok := appByID[r.AppID]
		if !ok || r.ID <= 0 || r.Code == "" || resourceByID[r.ID].ID != 0 || resourceByCode[r.Code].ID != 0 {
			return out, fmt.Errorf("legacy resource identity/application invalid or duplicated: id=%d", r.ID)
		}
		if !json.Valid([]byte(r.MetaJSON)) {
			r.MetaJSON = "{}"
			out.Warnings = append(out.Warnings, fmt.Sprintf("resource %d: invalid metadata replaced with empty object", r.ID))
		}
		supported := a.Code == "system" || a.Code == "basic"
		supported = supported && strings.HasPrefix(r.Code, a.Code+":")
		if strings.HasPrefix(r.Code, "basic:rcc") || r.Code == "system:log" || strings.HasPrefix(r.Code, "system:log:") {
			supported = false
		}
		if r.Code == "system:dashboard" || r.Code == "basic:dashboard" {
			r.Component = "common/dashboard/index"
		}
		switch r.Type {
		case "menu", "view":
			switch r.OpenWith {
			case "inside":
				supported = supported && catalog.SafeWebURL(r.Component) && catalog.LocalAppPath(a.Code, r.Path)
			case "outside":
				supported = supported && catalog.SafeWebURL(r.Path) && (r.Component == "" || r.Component == "LAYOUT") && r.Redirect == ""
			case "", "component", "route":
				if r.Component != "LAYOUT" {
					if !componentExists(projectRoot, r.Component) {
						supported = false
					} else {
						r.Type = "view"
					}
				}
				if r.Path != "/"+a.Code && !strings.HasPrefix(r.Path, "/"+a.Code+"/") {
					supported = false
				}
			default:
				supported = false
			}
		case "action", "field":
			// Legacy action records carry duplicate page paths and LAYOUT.
			// They are permission entries, never navigable frontend routes.
		default:
			supported = false
		}
		if !supported || r.Status != 1 || a.Status != 1 {
			r.Status = 0
		}
		if !supported {
			out.Warnings = append(out.Warnings, fmt.Sprintf("resource %d (%s): unsupported resource retained disabled", r.ID, r.Code))
		}
		resourceByID[r.ID], resourceByCode[r.Code] = r, r
		out.Resources = append(out.Resources, r)
	}
	// Missing, cross-app or cyclic parents must never become live navigation.
	for i := range out.Resources {
		r := &out.Resources[i]
		seen := map[int64]bool{r.ID: true}
		for parent := r.ParentID; parent != 0; {
			p, ok := resourceByID[parent]
			if !ok || p.AppID != r.AppID || seen[parent] || p.OpenWith == "outside" {
				r.Status, r.ParentID = 0, 0
				out.Warnings = append(out.Warnings, fmt.Sprintf("resource %d: invalid parent hierarchy detached and disabled", r.ID))
				break
			}
			seen[parent] = true
			if p.Status != 1 {
				r.Status = 0
			}
			parent = p.ParentID
		}
		resourceByID[r.ID], resourceByCode[r.Code] = *r, *r
	}

	current := map[string]appmanifest.Operation{}
	for _, op := range operations {
		key := strings.ToUpper(op.Method) + " " + op.Path
		if old, ok := current[key]; ok && (old.ID != op.ID || old.Action != op.Action) {
			return out, fmt.Errorf("ambiguous current operation %s", key)
		}
		current[key] = op
	}
	bindings := map[string]map[int64]bool{}
	for _, row := range apis {
		id := Int(row, "resource_id")
		r, ok := resourceByID[id]
		if !ok {
			out.Warnings = append(out.Warnings, fmt.Sprintf("legacy API %d: missing resource %d, skipped", Int(row, "id"), id))
			continue
		}
		if r.Status != 1 {
			continue
		}
		key := strings.ToUpper(String(row, "method")) + " " + String(row, "uri")
		if _, ok := current[key]; !ok {
			out.Warnings = append(out.Warnings, fmt.Sprintf("legacy API %d: unsupported method/path, skipped", Int(row, "id")))
			continue
		}
		if bindings[key] == nil {
			bindings[key] = map[int64]bool{}
		}
		bindings[key][id] = true
	}
	// Explicit semantic aliases cover renamed endpoints and list helpers. They
	// reuse existing resources; no fresh permission is granted to any role.
	aliases := map[string][]string{
		"GET /system/employee/setStatus": {"basic:user:main:setStatus", "system:tenant:detail:employee:setStatus"},
		"POST /system/tenant/approve":    {"system:tenant:main:verify"},
		"GET /system/org/query":          {"basic:user:org:view", "system:tenant:detail:query"},
	}
	for key, codes := range aliases {
		if _, ok := current[key]; !ok {
			continue
		}
		for _, code := range codes {
			r, ok := resourceByCode[code]
			if !ok || r.Status != 1 {
				continue
			}
			if bindings[key] == nil {
				bindings[key] = map[int64]bool{}
			}
			bindings[key][r.ID] = true
		}
	}
	readAliases := map[string]string{
		"GET /system/position/getItems":        "GET /system/position/query",
		"GET /system/employee/getTenantApps":   "GET /system/employee/queryTenantApps",
		"GET /system/employee/hasApp":          "GET /system/employee/queryTenantApps",
		"GET /system/app/getTenantResourceIds": "GET /system/app/getTenantResources",
	}
	for destination, source := range readAliases {
		if _, ok := current[destination]; !ok {
			continue
		}
		for id := range bindings[source] {
			if bindings[destination] == nil {
				bindings[destination] = map[int64]bool{}
			}
			bindings[destination][id] = true
		}
	}
	keys := make([]string, 0, len(bindings))
	for key := range bindings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		op := current[key]
		ids := make([]int64, 0, len(bindings[key]))
		for id := range bindings[key] {
			if appByID[resourceByID[id].AppID].Code == op.AppCode {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		if len(ids) == 0 {
			continue
		}
		r := resourceByID[ids[0]]
		bindingKey := fmt.Sprintf("%d %s %s", r.AppID, strings.ToUpper(op.Method), op.Path)
		out.BindingCandidates[bindingKey] = ids
		if len(ids) > 1 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("operation %s: candidate resources %v; canonical %d requires effective-grant equivalence validation", bindingKey, ids, r.ID))
		}
		out.Operations = append(out.Operations, catalog.Operation{ID: int64(len(out.Operations) + 1), AppID: r.AppID, ResourceID: r.ID, OperationID: op.ID, Method: strings.ToUpper(op.Method), Path: op.Path, GroupName: op.Group, Action: op.Action})
	}
	sort.Slice(out.Apps, func(i, j int) bool { return out.Apps[i].ID < out.Apps[j].ID })
	sort.Slice(out.Resources, func(i, j int) bool { return out.Resources[i].ID < out.Resources[j].ID })
	sort.Strings(out.Warnings)
	return out, nil
}

func componentExists(root, component string) bool {
	if component == "" || filepath.IsAbs(component) || strings.Contains(component, "..") || strings.ContainsAny(component, "\\?#%") {
		return false
	}
	base, err := filepath.EvalSymlinks(filepath.Join(root, "web/admin/src/views"))
	if err != nil {
		return false
	}
	path, err := filepath.EvalSymlinks(filepath.Join(base, component+".vue"))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
