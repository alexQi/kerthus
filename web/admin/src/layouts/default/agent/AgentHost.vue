<template>
  <div>
    <a-button class="agent-trigger" type="primary" shape="circle" size="large" @click="open = true">
      <template #icon><MessageOutlined /></template>
    </a-button>
    <a-drawer v-model:visible="open" title="Agent 对话" placement="right" :width="drawerWidth" :body-style="{ padding: 0 }" :destroy-on-close="false">
      <template #extra><a-button type="link" :disabled="loading || sessionLoading" @click="newConversation">新对话</a-button></template>
      <div class="agent-layout">
        <aside class="agent-sessions">
          <div class="agent-sessions-header">
            <span>对话列表</span>
            <a-button type="link" size="small" :disabled="loading || sessionLoading" @click="newConversation">新对话</a-button>
          </div>
          <div v-if="!sessions.length" class="agent-sessions-empty">暂无历史对话</div>
          <button v-for="item in sessions" :key="item.session_id" type="button" :class="['agent-session-item', { active: item.session_id === sessionId }]" @click="selectSession(item)">
            <span class="agent-session-title">{{ item.title }}</span>
            <span class="agent-session-meta">{{ item.message_count ? `${Math.ceil(item.message_count / 2)} 轮对话 · ` : '' }}{{ formatSessionTime(item.updated_at) }}</span>
          </button>
        </aside>
        <section class="agent-chat">
          <div class="agent-history-title">当前对话</div>
          <div ref="messageContainer" class="agent-messages">
            <a-empty v-if="!messages.length" description="可以询问当前页面的数据和操作" />
            <div v-for="(item, index) in messages" :key="index" :class="['agent-message-row', item.role]">
              <div class="agent-message-label">{{ item.role === 'user' ? '你' : item.role === 'assistant' ? 'Agent' : '系统' }}</div>
          <div class="agent-message">
            <div v-if="item.role === 'assistant' && index === messages.length - 1 && !item.text && loading" class="agent-thinking"><LoadingOutlined spin /><span>{{ progress }}</span></div>
            <template v-else-if="item.role === 'assistant'">
              <MarkdownViewer :value="item.text" class="agent-markdown" />
              <span v-if="loading && index === messages.length - 1" class="agent-inline-thinking"><LoadingOutlined spin /></span>
            </template>
            <span v-else>{{ item.text }}</span>
              </div>
            </div>
          </div>
        </section>
      </div>
      <template #footer>
        <a-input-search v-model:value="draft" enter-button="发送" :loading="loading" @search="send" />
      </template>
    </a-drawer>
  </div>
</template>

