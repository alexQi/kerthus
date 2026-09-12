package appmanifest

import (
	"encoding/json"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/usecase/core"
)

func (m Manifest) Definition() core.Definition {
	d := core.Definition{App: catalog.App{Code: m.AppCode, Name: m.Name, Version: m.ReleaseVersion, ServiceKey: m.ServiceKey, RoutePrefix: m.RoutePrefix, Home: m.Home, FrontendEntry: m.FrontendEntry, ResourceVersion: m.ResourceVersion}, Parents: map[string]string{}, OperationResources: map[string]string{}}
	for _, r := range m.Resources {
		meta, _ := json.Marshal(r.Meta)
		if r.Meta == nil {
			meta = []byte("{}")
		}
		d.Resources = append(d.Resources, catalog.Resource{Code: r.Code, Name: r.Name, Type: r.Type, Path: r.Path, Component: r.Component, OpenWith: r.OpenWith, Redirect: r.Redirect, Icon: r.Icon, Sort: r.Sort, IsDataAccess: r.DataScope, MetaJSON: string(meta)})
		d.Parents[r.Code] = r.ParentCode
	}
	for _, op := range m.Operations {
		d.Operations = append(d.Operations, catalog.Operation{OperationID: op.ID, Method: op.Method, Path: op.Path, GroupName: op.Group, Action: op.Action})
		d.OperationResources[op.ID] = op.ResourceCode
	}
	d.App.ManifestHash, _ = m.Fingerprint()
	return d
}
