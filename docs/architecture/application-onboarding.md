# 新应用接入首期底座

应用实现放在自己的 `internal/apps/<app_code>`、`cmd/<app_code>` 和逻辑数据库中，通过生成的 `gen/go/saas/v1` 契约调用平台。`internal/appkit/authorization` 只提供鉴权客户端和显式租户/owner范围判断。`scripts/check-boundaries.py` 阻止平台与应用实现包相互导入、不同应用直接导入实现。

## 1. 准备前端和注册文件

下面是将来一个应用定义的格式示意，不是当前已经部署的产品。

```yaml
schema_version: 1
app_code: notes
name: 笔记
release_version: 1.0.0
platform_api_version: v1
resource_version: 1
http_contract: api/http/apps/notes/v1.yaml
service_key: kerthus.notes
route_prefix: /api/apps/notes/v1
frontend_entry: bundled
home: /notes/home
resources:
  - code: notes:home
    name: 笔记
    type: view
    path: /notes/home
    component: apps/notes/index
    data_scope: true
```

组件必须真实存在于 `web/admin/src/views/apps/notes/index.vue`，需要重建前端。`component: LAYOUT` 可用作菜单父节点；旧 basic/system 组件继续保留原路径。资源码必须带自己应用的命名空间，视图路径必须属于 `/<app_code>`。父级用 `parent_code` 引用，菜单附加配置可用 `meta` 对象。

`http_contract` 是 OpenAPI 3 文档，每个操作需要以下元数据：

```yaml
openapi: 3.0.3
info: {title: Notes, version: 1.0.0}
paths:
  /api/apps/notes/v1/notes/{id}:
    get:
      operationId: notes.read
      x-app-code: notes
      x-resource-code: notes:home
      x-action: notes.read
      x-group: notes
      responses:
        '200': {description: Note}
```

schema v1 不支持第三方外部入口、执行安装脚本或公共匿名应用操作；模板参数支持单个路径段。注册会校验 schema/API版本、归属、路径、操作唯一性、资源环及前端组件存在性。

## 2. 验证、部署与注册

```sh
KERTHUS_ENV_FILE=.local/saas.env go run ./cmd/saasctl validate applications/notes/app.yaml
KERTHUS_ENV_FILE=.local/saas.env go run ./cmd/saasctl register applications/notes/app.yaml
```

首次注册创建应用目录与资源，后续更新需增加 `resource_version`；同版本内容改变会拒绝。版本摘要来自完整规范化清单及排序后的操作。更新保留资源 ID，删除的资源停用，新增资源不会自动加入旧租户/角色。已停用的应用不会因普通版本升级而恢复启用。平台用户在 UI 中创建的是未就绪目录草稿，完成清单注册后才可启用。

运行中的应用 HTTP 服务自行登记 Consul 节点：

- `Service.Name = kerthus.notes`，必须与清单 `service_key` 一致。
- `Node.Address = 私有主机:端口`，没有浏览器提供的 URL。
- `Node.Metadata["protocol"] = "http"`、`Node.Metadata["app_code"] = "notes"`。
- 持续更新 TTL/健康状态，退出注销；示例测试使用内存 registry 验证代理，不代替真实应用部署。

Gateway 只转发已授权、已登记的 method/path，保留完整 `/api/apps/notes/v1/...` 路径。服务不可用返回 503，不删除租户开通记录。首期应用 HTTP upstream 按可信私网接线；跨网络需要部署层的安全通道及相应代理扩展。

## 3. 应用必须自行鉴权

给应用独立随机服务凭据，在 SaaS 核心配置 `KERTHUS_RPC_APP_KEYS` 中绑定自己的 app_code。应用凭据只能访问 `Health` 和属于自己的 `CheckAccess`，不能修改平台角色/租户目录，也不能借传入其他 app_code 读取权限。

应用接到任何直连或网关请求后，调用 `authorization.New(platformClient, "notes", ownCredential)`，再执行 `CheckRequest(request)`。返回结果才是已验证的 actor/tenant/app。不要使用浏览器传来的 actor ID，不信任网关透传的自定义权限 metadata。Gateway 会移除 `X-Kerthus-*` 伪造头，但应用仍需在线检查。

查询和写入始终带 `tenant_id = result.TenantId`。若 `AllWithinTenant=false`，再限制允许的 owner 用户 ID；空 `UserIds` 是空范围。单条数据可用 `authorization.AllowsOwner(result, row.TenantID, row.OwnerID)`。这不替代应用自身对象归属、状态机和业务权限检查。

## 4. 租户开通与角色授权

注册与部署不等于租户开通。平台为租户选定应用资源上限和期限，租户管理员再配置业务角色及成员。应用有效性、成员有效性、租户开通和角色资源每次在线取交集。新增权限默认拒绝，已停用资源和撤销开通下次请求立即失效。

平台只维护通用元数据。新应用自己的表、migration、文件、任务和数据清理由应用管理；出现真实异步初始化需要时，再扩展 outbox/provisioning 状态，不借用平台事务直接写应用数据库。