<script lang="ts">
  import { computed, defineComponent, nextTick, onBeforeUnmount, ref, watch } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { Button, Drawer, Empty, InputSearch } from 'ant-design-vue';
  import { LoadingOutlined, MessageOutlined } from '@ant-design/icons-vue';
  import { MarkdownViewer } from '/@/components/Markdown';
  import { getSaasConf, getToken } from '/@/utils/auth';
  import { useGlobSetting } from '/@/hooks/setting';
  import 'vditor/dist/index.css';

  type SessionSummary = { session_id: string; title: string; message_count: number; updated_at: string };

  export default defineComponent({
    name: 'AgentHost',
    components: { AButton: Button, ADrawer: Drawer, AEmpty: Empty, AInputSearch: InputSearch, LoadingOutlined, MessageOutlined, MarkdownViewer },
    setup() {
      const route = useRoute();
      const router = useRouter();
      const open = ref(false);
      const drawerWidth = computed(() => (window.innerWidth >= 1200 ? '70%' : window.innerWidth >= 768 ? 720 : '100%'));
      const loading = ref(false);
      const draft = ref('');
      const sessionId = ref('');
      const sessionScope = ref('');
      const sessionPermissionVersion = ref('');
      const sessionLoading = ref(false);
      const sessionController = ref<AbortController | null>(null);
      const progress = ref('正在生成…');
      const messageContainer = ref<HTMLElement | null>(null);
      const activeController = ref<AbortController | null>(null);
      const messages = ref<{ role: string; text: string }[]>([]);
      const sessions = ref<SessionSummary[]>([]);
      const getSessionScope = () => {
        const conf: any = getSaasConf() || {};
        const token: any = getToken() || {};
        return `${token.user_id || 0}:${conf.tenantId || conf.tenant_id || token.tenant_id || 0}:${conf.appId || conf.app_id || token.app_id || 0}`;
      };
      const headers = () => {
        const conf: any = getSaasConf() || {};
        const token: any = getToken() || {};
        return {
          'Content-Type': 'application/json',
          'access-token': token.access_token || '',
          'tenant-id': String(conf.tenantId || conf.tenant_id || token.tenant_id || 0),
          'app-id': String(conf.appId || conf.app_id || token.app_id || 0),
        };
      };
      const { apiUrl = '' } = useGlobSetting();
      const agentUrl = (path: string) => `${apiUrl}${path}`;
      const context = () => ({ version: 'page_context.v1', route: window.location.hash.replace(/^#/, '') || route.fullPath, title: String(route.meta?.title || route.name || ''), locale: navigator.language, available_routes: router.getRoutes().map((item) => item.path).filter(Boolean) });
      const storageKey = (scope: string) => `kerthus.agent.session.${scope}`;
      const scrollToBottom = () => {
        void nextTick(() => {
          const container = messageContainer.value;
          if (!container) return;
          container.scrollTop = container.scrollHeight;
          // MarkdownViewer updates its DOM after Vue's tick. A second frame
          // keeps the viewport pinned while streamed tokens are rendered.
          window.requestAnimationFrame(() => {
            const current = messageContainer.value;
            if (current) current.scrollTop = current.scrollHeight;
          });
        });
      };
      const rememberSession = () => {
        try { localStorage.setItem(storageKey(sessionScope.value), sessionId.value); } catch { /* storage may be unavailable */ }
      };
      const forgetSession = () => {
        try { localStorage.removeItem(storageKey(sessionScope.value || getSessionScope())); } catch { /* storage may be unavailable */ }
        sessionId.value = ''; sessionScope.value = ''; sessionPermissionVersion.value = '';
      };
      const restoreMessages = (history: any[] = []) => {
        messages.value = history
          .filter((item) => (item.role === 'user' || item.role === 'assistant') && typeof item.content === 'string' && item.content.trim())
          .map((item) => ({ role: item.role, text: item.content }));
        scrollToBottom();
      };
      const formatSessionTime = (value: string) => {
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) return '';
        return date.toLocaleString([], { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });
      };
      const refreshSessions = async (signal?: AbortSignal) => {
        const response = await fetch(agentUrl('/api/agent/sessions'), { headers: headers(), signal });
        const body = await response.json().catch(() => ({}));
        if (!response.ok || body.code !== 0) throw new Error(body.msg || '读取 Agent 对话列表失败');
        sessions.value = Array.isArray(body.data) ? body.data : [];
      };
      const loadSession = async (id: string) => {
        if (loading.value || sessionLoading.value || id === sessionId.value) return;
        sessionLoading.value = true;
        try {
          const response = await fetch(agentUrl(`/api/agent/sessions/${encodeURIComponent(id)}`), { headers: headers() });
          const body = await response.json().catch(() => ({}));
          if (!response.ok || body.code !== 0 || !body.data?.session_id) throw new Error(body.msg || '读取 Agent 会话失败');
          sessionId.value = body.data.session_id;
          sessionScope.value = getSessionScope();
          sessionPermissionVersion.value = body.data.permission_version || '';
          restoreMessages(body.data.history);
          rememberSession();
          scrollToBottom();
        } catch (error: any) {
          messages.value.push({ role: 'error', text: error?.message || '读取 Agent 会话失败' });
        } finally {
          sessionLoading.value = false;
        }
      };
      const selectSession = (item: SessionSummary) => { void loadSession(item.session_id); };
      let sessionRequest: Promise<void> | null = null;
      const ensureSession = () => {
        if (sessionRequest) return sessionRequest;
        sessionLoading.value = true;
        const controller = new AbortController();
        sessionController.value = controller;
        const timeout = window.setTimeout(() => controller.abort(), 15000);
        sessionRequest = (async () => {
          const scope = getSessionScope();
          if (sessionId.value && sessionScope.value === scope) return;
          sessionId.value = ''; sessionScope.value = scope; messages.value = [];
          let saved = '';
          try { saved = localStorage.getItem(storageKey(scope)) || ''; } catch { /* storage may be unavailable */ }
          if (saved) {
            const response = await fetch(agentUrl(`/api/agent/sessions/${encodeURIComponent(saved)}`), { headers: headers(), signal: controller.signal });
            const body = await response.json().catch(() => ({}));
            if (getSessionScope() !== scope) throw new Error('应用上下文已变更，请重新发送');
            if (response.ok && body.code === 0 && body.data?.session_id) {
              sessionId.value = body.data.session_id;
              sessionPermissionVersion.value = body.data.permission_version || '';
              restoreMessages(body.data.history);
              return;
            }
            if (body.code !== 404 && body.code !== 403 && response.status !== 404 && response.status !== 403) {
              throw new Error(body.msg || '读取 Agent 会话失败');
            }
            forgetSession(); sessionScope.value = scope;
          }
          const response = await fetch(agentUrl('/api/agent/sessions'), { method: 'POST', headers: headers(), body: JSON.stringify({ context: context() }), signal: controller.signal });
          const body = await response.json().catch(() => ({}));
          if (getSessionScope() !== scope) throw new Error('应用上下文已变更，请重新发送');
          if (!response.ok || body.code !== 0 || !body.data?.session_id) throw new Error(body.msg || 'Agent 会话创建失败');
          sessionId.value = body.data.session_id;
          sessionScope.value = scope;
          sessionPermissionVersion.value = body.data.permission_version || '';
          rememberSession();
        })().finally(() => {
          window.clearTimeout(timeout);
          if (sessionController.value === controller) sessionController.value = null;
          sessionLoading.value = false; sessionRequest = null;
        });
        return sessionRequest;
      };
      const syncContext = async (signal?: AbortSignal) => {
        if (!sessionId.value) return;
        if (sessionScope.value !== getSessionScope()) {
          activeController.value?.abort();
          sessionId.value = ''; sessionScope.value = ''; messages.value = [];
          return;
        }
        const response = await fetch(agentUrl(`/api/agent/sessions/${sessionId.value}/context`), { method: 'POST', headers: headers(), body: JSON.stringify({ context: context() }), signal });
        const body = await response.json().catch(() => ({}));
        if (!response.ok || body.code !== 0) {
          if (body.code === 404 || body.code === 403) forgetSession();
          throw new Error(body.msg || '更新页面上下文失败');
        }
        if (body.data?.permission_version !== sessionPermissionVersion.value) {
          sessionPermissionVersion.value = body.data?.permission_version || '';
          restoreMessages(body.data?.history);
        }
      };
      const newConversation = () => {
        forgetSession(); messages.value = []; draft.value = '';
      };
      watch(() => route.fullPath, () => { void syncContext().catch(() => undefined); });
      watch(messages, () => scrollToBottom(), { deep: true });
      watch(open, (visible) => {
        if (visible && !loading.value) void refreshSessions().catch(() => undefined).then(() => ensureSession()).then(() => syncContext()).then(() => refreshSessions()).catch((error) => {
          messages.value.push({ role: 'error', text: error.message || '读取 Agent 会话失败' });
        });
      });
      const appendAnswer = (answer: string) => {
        const current = messages.value[messages.value.length - 1];
        if (current?.role === 'assistant') current.text = answer;
        scrollToBottom();
      };

      const send = async () => {
        const text = draft.value.trim();
        if (!text || loading.value) return;
        loading.value = true; progress.value = '正在生成…';
        const controller = new AbortController();
        activeController.value = controller;
        controller.signal.addEventListener('abort', () => sessionController.value?.abort(), { once: true });
        const timeoutId = window.setTimeout(() => controller.abort(), 120000);
        try {
          await ensureSession();
          await syncContext(controller.signal);
          if (controller.signal.aborted) throw new DOMException('请求已取消', 'AbortError');
          draft.value = ''; messages.value.push({ role: 'user', text });
          const response = await fetch(agentUrl(`/api/agent/sessions/${sessionId.value}/messages`), { method: 'POST', headers: headers(), body: JSON.stringify({ message: text }), signal: controller.signal });
          if (!response.ok || !response.body || !response.headers.get('content-type')?.includes('text/event-stream')) {
            const body = await response.json().catch(() => ({}));
            if (body.code === 404) forgetSession();
            throw new Error(body.msg || 'Agent 请求失败');
          }
          const reader = response.body.getReader(); const decoder = new TextDecoder(); let answer = '';
          messages.value.push({ role: 'assistant', text: '' });
          let eventType = 'message'; let buffer = ''; let dataLines: string[] = []; let completed = false;

          const processEvent = () => {
            if (!dataLines.length) { eventType = 'message'; return; }
            const raw = dataLines.join('\n');
            dataLines = [];
            const type = eventType; eventType = 'message';
            let event: any;
            try { event = JSON.parse(raw); } catch { throw new Error('Agent 流式响应格式错误'); }
            if (type === 'error') throw new Error(typeof event === 'string' ? event : event.error || event.Error || 'Agent 请求失败');
            const actualType = event.Type || event.type || type;
            if (actualType === 'tool_start') progress.value = '正在调用工具…';
            if (actualType === 'tool_end') progress.value = '正在整理工具结果…';
            const token = event.Token || event.token || '';
            if (token) { answer += token; appendAnswer(answer); }
            if (actualType === 'done') {
              const finalAnswer = event.Response?.Reply || event.response?.Reply || event.Response?.Content || event.response?.Content;
              if (typeof finalAnswer === 'string' && finalAnswer) answer = finalAnswer;
              if (!answer.trim()) throw new Error('模型未返回内容，请重试');
              appendAnswer(answer); completed = true;
            }
          };
          const processLine = (line: string) => {
            const normalized = line.endsWith('\r') ? line.slice(0, -1) : line;
            if (!normalized) { processEvent(); return; }
            if (normalized.startsWith('event:')) eventType = normalized.slice(6).trim() || 'message';
            if (normalized.startsWith('data:')) dataLines.push(normalized.slice(5).trimStart());
          };
          try {
            while (true) {
              const part = await reader.read();
              if (part.done) break;
              buffer += decoder.decode(part.value, { stream: true });
              const lines = buffer.split('\n'); buffer = lines.pop() || '';
              lines.forEach(processLine);
            }
            buffer += decoder.decode();
            if (buffer) buffer.split('\n').forEach(processLine);
            processEvent();
            if (!completed) throw new Error('回复连接已中断，请重试');
            await refreshSessions().catch(() => undefined);
          } finally {
            await reader.cancel().catch(() => undefined);
            reader.releaseLock();
          }
        } catch (error: any) {
          const pending = messages.value[messages.value.length - 1];
          if (pending?.role === 'assistant' && !pending.text) messages.value.pop();
          const message = error?.name === 'AbortError' ? 'Agent 响应超时，请重试' : error?.message || 'Agent 请求失败';
          messages.value.push({ role: 'error', text: message });
        } finally {
          window.clearTimeout(timeoutId);
          if (activeController.value === controller) activeController.value = null;
          loading.value = false;
        }
      };
      onBeforeUnmount(() => {
        activeController.value?.abort();
        sessionController.value?.abort();
        activeController.value = null;
        sessionId.value = '';
        sessionScope.value = '';
      });
      return { open, drawerWidth, loading, sessionLoading, draft, messages, progress, messageContainer, sessions, sessionId, send, newConversation, selectSession, formatSessionTime };
    },
  });
</script>

<style scoped lang="less">
  .agent-trigger { position: fixed; right: 24px; bottom: 24px; z-index: 1000; box-shadow: 0 4px 16px rgb(0 0 0 / 20%); }
  .agent-layout { display: flex; height: 100%; min-height: 0; margin: 0; }
  .agent-sessions { width: 220px; flex: 0 0 220px; padding: 20px 16px; border-right: 1px solid #f0f0f0; overflow-y: auto; }
  .agent-sessions-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; color: var(--text-color-base, rgb(0 0 0 / 88%)); font-size: 13px; font-weight: 600; }
  .agent-sessions-empty { padding: 20px 8px; color: rgb(0 0 0 / 45%); font-size: 12px; text-align: center; }
  .agent-session-item { display: flex; flex-direction: column; width: 100%; margin-bottom: 4px; padding: 9px 10px; border: 0; border-radius: 8px; color: var(--text-color-base, rgb(0 0 0 / 88%)); background: transparent; cursor: pointer; text-align: left; transition: background 0.2s; }
  .agent-session-item:hover { background: #f5f5f5; }
  .agent-session-item.active { color: #1677ff; background: #e6f4ff; }
  .agent-session-title { overflow: hidden; font-size: 13px; line-height: 1.45; text-overflow: ellipsis; white-space: nowrap; }
  .agent-session-meta { margin-top: 3px; color: rgb(0 0 0 / 45%); font-size: 11px; }
  .agent-chat { display: flex; min-width: 0; flex: 1; flex-direction: column; padding: 20px 28px; }
  .agent-history-title { margin-bottom: 16px; color: var(--text-color-base, rgb(0 0 0 / 88%)); font-size: 15px; font-weight: 600; }
  .agent-messages { display: flex; min-height: 0; flex: 1; flex-direction: column; gap: 14px; overflow-y: auto; padding: 2px 4px 12px 0; }
  .agent-messages :deep(.vditor-reset) { padding: 0; background: transparent; font-size: 14px; line-height: 1.65; }
  .agent-messages :deep(.vditor-reset > :first-child) { margin-top: 0; }
  .agent-messages :deep(.vditor-reset > :last-child) { margin-bottom: 0; }
  .agent-message-row { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; }
  .agent-message-row.user { align-items: flex-end; }
  .agent-message-label { color: rgb(0 0 0 / 45%); font-size: 12px; line-height: 1.4; }
  .agent-message { max-width: 92%; padding: 10px 12px; white-space: pre-wrap; border-radius: 10px; background: #f5f5f5; overflow-wrap: anywhere; }
  .agent-thinking { display: inline-flex; align-items: center; gap: 8px; color: rgb(0 0 0 / 45%); }
  .agent-inline-thinking { display: inline-flex; margin-top: 6px; color: #1677ff; }
  .agent-message-row.assistant .agent-message { white-space: normal; }
  .agent-message-row.user .agent-message { color: #fff; background: #1677ff; }
  .agent-message-row.error .agent-message { color: #d4380d; background: #fff2e8; }
  html[data-theme='dark'] .agent-history-title { color: rgb(255 255 255 / 85%); }
  html[data-theme='dark'] .agent-sessions { border-right-color: #303030; }
  html[data-theme='dark'] .agent-sessions-header { color: rgb(255 255 255 / 85%); }
  html[data-theme='dark'] .agent-sessions-empty { color: rgb(255 255 255 / 45%); }
  html[data-theme='dark'] .agent-session-item { color: rgb(255 255 255 / 85%); }
  html[data-theme='dark'] .agent-session-item:hover { background: #262626; }
  html[data-theme='dark'] .agent-session-item.active { color: #69b1ff; background: #111d2c; }
  html[data-theme='dark'] .agent-session-meta { color: rgb(255 255 255 / 45%); }
  html[data-theme='dark'] .agent-message-label { color: rgb(255 255 255 / 45%); }
  html[data-theme='dark'] .agent-message { color: rgb(255 255 255 / 85%); background: #262626; }
  html[data-theme='dark'] .agent-thinking { color: rgb(255 255 255 / 45%); }
  html[data-theme='dark'] .agent-message-row.user .agent-message { color: #fff; background: #1677ff; }
  html[data-theme='dark'] .agent-message-row.error .agent-message { color: #ffb7b2; background: #2b1d1b; }
  @media (max-width: 560px) {
    .agent-sessions { width: 160px; flex-basis: 160px; padding: 16px 10px; }
    .agent-chat { padding: 16px; }
    .agent-trigger { right: 16px; bottom: 16px; }
  }
</style>
