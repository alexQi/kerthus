# 基础文件接口

迁移 `009_file_attachments.sql` 只给 `files` 增加 `filename` 与 `private`，原有图片记录保持公开图片语义；不改原数据库或旧静态资源。核心与网关部署时应使用相同 `KERTHUS_ATTACHMENT_MAX_BYTES`，默认 `52428800`（50 MiB），配置范围 1 MiB–1 GiB。图片仍固定 10 MiB。网关同时接收最多 4 个上传，超出返回 HTTP 429 和 `Retry-After: 2`。

## 上传兼容

`POST /app/file/upload` 接收 multipart 字段 `file`。旧字段 `directory`、`param` 继续接受，但不会参与存储路径。随机路径由核心生成，不信任客户端目录与文件名。

- 不指定 `purpose` 时，PNG/JPEG/WebP 按旧头像、Logo流程生成 `public/<tenant>/<app>/<random>.<ext>`；其他文件自动存成私有附件。
- 通用附件必须显式传 `purpose=attachment`，包括需要保密的图片。
- `purpose=image` 只接受 PNG/JPEG/WebP；其他 purpose 拒绝。
- 文件类型使用内容嗅探，不信任 multipart Content-Type。原文件名经过路径、控制符和长度清理，仅作下载展示名。
- 文件先流式写入权限 0600 的临时文件，校验大小后流式写入对象存储。临时文件每次请求结束删除，失败请求取消尚未确认的对象和元数据。

沿用请求头 `access-token`、`tenant-id`、`app-id`（以及适用的 `unit-id`、`section-id`）。返回中的 `data` 仍为字符串，旧图片组件无须改变解析：

```json
{
  "code": 0,
  "data": "private/2/3/<random>",
  "msg": "",
  "mesc": "",
  "file": {
    "id": 12,
    "filename": "季度报告.pdf",
    "size": 1234,
    "content_type": "application/pdf",
    "private": true,
    "max_size": 52428800,
    "download_path": "/app/file/download?key=private%2F2%2F3%2F<random>"
  }
}
```

`GET /app/file/limits` 返回 `{code:0,data:{image_max_bytes:10485760,attachment_max_bytes:52428800},msg:"",mesc:""}`，附件上限随部署配置变化。

## 私有附件下载

`GET /app/file/download?key=<object_key>` 或 `?id=<file_id>`，使用上述上下文请求头；两者同时提供时必须指向同一记录。也支持 HEAD。响应为二进制文件，`Content-Disposition: attachment` 支持中文 UTF-8 文件名，`Cache-Control: private, no-store`，禁止 MIME 嗅探。CORS 允许读取 `Content-Disposition` 与 `Content-Length`。

浏览器通过已有带鉴权头的 HTTP 客户端请求 Blob，创建本地 `URL.createObjectURL(blob)` 并触发下载，完成后撤销 URL。不得把 token 拼进下载 URL，不得把私有 key 拼到公开 `/files/` 或旧 CDN 地址。

下载与私有附件上传使用相同当前授权：有效会话、有效租户成员、已审批启用且未过期的租户、启用且有效开通的应用、该应用至少一个有效资源权限。同租户、同应用且拥有上述授权的成员可下载该应用的附件；这不是成员个人私有网盘。跨租户、跨应用、撤权、停用和过期均不能下载。返回 key 本身不能绕过授权。

公开 `/files/public/...` 保留图片兼容。`/files/private/...` 即便配置旧静态回源也始终拒绝。旧目录头像/Logo仍可使用受配置约束的旧资源回源。

## 失败清理

创建文件后只有确认通过才向 HTTP 客户端返回 key。存储或确认失败时，网关调用仅网关 RPC 凭据可访问的 `CancelUpload`，以随机 pending key、文件 ID、租户 ID、应用 ID 四项绑定清理。该内部清理凭据不能删除已确认文件，并且不依赖可能已登出的用户会话。跨租户上下文或猜测文件 ID 不能触发清理。

数据库或对象存储整体不可用时，清理会在独立的 10 秒上下文中尝试，失败会记录不含凭据和对象 key 的运维日志；这种基础设施故障需要运维重试 pending 文件清理，不把未确认对象公开。该接口当前不提供业务附件删除/业务记录绑定，后续应用应自行维护已确认附件的业务归属。
