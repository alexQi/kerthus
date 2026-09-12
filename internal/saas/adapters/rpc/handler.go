package rpc

import (
	"context"
	"encoding/json"
	microerrors "go-micro.dev/v6/errors"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/platform/rpcauth"
	"kerthus/internal/saas/domain/fault"
	c "kerthus/internal/saas/usecase/contracts"
	"kerthus/internal/saas/usecase/core"
)

type Handler struct {
	service *core.Service
	auth    *rpcauth.Authenticator
}

func New(service *core.Service, config rpcauth.Config) (*Handler, error) {
	a, e := rpcauth.New(config)
	if e != nil {
		return nil, e
	}
	return &Handler{service: service, auth: a}, nil
}
func mapValue(in, out any) error {
	data, e := json.Marshal(in)
	if e != nil {
		return e
	}
	return json.Unmarshal(data, out)
}
func rpcError(e error) error {
	if e == nil {
		return nil
	}
	code, message := fault.Code(e)
	return microerrors.New("kerthus.saas", message, code)
}

var _ pb.PlatformHandler = (*Handler)(nil)

func (h *Handler) Login(ctx context.Context, in *pb.LoginRequest, out *pb.LoginReply) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "Login", ""); e != nil {
		return rpcError(e)
	}
	var request c.LoginRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.Login(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) Logout(ctx context.Context, in *pb.Context, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "Logout", ""); e != nil {
		return rpcError(e)
	}
	var request c.Context
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.Logout(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) Profile(ctx context.Context, in *pb.Context, out *pb.User) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "Profile", ""); e != nil {
		return rpcError(e)
	}
	var request c.Context
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.Profile(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) Auth(ctx context.Context, in *pb.Context, out *pb.AuthReply) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "Auth", ""); e != nil {
		return rpcError(e)
	}
	var request c.Context
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.Auth(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) Query(ctx context.Context, in *pb.QueryRequest, out *pb.QueryReply) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "Query", ""); e != nil {
		return rpcError(e)
	}
	var request c.QueryRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.Query(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) Save(ctx context.Context, in *pb.SaveRequest, out *pb.SaveReply) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "Save", ""); e != nil {
		return rpcError(e)
	}
	var request c.SaveRequest
	if e := mapValue(in.GetContext(), &request.Context); e != nil {
		return rpcError(e)
	}
	request.Password = in.Password
	switch v := in.Entity.(type) {
	case *pb.SaveRequest_User:
		request.User = &c.User{}
		if e := mapValue(v.User, request.User); e != nil {
			return rpcError(e)
		}
	case *pb.SaveRequest_Tenant:
		request.Tenant = &c.Tenant{}
		if e := mapValue(v.Tenant, request.Tenant); e != nil {
			return rpcError(e)
		}
	case *pb.SaveRequest_Member:
		request.Member = &c.Member{}
		if e := mapValue(v.Member, request.Member); e != nil {
			return rpcError(e)
		}
	case *pb.SaveRequest_Org:
		request.Org = &c.Org{}
		if e := mapValue(v.Org, request.Org); e != nil {
			return rpcError(e)
		}
	case *pb.SaveRequest_Position:
		request.Position = &c.Position{}
		if e := mapValue(v.Position, request.Position); e != nil {
			return rpcError(e)
		}
	case *pb.SaveRequest_App:
		request.App = &c.App{}
		if e := mapValue(v.App, request.App); e != nil {
			return rpcError(e)
		}
	case *pb.SaveRequest_Resource:
		request.Resource = &c.Resource{}
		if e := mapValue(v.Resource, request.Resource); e != nil {
			return rpcError(e)
		}
	case *pb.SaveRequest_Role:
		request.Role = &c.Role{}
		if e := mapValue(v.Role, request.Role); e != nil {
			return rpcError(e)
		}
	default:
		return rpcError(fault.Invalid("必须提供实体"))
	}
	reply, e := h.service.Save(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) Delete(ctx context.Context, in *pb.DeleteRequest, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "Delete", ""); e != nil {
		return rpcError(e)
	}
	var request c.DeleteRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.Delete(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) SetStatus(ctx context.Context, in *pb.StatusRequest, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "SetStatus", ""); e != nil {
		return rpcError(e)
	}
	var request c.StatusRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.SetStatus(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) ChangePassword(ctx context.Context, in *pb.PasswordRequest, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "ChangePassword", ""); e != nil {
		return rpcError(e)
	}
	var request c.PasswordRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.ChangePassword(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) ApproveTenant(ctx context.Context, in *pb.ApproveRequest, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "ApproveTenant", ""); e != nil {
		return rpcError(e)
	}
	var request c.ApproveRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.ApproveTenant(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) SetEntitlements(ctx context.Context, in *pb.EntitlementsRequest, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "SetEntitlements", ""); e != nil {
		return rpcError(e)
	}
	var request c.EntitlementsRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.SetEntitlements(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) SetRoleResources(ctx context.Context, in *pb.RoleResourcesRequest, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "SetRoleResources", ""); e != nil {
		return rpcError(e)
	}
	var request c.RoleResourcesRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.SetRoleResources(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) SetRoleMembers(ctx context.Context, in *pb.RoleMembersRequest, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "SetRoleMembers", ""); e != nil {
		return rpcError(e)
	}
	var request c.RoleMembersRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.SetRoleMembers(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) CheckAccess(ctx context.Context, in *pb.AccessRequest, out *pb.AccessReply) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "CheckAccess", in.GetAppCode()); e != nil {
		return rpcError(e)
	}
	var request c.AccessRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.CheckAccess(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) CreateUpload(ctx context.Context, in *pb.UploadRequest, out *pb.UploadReply) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "CreateUpload", ""); e != nil {
		return rpcError(e)
	}
	var request c.UploadRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.CreateUpload(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) ConfirmUpload(ctx context.Context, in *pb.ConfirmUploadRequest, out *pb.UploadReply) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "ConfirmUpload", ""); e != nil {
		return rpcError(e)
	}
	var request c.ConfirmUploadRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.ConfirmUpload(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) ResolveDownload(ctx context.Context, in *pb.FileRequest, out *pb.UploadReply) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "ResolveDownload", ""); e != nil {
		return rpcError(e)
	}
	var request c.FileRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.ResolveDownload(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) CancelUpload(ctx context.Context, in *pb.FileRequest, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "CancelUpload", ""); e != nil {
		return rpcError(e)
	}
	var request c.FileRequest
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.CancelUpload(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
func (h *Handler) Health(ctx context.Context, in *pb.Empty, out *pb.Result) error {
	if in == nil || out == nil {
		return rpcError(fault.Invalid("请求为空"))
	}
	if e := h.auth.Authorize(ctx, "Health", ""); e != nil {
		return rpcError(e)
	}
	var request c.Empty
	if e := mapValue(in, &request); e != nil {
		return rpcError(fault.Invalid("请求格式错误"))
	}
	reply, e := h.service.Health(ctx, &request)
	if e != nil {
		return rpcError(e)
	}
	return rpcError(mapValue(reply, out))
}
