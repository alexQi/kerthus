# 页面 Agent Gateway

Gateway 从登录态解析用户、租户与应用，按当前权限注入工具。当前 Provider 使用 OpenAI 兼容 Chat Completions 协议。

## 多轮会话

- `POST /api/agent/sessions` 创建会话，请求体可带 `context`。
- `GET /api/agent/sessions/{session_id}` 读取会话及历史，仅对应用户、租户、应用可访问。
- `POST /api/agent/sessions/{session_id}/context` 更新脱敏页面上下文。
- `POST /api/agent/sessions/{session_id}/messages` 执行一个会话轮次。
- `GET /api/agent/tools` 查看当前可用工具。

执行 `make migrate` 应用 `010_agent_sessions.sql` 后，Gateway 使用已有 MySQL 配置保存 `agent_sessions`，不需要新增数据库。完成轮次包含 user、assistant、tool_calls、tool_call_id 和工具结果，供后续轮次使用。Runtime 每次按最新 Provider 和权限创建，显式装载该会话的历史；没有按固定 Agent 名称共享记忆。

会话闲置 30 天过期；历史最多保留最近 20 个完整轮次、128 条消息、512 KiB。裁剪以完整轮次为单位，工具调用与结果不会被拆开。MySQL 行锁和两分钟生成租约防止多个网关实例并发覆盖同一会话。服务端执行限时 90 秒；失败、取消、截断不会作为完成轮次保存，也不会自动重试已经执行过的工具。

前端仅在 localStorage 保存按用户、租户、应用区分的会话 ID，刷新后从服务器恢复内容。点击“新对话”可开始独立会话。权限集变化时会清除旧上下文历史；每次工具执行仍会重新检查当前权限。

## 真实 SSE 与工具调用

go-micro v6.13.0 的 `agent.StreamAsk` 先执行 `Generate`，再切分最终文本。Gateway 使用本地流式执行适配器，沿用 go-micro 的 `ai.Tool`、工具处理器与 `agent.StreamEvent` 契约：

1. 每轮都发送 `stream: true` 和本轮允许的工具定义。
2. `delta.content` 到达后立即发送 `token` SSE，保留换行和空格以渲染 Markdown。
3. 按 index 聚合工具参数分片，收到完整工具调用结束标记后才执行工具。
4. 发送 `tool_start`、`tool_end`，将调用和结果加入协议消息，继续请求上游流。
5. 正常完成并成功写入 MySQL 后才发送 `done`；失败发送 `error`，前端不能把断流误判为完成。

模型流必须返回正常 finish_reason 和 `[DONE]`。空内容、错误帧、无结束标记、截断、工具参数错误都会终止该轮。上游响应与工具参数有大小上限；未知工具、过多调用和重复循环被限制。浏览器取消请求会关闭上游连接。

协议参考：[OpenAI Function calling](https://developers.openai.com/api/docs/guides/function-calling)。

## Provider 模型测试

`POST /api/agent/provider/test` 不再以 HTTP 200 作为成功标准。请求指定模型进行实际文本生成，并读取完整响应：SSE 需非空文本、`finish_reason=stop` 和 `[DONE]`；兼容服务返回 JSON 时，需完整 assistant 文本及正常结束状态。

测试限时 45 秒，最大响应 1 MiB，禁止携带凭据跟随重定向。成功返回模型名、耗时和输出字符数；上游错误正文与凭据不回传给浏览器。模型测试不写入对话历史。

## 验证

- `go test ./internal/agent ./internal/gateway`：真实 token 时序、工具参数分片与多轮协议、HTTP SSE、取消、并发、错误与空响应。
- 设置测试环境变量 `KERTHUS_AGENT_TEST_DSN` 后执行 `go test ./internal/agent -run TestMySQLSessionSurvivesReopenAndSharesLease -v`：验证数据库连接重建后恢复历史与跨实例租约；只创建并清理随机测试会话。
- `cd web/admin && npm run type:check && npm run build`。

页面上下文视为不可信输入，身份与权限始终来自服务端。破坏性工具的一次性人工确认令牌、Provider 密钥加密存储属于单独的安全增强工作。
