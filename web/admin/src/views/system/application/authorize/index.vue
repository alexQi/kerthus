<template>
  <div>
    <BasicTable @register="registerTable">
      <template #toolbar>
        <a-button
          v-auth="['system:application:authorize:deauth']"
          type="danger"
          :disabled="!checkedKeys.length"
          @click="handleBatchDeauthorize"
        >
          取消授权
        </a-button>
        <a-button
          v-auth="['system:application:authorize:auth']"
          type="primary"
          @click="handleCreate"
        >
          授权
        </a-button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'expiration_time'">
          <Tag :color="record.expiration_time > 0 ? 'warning' : 'success'">
            {{
              record.expiration_time > 0
                ? formatToDateTime(record.expiration_time * 1000)
                : '永久有效'
            }}
          </Tag>
        </template>
        <template v-if="column.key === 'updated_at'">
          {{ formatToDateTime(record.updated_at * 1000) }}
        </template>
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                label: '取消授权',
                color: 'error',
                ifShow: hasPermission(['system:application:authorize:deauth']),
                popConfirm: {
                  title: '是否确认取消授权',
                  placement: 'left',
                  confirm: handleDeauthorize.bind(null, record),
                },
              },
            ]"
          />
        </template>
      </template>
    </BasicTable>
  </div>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { Tag } from 'ant-design-vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { deauthorizeApp, getList } from '/@/api/application/authorize';
  import { formatToDateTime } from '/@/utils/dateUtil';
  import { columns, searchFormSchema } from './data';
  import { Key } from 'ant-design-vue/lib/table/interface';
  import { useGo } from '/@/hooks/web/usePage';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'ApplicationAuthorizes',
    components: { BasicTable, TableAction, Tag },
    setup() {
      const go = useGo();
      const { hasPermission } = usePermission();
      const checkedKeys = ref<Key[]>([]);
      const [registerTable, { reload, clearSelectedRowKeys }] = useTable({
        title: '租户应用授权列表',
        api: getList,
        columns,
        formConfig: {
          labelWidth: 100,
          schemas: searchFormSchema,
        },
        rowKey: 'id',
        rowSelection: {
          type: 'checkbox',
          preserveSelectedRowKeys: true,
          onChange: (keys) => {
            checkedKeys.value = [...keys];
          },
        },
        useSearchForm: true,
        showTableSetting: true,
        bordered: true,
        showIndexColumn: false,
        actionColumn: {
          width: 120,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
          ifShow: hasPermission(['system:application:authorize:deauth']),
        },
      });

      function handleCreate() {
        go('/system/application/auth');
      }

      async function handleBatchDeauthorize() {
        if (!checkedKeys.value.length) return;
        const data = await deauthorizeApp({ tenant_app_ids: [...checkedKeys.value] });
        if (data) {
          checkedKeys.value = [];
          clearSelectedRowKeys();
          reload();
        }
      }

      async function handleDeauthorize(record: Recordable) {
        const data = await deauthorizeApp({ tenant_app_ids: [record.id] });
        if (data) {
          checkedKeys.value = [];
          clearSelectedRowKeys();
          reload();
        }
      }

      function handleSuccess() {
        reload();
      }

      return {
        hasPermission,
        checkedKeys,
        registerTable,
        handleCreate,
        handleBatchDeauthorize,
        handleDeauthorize,
        handleSuccess,
        formatToDateTime,
      };
    },
  });
</script>
