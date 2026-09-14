# 前端接口请求规范

## 基本规则

所有业务接口必须在 `web/admin/src/api` 下定义请求方法，页面和组件只调用语义化方法，不直接拼接 URL、请求头或鉴权参数。

```ts
// src/api/basic/example.ts
import { defHttp } from '/@/utils/http/axios';

enum Api {
  Query = '/system/example/query',
}

export function getExampleList(params?: Record<string, unknown>) {
  return defHttp.get({ url: Api.Query, params }, { errorMessageMode: 'none' });
}
```

```ts
// 页面或组件
import { getExampleList } from '/@/api/basic/example';

const rows = await getExampleList(params);
```

## 公共请求上下文

`access-token`、`tenant-id`、`unit-id`、`section-id`、`app-id` 和 JSON Content-Type 由 Axios 请求拦截器统一注入。页面、表单和 API 方法不得从 `getToken()`/`getSaasConf()` 手工拼接这些 header，也不得为了补鉴权而使用裸 `fetch`。

例外只有浏览器原生流式协议（当前 Agent SSE）。这类请求必须封装在 `src/api` 的专用方法中，由共享的 `getAuthHeaders()` 提供公共上下文；组件只能消费返回的 `Response` 流，不得自行构造 URL 或 header。

## 响应和错误

- API 方法负责声明 URL、HTTP 方法、参数位置和返回类型。
- 默认使用统一响应转换和错误处理；页面需要自定义提示时使用 `errorMessageMode: 'none'` 后再展示业务文案。
- 页面不解析统一 `{ code, data, msg }` 包络，API 方法应返回转换后的 `data`。
- SSE API 方法只负责建立请求和返回流；事件解析和界面状态更新属于 Agent 组件。

## 审查规则

提交前使用以下命令检查业务页面是否绕过规范：

```sh
rg -n "fetch\\(|axios\\.|defHttp\\.(get|post|put|delete).*url:|access-token|tenant-id|app-id" web/admin/src/views web/admin/src/layouts
```

页面中出现上述内容时，必须能说明它是通用组件实现或已封装的 SSE 例外；新增业务接口应先补充 `src/api` 方法。
