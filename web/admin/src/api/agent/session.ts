import { useGlobSetting } from '/@/hooks/setting';
import { getAuthHeaders } from '/@/utils/http/axios/requestHeaders';

enum Api {
  Sessions = '/api/agent/sessions',
  Context = '/context',
  Messages = '/messages',
}

const { apiUrl = '' } = useGlobSetting();

function agentRequest(path: string, init: RequestInit = {}) {
  const headers = new Headers(getAuthHeaders());
  new Headers(init.headers || {}).forEach((value, key) => headers.set(key, value));
  if (init.body != null && !headers.has('content-type')) headers.set('content-type', 'application/json');
  return fetch(`${apiUrl}${path}`, { ...init, headers });
}

async function readResult<T>(response: Response, fallback: string): Promise<T> {
  const body = await response.json().catch(() => ({}));
  if (!response.ok || body.code !== 0) {
    const error = new Error(body.msg || fallback) as Error & { status?: number; code?: number };
    error.status = response.status;
    error.code = body.code;
    throw error;
  }
  return body.data as T;
}

export interface AgentSessionSummary {
  session_id: string;
  title: string;
  message_count: number;
  updated_at: string;
}

export function listAgentSessions(signal?: AbortSignal) {
  return agentRequest(Api.Sessions, { signal }).then((response) =>
    readResult<AgentSessionSummary[]>(response, '读取 Agent 对话列表失败'),
  );
}

export function getAgentSession(id: string, signal?: AbortSignal) {
  return agentRequest(`${Api.Sessions}/${encodeURIComponent(id)}`, { signal }).then((response) =>
    readResult<any>(response, '读取 Agent 会话失败'),
  );
}

export function createAgentSession(context: unknown, signal?: AbortSignal) {
  return agentRequest(Api.Sessions, {
    method: 'POST',
    body: JSON.stringify({ context }),
    signal,
  }).then((response) => readResult<any>(response, 'Agent 会话创建失败'));
}

export function updateAgentSessionContext(id: string, context: unknown, signal?: AbortSignal) {
  return agentRequest(`${Api.Sessions}/${encodeURIComponent(id)}${Api.Context}`, {
    method: 'POST',
    body: JSON.stringify({ context }),
    signal,
  }).then((response) => readResult<any>(response, '更新页面上下文失败'));
}

/** Streaming is intentionally exposed as a Response so the component can consume SSE chunks. */
export function streamAgentMessage(id: string, message: string, signal?: AbortSignal) {
  return agentRequest(`${Api.Sessions}/${encodeURIComponent(id)}${Api.Messages}`, {
    method: 'POST',
    body: JSON.stringify({ message }),
    signal,
  });
}
