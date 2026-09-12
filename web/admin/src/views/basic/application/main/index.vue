<template>
  <div>
    <BasicTable @register="registerTable">
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

  export default defineComponent({
    name: 'ApplicationList',
    components: { BasicTable, Detail, TableAction, Tag },
    setup() {
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
      const [registerDrawer, { openDrawer }] = useDrawer();
      const [registerTable] = useTable({
        title: '应用列表',
        api: queryTenantApps,
        columns,
        formConfig: {
          labelWidth: 100,
          schemas: searchFormSchema,
        },
        useSearchForm: true,
        showTableSetting: true,
        bordered: true,
        showIndexColumn: true,
        actionColumn: {
          width: 80,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
        },
      });

      function handleCreate() {
        openDrawer(true, {
          isUpdate: false,
        });
      }

      function handleDetail(record: Recordable) {
        openDrawer(true, {
          record,
          isUpdate: true,
        });
      }

      return {
        registerTable,
        registerDrawer,
        handleCreate,
        handleDetail,
        formatToDateTime,
        typeMap,
        normalizeAppType,
      };
    },
  });
</script>
