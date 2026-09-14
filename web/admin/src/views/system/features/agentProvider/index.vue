<template>
  <div>
    <BasicTable @register="registerTable">
      <template #toolbar>
        <Button type="primary" @click="handleCreate"
          ><Icon icon="ant-design:plus-outlined" />新增模型提供商</Button
        >
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'"
          ><b>{{ record.name || record.provider }}</b
          ><div class="muted">{{ record.code || '未设置标识' }}</div></template
        >
        <template v-else-if="column.key === 'provider'">{{
          providerLabel(record.provider)
        }}</template>
        <template v-else-if="column.key === 'endpoint'"
          ><span class="endpoint">{{ record.endpoint || '未设置' }}</span></template
        >
        <template v-else-if="column.key === 'model'">{{ record.model || '未设置' }}</template>
        <template v-else-if="column.key === 'status'"
          ><Tag :color="record.enabled ? 'green' : 'default'">{{
            record.enabled ? '启用' : '停用'
          }}</Tag
          ><Tag v-if="record.default" color="blue">默认</Tag></template
        >
        <template v-else-if="column.key === 'models'"
          >{{ (record.models || []).length }} 个模型</template
        >
        <template v-else-if="column.key === 'action'"
          ><TableAction
            :actions="[
              {
                label: '测试连通',
                icon: 'ant-design:api-outlined',
                onClick: handleTest.bind(null, record),
              },
              {
                label: '编辑',
                icon: 'clarity:note-edit-line',
                onClick: handleEdit.bind(null, record),
              },
              {
                label: '删除',
                icon: 'ant-design:delete-outlined',
                color: 'error',
                popConfirm: {
                  title: '是否确认删除',
                  placement: 'left',
                  confirm: handleDelete.bind(null, record),
                },
              },
            ]"
        /></template>
      </template>
    </BasicTable>
    <EditForm @register="registerDrawer" @success="handleSuccess" />
  </div>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { Button, Tag, message } from 'ant-design-vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { Icon } from '/@/components/Icon';
  import { useDrawer } from '/@/components/Drawer';
  import { getInfo, saveData } from '/@/api/tenant/tenant';
  import { getSaasConf } from '/@/utils/auth';
  import EditForm from './form.vue';
  import { columns, searchFormSchema } from './data';

  export default defineComponent({
    name: 'ModelProviderList',
    components: { BasicTable, TableAction, Button, Tag, Icon, EditForm },
    setup() {
      const providers = ref<any[]>([]);
      const saasConf: any = getSaasConf() || {};
      const tenantId = Number(saasConf.tenantId || saasConf.tenant_id || 0);
      const [registerDrawer, { openDrawer }] = useDrawer();
      const loadProviders = async (params: any = {}) => {
        const keyword = String(params.keyword || '')
          .trim()
          .toLowerCase();
        return providers.value.filter(
          (provider) =>
            !keyword ||
            [provider.name, provider.code, provider.endpoint, provider.provider].some((value) =>
              String(value || '')
                .toLowerCase()
                .includes(keyword),
            ),
        );
      };
      const [registerTable, { reload }] = useTable({
        title: '模型提供商列表',
        api: loadProviders,
        columns,
        formConfig: { labelWidth: 90, schemas: searchFormSchema },
        useSearchForm: true,
        showTableSetting: true,
        bordered: true,
        showIndexColumn: false,
        actionColumn: { width: 250, title: '操作', dataIndex: 'action', fixed: undefined },
      });
      function providerLabel(provider: string) {
        return provider === 'openai' ? 'OpenAI 兼容' : provider || '未设置';
      }
      function handleCreate() {
        openDrawer(true, { isUpdate: false, providers: providers.value });
      }
      function handleEdit(record: Recordable) {
        openDrawer(true, { isUpdate: true, record, providers: providers.value });
      }
      function handleTest(record: Recordable) {
        openDrawer(true, { isUpdate: true, record, providers: providers.value, autoTest: true });
      }
      async function handleDelete(record: Recordable) {
        providers.value = providers.value.filter((provider) => provider !== record);
        if (providers.value.length && !providers.value.some((provider) => provider.default))
          providers.value[0].default = true;
        await persist();
        reload();
      }
      async function persist() {
        await saveData({
          id: tenantId,
          name: '当前租户',
          agent_providers: JSON.stringify(providers.value),
        });
      }
      function handleSuccess(nextProviders: any[]) {
        providers.value = nextProviders;
        reload();
      }
      getInfo({ id: tenantId })
        .then((value: any) => {
          try {
            providers.value = JSON.parse(value?.agent_providers || '[]');
          } catch {
            providers.value = [];
          }
          reload();
        })
        .catch((error: any) => message.error(error?.message || '模型提供商配置加载失败'));
      return {
        registerTable,
        registerDrawer,
        handleCreate,
        handleEdit,
        handleTest,
        handleDelete,
        handleSuccess,
        providerLabel,
      };
    },
  });
</script>
<style scoped>
  .muted {
    color: #8c8c8c;
    font-size: 12px;
    margin-top: 4px;
  }
  .endpoint {
    display: inline-block;
    max-width: 210px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
