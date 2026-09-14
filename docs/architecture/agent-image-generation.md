# AI 助手多模态生成方案

## 目标

AI 助手根据当前登录用户、租户、应用和权限工作。除文本问答外，Provider 支持的图片模型可以通过受权限控制的工具生成图片。生成结果进入当前应用维度的会话历史，并通过 SSE 实时反馈状态。

## 边界和原则

- Provider 管理权限与图片生成权限分离。能管理模型提供商不等于能调用图片生成；图片工具每次调用都重新校验租户、应用和用户权限。
- Agent 只使用服务端根据当前应用权限筛选后的工具。客户端传入的应用、Provider、模型和权限信息不可信。
- 图片是结构化 artifact，不是模型返回的裸 URL、Base64 或 Markdown 图片链接。
- 生成文件存入现有私有文件存储，下载继续经过租户、应用、用户和会话权限校验。
- 先建立统一 Provider 适配接口，再实现各家协议；不能把图片请求混入现有 Chat Completions 文本流。

## 数据模型

### Provider 模型能力

Provider 同步模型时保存显式能力字段，模型列表接口没有能力字段时不根据名称猜测：

```json
{
  "id": "gpt-image-2",
  "name": "GPT Image 2",
  "capabilities": ["chat", "image_generation"],
  "enabled": true,
  "default": false
}
```

模型能力支持服务端配置覆盖、同步版本、去重和并发更新保护。只有已同步且启用 `image_generation` 的模型才能被图片工具调用。

### Artifact

新增 `agent_artifacts` 关联记录，至少包含：`id`、`session_id`、`file_id`、`tenant_id`、`app_id`、`user_id`、`provider_id`、`model`、`status`、`mime_type`、`size`、`width`、`height`、`metadata_json`、`created_at`、`expires_at`。

会话消息只保存 artifact 元数据和 ID，不保存 Base64；恢复历史时按权限解析为受保护的下载地址。生成失败、取消和超时的临时对象要清理。

## 后端分层

### ProviderAdapter

定义统一的图片生成接口，输入包含模型、提示词、尺寸、质量、数量、幂等键和取消上下文，输出图片字节或受控的上游结果引用。

- 首期实现 OpenAI 兼容 `POST /v1/images/generations`。
- 后续扩展 Responses、Gemini 等协议时新增 adapter，不改变 Agent 工具协议。
- 所有请求使用 Provider 服务端配置，禁止工具参数覆盖 endpoint、API key 或任意 URL。

### ArtifactService

负责权限、配额、并发、幂等、上游结果获取和私有存储：

1. 校验当前用户在当前应用的图片权限、Provider、模型能力和资源限制。
2. 生成带租户、应用、会话和轮次的幂等键；相同请求恢复已有结果。
3. 调用上游并限制超时、重定向、响应大小、MIME、像素数和真实图片解码结果，防止 SSRF 和伪造内容。
4. 将图片写入现有私有文件服务，创建 `agent_artifacts` 记录。
5. 返回不含密钥和上游地址的 artifact 引用。

图片生成使用独立的并发数、每日配额和审计记录。不能直接套用聊天请求的自动重试，除非上游支持并识别幂等键；遇到 `Retry-After` 时遵守服务端上限。

### Agent tool

新增 `image_generate` 工具，描述包含提示词、模型、尺寸、质量和数量等允许参数。工具只返回结构化 `ToolOutcome`/artifact 引用，不能返回可执行 URL。工具注入范围是当前应用中拥有图片权限的用户；Provider 管理工具仍只在模型提供商管理场景注入。

## SSE 协议

在现有 `token`、`tool_start`、`tool_end`、`done`、`error` 之外增加：

- `artifact_start`：开始生成，包含 artifact ID、模型和状态。
- `artifact_progress`：可选进度或阶段信息。
- `artifact_ready`：包含 artifact ID、受保护的展示信息和附件元数据。
- `artifact_error`：包含可展示的错误码和恢复建议。
- `artifact_cancelled`：浏览器取消或服务端超时。

文本 Runtime 继续处理 token；网关负责把结构化工具结果转换成 artifact SSE 事件。生成结果持久化成功后再发 `done`。断线重连按 artifact ID 去重，长任务增加心跳和取消传播。

## 前端行为

- 消息模型增加稳定的消息 ID、附件数组、artifact 状态和错误信息；历史恢复不能丢失图片。
- 图片使用专用附件卡片渲染，Markdown 只渲染文本；禁止直接信任模型生成的外链、`data:` URI 或 HTML 图片。
- 生成中展示占位、阶段状态和停止按钮；成功显示预览和下载，失败显示重试；重复 SSE 事件不重复插入。
- 当前应用切换时按 `tenant_id + app_id` 读取会话列表和工具上下文。
- 输入区、滚动到底部、长文本 Markdown 更新与图片加载状态保持现有 AI 助手交互约定。

## API 规划

- `GET /api/agent/tools`：返回当前应用权限过滤后的工具和能力摘要。
- `POST /api/agent/sessions/{id}/messages`：复用现有 SSE 入口，增加 artifact 事件。
- `GET /api/agent/artifacts/{id}`：校验会话和当前应用权限后返回临时下载或展示响应。
- `POST /api/agent/artifacts/{id}/retry`：仅对可安全重试的失败任务生成新的幂等轮次。
- Provider 的模型同步与连通性测试继续使用现有管理 API，但测试必须按模型能力选择文本或图片探针。

## 安全和运维

- 记录租户、应用、用户、Provider、模型、工具调用、耗时、状态和费用相关审计信息；禁止记录 API key 和完整提示词中的敏感内容。
- 下载响应使用私有缓存策略和正确的 `Content-Disposition`；CSP 的 `img-src` 只允许本系统来源。
- 设定单图字节、像素、数量、并发、超时和生命周期上限；后台任务清理过期 artifact 和孤儿对象。
- 所有破坏性或高成本动作预留人工确认扩展点。

## 实施顺序

1. 数据库迁移、模型能力字段和 artifact 表。
2. ProviderAdapter、ArtifactService、权限/配额/幂等和私有存储接入。
3. `image_generate` 工具及当前应用权限过滤。
4. SSE artifact 生命周期、取消、心跳和断线去重。
5. 前端附件消息、历史恢复、预览下载、失败重试和视觉验收。
6. OpenAI 兼容 Provider 端到端测试，再补其他 Provider adapter。

## 验收标准

- 在支持图片模型且有权限的当前应用中，文本请求可以调用 `image_generate` 并得到真实图片。
- 切换应用后只能看到该应用的会话、工具和 artifact；跨租户、跨应用、无权限下载均被拒绝。
- 刷新或断线重连后图片和状态可恢复，SSE 不重复渲染；取消会停止上游并清理临时对象。
- Provider 返回错误、超时、非法 MIME、超大图片、重复请求和上游重试均有明确结果。
- 现有文本 Agent、多轮历史、工具调用、Provider 管理和前端视觉回归测试全部通过。
