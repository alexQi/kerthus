<template>
  <div>
    <BasicTable @register="registerTable">
      <template #toolbar>
        <a-button v-auth="['system:tenant:detail:apps:auth']" type="primary" @click="handleCreate">
          授权
        </a-button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'type'">
          <Tag :color="(typeMap[normalizeAppType(record.type)] || typeMap.unknown).color">
            {{ (typeMap[normalizeAppType(record.type)] || typeMap.unknown).text }}
          </Tag>
        </template>
        <template v-if="column.key === 'expiration_time'">
          <Tag :color="record.expiration_time > 0 ? 'warning' : 'success'">
            {{
              record.expiration_time > 0
                ? formatToDateTime(record.expiration_time * 1000)
                : '永久有效'
            }}
          </Tag>
        </template>
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                icon: 'ant-design:eye-outlined',
                onClick: handleDetail.bind(null, record),
              },
              {
                color: 'error',
                icon: 'ant-design:close-outlined',
                ifShow: hasPermission(['system:tenant:detail:apps:deauth']),
                disabled: !entitlementId(record),
                tooltip: entitlementId(record) ? '取消授权' : '缺少租户授权记录，请刷新后重试',
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
    <Detail @register="registerDrawer" />
  </div>
</template>
<script lang="ts">
  import { normalizeAppType } from '/@/utils/externalUrl';
  import { defineComponent } from 'vue';
  import { Tag } from 'ant-design-vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { queryTenantApps } from '/@/api/basic/employee';
  import { formatToDateTime } from '/@/utils/dateUtil';
  import { useDrawer } from '/@/components/Drawer';
  import Detail from './detail.vue';
  import { columns, searchFormSchema } from './data';
  import { deauthorizeApp } from '/@/api/application/authorize';
  import { useGo } from '/@/hooks/web/usePage';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'ApplicationList',
    components: { BasicTable, Detail, TableAction, Tag },
    props: {
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
    },
    setup(props) {
      const typeMap = {
        unknown: { color: 'default', text: '未知' },
        self: {
          color: '#108ee9',
          text: '自建',
        },
        third: {
          color: '#f50',
          text: '第三方',
        },
      };
      const go = useGo();
      const { hasPermission } = usePermission();
      const { createMessage } = useMessage();
      const [registerDrawer, { openDrawer }] = useDrawer();
      const [registerTable, { reload }] = useTable({
        title: '应用列表',
        api: queryTenantApps,
        columns,
        formConfig: {
          labelWidth: 100,
          schemas: searchFormSchema,
        },
        searchInfo: {
          tenant_id: props.fetchParams.id,
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
        },
      });

      function handleCreate() {
        go('/system/application/auth?id=' + props.fetchParams.id);
      }

      function handleDetail(record: Recordable) {
        openDrawer(true, {
          record,
          isUpdate: true,
        });
      }

      function entitlementId(record: Recordable) {
        const id = Number(record.tenant_app_id);
        return Number.isSafeInteger(id) && id > 0 ? id : 0;
      }

      async function handleDeauthorize(record: Recordable) {
        const id = entitlementId(record);
        if (!id) {
          createMessage.error('缺少租户授权记录，请刷新后重试');
          return;
        }
        const data = await deauthorizeApp({ tenant_app_ids: [id] });
        if (data) {
          reload();
        }
      }

      return {
        hasPermission,
        registerTable,
        registerDrawer,
        handleCreate,
        handleDetail,
        handleDeauthorize,
        entitlementId,
        formatToDateTime,
        typeMap,
        normalizeAppType,
      };
    },
  });
</script>
