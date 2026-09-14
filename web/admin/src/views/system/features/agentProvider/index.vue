<template>
  <PageWrapper title="Agent Provider">
    <a-card>
      <template #title>Provider 配置</template>
      <template #extra><a-button type="primary" @click="openCreate">新增 Provider</a-button></template>
      <a-table :data-source="providers" :columns="columns" :pagination="false" row-key="_id" bordered>
        <template #bodyCell="{ column, record, index }">
          <template v-if="column.key === 'name'"><b>{{ record.name || record.provider }}</b><div class="muted">{{ record.code || '未设置标识' }}</div></template>
          <template v-else-if="column.key === 'type'">{{ record.provider }}</template>
          <template v-else-if="column.key === 'endpoint'"><span class="endpoint">{{ record.endpoint }}</span></template>
          <template v-else-if="column.key === 'model'">{{ record.model || '未设置' }}</template>
          <template v-else-if="column.key === 'status'"><a-tag :color="record.enabled ? 'green' : 'default'">{{ record.enabled ? '启用' : '停用' }}</a-tag><a-tag v-if="record.default" color="blue">默认</a-tag></template>
          <template v-else-if="column.key === 'models'">{{ (record.models || []).length }} 个模型</template>
          <template v-else-if="column.key === 'action'"><a-space><a-button type="link" @click="testModel(record)">测试连通</a-button><a-button type="link" @click="openEdit(index)">编辑</a-button><a-button type="link" danger @click="removeProvider(index)">删除</a-button></a-space></template>
        </template>
      </a-table>
      <a-empty v-if="!providers.length" description="暂无 Provider，请先新增" />
    </a-card>
    <a-drawer v-model:visible="drawerOpen" :title="editingIndex < 0 ? '新增 Provider' : '编辑 Provider'" width="520">
      <a-form layout="vertical" :model="draft">
        <a-form-item label="名称"><a-input v-model:value="draft.name" placeholder="例如 OpenAI 主账号" /></a-form-item>
        <a-form-item label="标识"><a-input v-model:value="draft.code" placeholder="例如 openai-main" /></a-form-item>
        <a-form-item label="服务商类型"><a-select v-model:value="draft.provider" :options="providerOptions" /></a-form-item>
        <a-form-item label="接口地址"><a-input v-model:value="draft.endpoint" placeholder="https://api.openai.com" /></a-form-item>
        <a-form-item label="API 密钥"><a-input-password v-model:value="draft.api_key" placeholder="留空保持原密钥" /></a-form-item>
        <a-form-item label="模型"><a-select v-model:value="draft.model" :options="modelOptions" placeholder="先同步模型" /></a-form-item>
        <a-form-item label="状态"><a-switch v-model:checked="draft.enabled" /></a-form-item>
        <a-form-item label="租户默认 Provider"><a-switch v-model:checked="draft.default" /></a-form-item>
        <a-form-item label="已同步模型"><a-select v-model:value="selectedModel" :options="modelOptions" placeholder="选择模型后测试" /></a-form-item>
      </a-form>
      <template #footer><a-space><a-button @click="syncModels" :loading="syncing">同步模型</a-button><a-button @click="testDraft" :loading="testing">测试模型</a-button><a-button type="primary" @click="save">保存</a-button></a-space></template>
    </a-drawer>
  </PageWrapper>
