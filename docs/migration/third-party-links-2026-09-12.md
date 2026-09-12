# 第三方应用、内外链与附件前端补齐

本轮在 2026-09-11 对照复查后继续实现先前明确未支持的应用类型、地址和链接路由。沿用原页面结构，无新增业务界面；前端 agent 未操作浏览器或真实数据库；主 agent 的实际验收与部署见 [补齐验收](foundation-completion-2026-09-12.md)。

## 兼容旧字段的契约

| 对象 | 字段 | 行为 |
| --- | --- | --- |
| 自建应用 | `type=self` | 原应用切换、刷新授权、进入可访问首页 |
| 第三方应用 | `type=third`、`url` | 有效租户开通后显示在可用应用中，直接新窗口打开配置地址，当前租户/应用上下文保持不变 |
| 旧拼写 | `type=thrid` | 前端回显按 `third` 兼容；保存使用 `third` |
| 应用公开属性 | `is_public=0/1` | 表单和列表展示属性，不绕过租户授权 |
| 组件资源 | `open_with=component`（兼容旧返回 `route`） | `path` 为站内路径，`component` 为 Vue 组件 |
| 内链资源 | `open_with=inside` | `path` 为站内路径，`component` 为 HTTP(S) 网站地址，iframe 展示 |
| 外链资源 | `open_with=outside` | `path` 为 HTTP(S) 网站地址；菜单新窗口打开，不注册 Vue 路由、不作为首页，不允许子节点 |

旧 PHP `Logic/User` 原样传递 `path/component/open_with`；旧前端 inside 使用 component 作为 frameSrc，outside 靠菜单识别 path URL 打开。旧 App 模型注释为 thrid，表单提交实际为 third。原应用切换未实现第三方分支，本轮补齐。

## 实现

- `src/utils/externalUrl.ts` 统一浏览器 URL 校验和安全链接：只接受显式 HTTP(S)、有效主机，不接受相对/协议相对地址、恶意 scheme、userinfo、反斜杠、控制字符、内部空白；保留配置地址原有查询串，不追加平台 token、tenant/app ID 等信息。
- 工作台应用卡片与顶栏应用列表识别第三方后同步点击链接，以 `_blank`、`noopener noreferrer`、`no-referrer` 打开，不走平台请求客户端，不切 SaaS 上下文。
- 侧栏菜单、顶栏菜单、菜单搜索、工作台快捷导航共用外部导航边界；outside 被动态路由转换过滤。修正顶层 inside 曾把 LAYOUT 函数当 frameSrc 的问题；iframe 元信息只留在真实子路由。
- iframe 使用 `referrerpolicy=no-referrer`，sandbox 仅允许脚本、表单和弹窗，不开放 same-origin/顶层导航；固定显示新窗口入口。外站的 frame 拒绝策略不能由跨域前端可靠识别，加载状态最长 10 秒；网站需要存储/同源能力或拒绝嵌入时使用新窗口入口。
- 第三方开通无需站内资源，授权页使用独立应用勾选状态发送空 `ids`。已有第三方开通固定勾选，可调整有效期，撤权沿用授权列表，避免取消勾选却被空资源提交重新开通。现有开通记录分页读取，完整回填有效期。两个成员默认应用下拉过滤第三方。
- 应用列表/详情兼容 unknown 类型及旧 thrid，不访问不存在的 typeMap 导致渲染报错。应用表单恢复 type/url/is_public。

## 通用附件 API

`src/api/app/file.ts` 提供以下函数，不新增界面：

- `uploadAttachmentApi(params, progress?)`：沿用上传接口，强制 multipart `purpose=attachment`，返回体 `data` 仍为 object key；上传结果模型包含可选顶层 file 元信息。
- `getFileLimits()`：GET `/app/file/limits`，返回 `image_max_bytes`、`attachment_max_bytes`。
- `getAttachmentBlob(key)`：GET `/app/file/download?key=...`，responseType Blob，通过现有拦截器发送 access-token/tenant-id/app-id 请求头。
- `downloadAttachment(key, filename?)`：优先解析 Content-Disposition `filename*`，兼容带引号/不带引号 filename，通过本地 Blob URL 下载，清除文件名中的路径分隔符和控制字符。不拼公开静态文件 URL，不在 URL 中放鉴权信息。

## 验证与后续浏览器回归

- `yarn esno tests/externalUrl.test.ts` 已通过：安全 URL、恶意 scheme、协议相对地址、userinfo、控制字符、旧拼写以及链接目标、referrer/rel 和地址不追加参数。
- 全量 `yarn type:check` 已通过（24.79 秒）。
- `yarn build` 通过，耗时 51.08 秒（Vite 48.78 秒）；记录 `.local/third-party-frontend-build.log`。
- 隔离浏览器应回归：第三方新建/编辑/开通/有效期/撤权；点击外部应用不改变当前上下文；inside 直接路由与嵌套路由显示；outside 菜单、搜索、快捷导航均新开；站点拒绝 iframe 时新窗口可用；非法 URL 被表单阻止；默认应用不出现第三方。

主 agent 浏览器后续发现 AppLogo 硬编码 `/dashboard` 并修复为 `go()`；`useGo` 实时读取当前授权首页，不缓存初始化时的旧首页。固定 tab key 使用 `fullPath || path`，affix 通过 router.resolve 生成完整位置。未保留路由后缀猜测回退。
