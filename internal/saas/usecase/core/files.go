package core

import (
	"context"
	"fmt"
	"kerthus/internal/saas/domain/fault"
	file "kerthus/internal/saas/domain/file"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
	"strings"
)

const MaxUploadSize int64 = 10 << 20
const DefaultAttachmentMaxBytes int64 = 50 << 20

func (s *Service) uploadLimit(private bool) int64 {
	if private {
		if s.AttachmentMaxBytes > 0 {
			return s.AttachmentMaxBytes
		}
		return DefaultAttachmentMaxBytes
	}
	return MaxUploadSize
}

func (s *Service) attachmentAccess(u ports.Unit, a *actor) error {
	_, _, resources, err := s.effective(u, a)
	if err != nil {
		return err
	}
	if len(resources) == 0 {
		return fault.Forbidden
	}
	return nil
}

func (s *Service) fileReply(v file.File) *c.UploadReply {
	return &c.UploadReply{ID: v.ID, ObjectKey: v.ObjectKey, MaxSize: s.uploadLimit(v.Private), Filename: v.Filename, ContentType: v.ContentType, Size: v.Size, Private: v.Private}
}

func (s *Service) CreateUpload(ctx context.Context, req *c.UploadRequest) (*c.UploadReply, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	out := &c.UploadReply{}
	if req.Size <= 0 || req.Size > s.uploadLimit(req.Private) {
		return nil, fault.Invalid("文件大小超出允许范围")
	}
	ext := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp"}[req.ContentType]
	if !req.Private && ext == "" {
		return nil, fault.Invalid("仅支持 PNG、JPEG、WebP 图片")
	}
	if len(req.ContentType) == 0 || len(req.ContentType) > 191 || strings.ContainsAny(req.ContentType, "\r\n") {
		return nil, fault.Invalid("文件类型不合法")
	}
	e := s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, true)
		if e != nil {
			return e
		}
		if req.Private {
			if e = s.attachmentAccess(u, a); e != nil {
				return e
			}
		}
		token, e := randomToken()
		if e != nil {
			return e
		}
		key := fmt.Sprintf("public/%d/%d/%s.%s", a.Tenant.ID, a.App.ID, token, ext)
		if req.Private {
			key = fmt.Sprintf("private/%d/%d/%s", a.Tenant.ID, a.App.ID, token)
		}
		v := file.File{TenantID: a.Tenant.ID, AppID: a.App.ID, UserID: a.User.ID, ObjectKey: key, ContentType: req.ContentType, Filename: file.SafeFilename(req.Filename), Private: req.Private, Size: req.Size, CreatedAt: s.now()}
		if e = u.Save(ports.Files, &v); e != nil {
			return e
		}
		out = s.fileReply(v)
		return nil
	})
	return out, e
}
func (s *Service) ConfirmUpload(ctx context.Context, req *c.ConfirmUploadRequest) (*c.UploadReply, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	out := &c.UploadReply{}
	e := s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, true)
		if e != nil {
			return e
		}
		var v file.File
		if e = u.Get(ports.Files, req.ID, &v); e != nil {
			return e
		}
		if v.TenantID != a.Tenant.ID || v.AppID != a.App.ID || v.UserID != a.User.ID {
			return fault.Forbidden
		}
		if v.Private {
			if e = s.attachmentAccess(u, a); e != nil {
				return e
			}
		}
		if v.Status == 1 {
			out = s.fileReply(v)
			return nil
		}
		if s.Objects == nil {
			return fault.New(503, "文件存储未配置")
		}
		size, ct, e := s.Objects.Stat(ctx, v.ObjectKey)
		if e != nil {
			return e
		}
		if size != v.Size || ct != v.ContentType {
			return fault.Invalid("上传对象与授权不一致")
		}
		v.Status = 1
		if e = u.Save(ports.Files, &v); e != nil {
			return e
		}
		out = s.fileReply(v)
		return s.record(u, a, "file.upload", v.ID, req.Context)
	})
	return out, e
}

func (s *Service) ResolveDownload(ctx context.Context, req *c.FileRequest) (*c.UploadReply, error) {
	if req == nil || (req.ID <= 0 && req.ObjectKey == "") || req.ID < 0 || len(req.ObjectKey) > 255 {
		return nil, fault.Invalid("缺少有效文件标识")
	}
	var out *c.UploadReply
	err := s.DB.Read(ctx, func(u ports.Unit) error {
		a, err := s.resolve(ctx, u, req.Context, true)
		if err != nil {
			return err
		}
		filter := filter("tenant_id", a.Tenant.ID, "app_id", a.App.ID, "status", 1)
		if req.ID > 0 {
			filter.Equal["id"] = req.ID
		}
		if req.ObjectKey != "" {
			filter.Equal["object_key"] = req.ObjectKey
		}
		v, err := first[file.File](u, ports.Files, filter)
		if err != nil {
			return err
		}
		if v.Private {
			if err = s.attachmentAccess(u, a); err != nil {
				return err
			}
		}
		out = s.fileReply(v)
		return nil
	})
	return out, err
}

// CancelUpload is exposed only to the authenticated gateway RPC principal.
// The pending random key is an internal cleanup capability, never returned to
// an HTTP client before confirmation. This also permits cleanup after logout.
func (s *Service) CancelUpload(ctx context.Context, req *c.FileRequest) (*c.Result, error) {
	if req == nil || req.ID <= 0 || req.Context == nil || req.Context.TenantID <= 0 || req.Context.AppID <= 0 || req.ObjectKey == "" {
		return nil, fault.Invalid("缺少有效文件标识")
	}
	err := s.DB.Write(ctx, func(u ports.Unit) error {
		v, err := first[file.File](u, ports.Files, filter("id", req.ID, "object_key", req.ObjectKey, "tenant_id", req.Context.TenantID, "app_id", req.Context.AppID))
		if err != nil {
			return err
		}
		if v.Status == 1 {
			return fault.Conflict("已确认文件不能取消上传")
		}
		if s.Objects == nil {
			return fault.New(503, "文件存储未配置")
		}
		if err = s.Objects.Remove(ctx, v.ObjectKey); err != nil {
			return err
		}
		return u.Delete(ports.Files, filter("id", v.ID, "status", 0))
	})
	return &c.Result{Success: err == nil}, err
}