</template>
<script lang="ts">
import { defineComponent, reactive, ref, computed, onMounted } from 'vue';
import { Card, Table, Button, Space, Tag, Empty, Drawer, Form, Input, InputPassword, Select, Switch, message } from 'ant-design-vue';
import { PageWrapper } from '/@/components/Page';
import { getInfo, saveData } from '/@/api/tenant/tenant';
import { getSaasConf, getToken } from '/@/utils/auth';
import { useGlobSetting } from '/@/hooks/setting';
export default defineComponent({ components: { PageWrapper, ACard: Card, ATable: Table, AButton: Button, ASpace: Space, ATag: Tag, AEmpty: Empty, ADrawer: Drawer, AForm: Form, AFormItem: Form.Item, AInput: Input, AInputPassword: InputPassword, ASelect: Select, ASwitch: Switch }, setup() {
  const providers = ref<any[]>([]); const saving = ref(false); const syncing = ref(false); const testing = ref(false); const drawerOpen = ref(false); const editingIndex = ref(-1); const selectedModel = ref('');
  const draft = reactive<any>({ name: '', code: '', provider: 'openai', endpoint: '', api_key: '', model: '', models: [], enabled: true, default: false });
  const saasConf: any = getSaasConf() || {};
  const tenantId = Number(saasConf.tenantId || saasConf.tenant_id || 0);
  const { apiUrl = '' } = useGlobSetting();
  const agentUrl = (path: string) => `${apiUrl}${path}`;
  const columns = [{ title: '名称 / 标识', key: 'name' }, { title: '类型', key: 'type', width: 130 }, { title: '接口地址', key: 'endpoint', width: 220 }, { title: '默认模型', key: 'model', width: 150 }, { title: '状态', key: 'status', width: 120 }, { title: '支持模型', key: 'models', width: 100 }, { title: '操作', key: 'action', width: 230 }];
  // The gateway currently implements the OpenAI-compatible models and chat
  // completions protocol. Keep the form aligned with the supported backend
  // until protocol-specific adapters are added.
  const providerOptions = [{ value: 'openai', label: 'OpenAI 兼容' }];
  const modelOptions = computed(() => (draft.models || []).map((m: any) => ({ value: m.id || m, label: m.name || m.id || m })));
  const headers = () => { const token: any = getToken() || {}; return { 'Content-Type': 'application/json', 'access-token': token.access_token || '', 'tenant-id': String(tenantId) }; };
  const openCreate = () => { editingIndex.value = -1; Object.assign(draft, { name: '', code: '', provider: 'openai', endpoint: '', api_key: '', model: '', models: [], enabled: true, default: !providers.value.length }); selectedModel.value = ''; drawerOpen.value = true; };
  const openEdit = (index: number) => { editingIndex.value = index; Object.assign(draft, JSON.parse(JSON.stringify(providers.value[index]))); selectedModel.value = draft.model; drawerOpen.value = true; };
  const removeProvider = (index: number) => { providers.value.splice(index, 1); if (providers.value.length && !providers.value.some((p) => p.default)) providers.value[0].default = true; };
  const syncModels = async () => { syncing.value = true; try { const res = await fetch(agentUrl('/api/agent/provider/models'), { method: 'POST', headers: headers(), body: JSON.stringify({ provider: draft.provider, endpoint: draft.endpoint, api_key: draft.api_key }) }); const body = await res.json(); if (!res.ok || body.code !== 0) throw new Error(body.msg || '模型同步失败'); draft.models = body.data; if (!draft.model && body.data[0]) draft.model = body.data[0].id; message.success(`已同步 ${body.data.length} 个模型`); } catch (error: any) { message.error(error?.message || '模型同步失败'); } finally { syncing.value = false; } };
  const testDraft = async () => { testing.value = true; try { const res = await fetch(agentUrl('/api/agent/provider/test'), { method: 'POST', headers: headers(), body: JSON.stringify({ ...draft, model: selectedModel.value || draft.model }) }); const body = await res.json(); if (!res.ok || body.code !== 0) throw new Error(body.msg || '模型连通性测试失败'); message.success(`模型 ${selectedModel.value || draft.model} 连通正常`); } catch (error: any) { message.error(error?.message || '模型连通性测试失败'); } finally { testing.value = false; } };
  const testModel = (record: any) => { Object.assign(draft, JSON.parse(JSON.stringify(record))); selectedModel.value = record.model; editingIndex.value = providers.value.indexOf(record); drawerOpen.value = true; void testDraft(); };
  const save = async () => { saving.value = true; try { const value = JSON.parse(JSON.stringify(draft)); const targetIndex = editingIndex.value < 0 ? providers.value.length : editingIndex.value; if (editingIndex.value < 0) providers.value.push({ ...value, _id: `${Date.now()}` }); else providers.value[editingIndex.value] = value; let defaultIndex = value.default ? targetIndex : providers.value.findIndex((p, i) => i !== targetIndex && p.enabled && p.default); if (defaultIndex < 0) defaultIndex = providers.value.findIndex((p) => p.enabled); providers.value.forEach((p, i) => { p.default = defaultIndex >= 0 && i === defaultIndex; }); await saveData({ id: tenantId, name: '当前租户', agent_providers: JSON.stringify(providers.value) }); drawerOpen.value = false; } catch (error: any) { message.error(error?.message || 'Provider 配置保存失败'); } finally { saving.value = false; } };
  onMounted(async () => { try { const value: any = await getInfo({ id: tenantId }); providers.value = JSON.parse(value.agent_providers || '[]'); } catch (error: any) { providers.value = []; message.error(error?.message || 'Provider 配置加载失败'); } });
  return { providers, saving, syncing, testing, drawerOpen, editingIndex, draft, columns, providerOptions, modelOptions, selectedModel, openCreate, openEdit, removeProvider, syncModels, testDraft, testModel, save };
} });
</script>
<style scoped>
.muted { color: #8c8c8c; font-size: 12px; margin-top: 4px; }
.endpoint { display: inline-block; max-width: 210px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
