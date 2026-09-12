<template>
  <div>
    <BasicTable @register="registerTable">
      <template #toolbar>
        <Button @click="handleCreate" type="primary" v-auth="['system:application:main:create']">
          <Icon icon="ant-design:plus-outlined" />
          新增应用
        </Button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'type'">
          <Tag :color="(typeMap[normalizeAppType(record.type)] || typeMap.unknown).color">
            {{ (typeMap[normalizeAppType(record.type)] || typeMap.unknown).text }}
          </Tag>
        </template>
        <template v-if="column.key === 'is_public'">
          <Tag :color="record.is_public === 1 ? '#f50' : '#87d068'">
            {{ record.is_public === 1 ? '公开' : '私有' }}
          </Tag>
        </template>
        <template v-if="column.key === 'created_at'">
          {{ formatToDateTime(record.created_at * 1000) }}
        </template>
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                icon: 'clarity:note-edit-line',
                onClick: handleEdit.bind(null, record),
                ifShow: hasPermission(['system:application:main:edit']),
              },
              {
                icon: 'ant-design:delete-outlined',
                color: 'error',
                ifShow: hasPermission(['system:application:main:delete']),
                popConfirm: {
                  title: '是否确认删除',
                  placement: 'left',
                  confirm: handleDelete.bind(null, record),
                },
              },
            ]"
          />
        </template>
      </template>
    </BasicTable>
    <EditForm @register="registerDrawer" @success="handleSuccess" />
  </div>
</template>
<script lang="ts">
  import { normalizeAppType } from '/@/utils/externalUrl';
  import { defineComponent } from 'vue';
  import { Button, Tag } from 'ant-design-vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { Icon } from '/@/components/Icon';
  import { deleteData, getList } from '/@/api/application/application';
  import { formatToDateTime } from '/@/utils/dateUtil';
  import { useDrawer } from '/@/components/Drawer';
  import EditForm from './form.vue';
  import { columns, searchFormSchema } from './data';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'ApplicationList',
    components: { Button, Icon, BasicTable, EditForm, TableAction, Tag },
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
      const { hasPermission } = usePermission();
      const [registerDrawer, { openDrawer }] = useDrawer();
      const [registerTable, { reload }] = useTable({
        title: '应用列表',
        api: getList,
        columns,
        formConfig: {
          labelWidth: 100,
          schemas: searchFormSchema,
        },
        useSearchForm: true,
        showTableSetting: true,
        bordered: true,
        showIndexColumn: false,
        actionColumn: {
          width: 80,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
          ifShow: hasPermission(['system:application:main:edit', 'system:application:main:delete']),
        },
      });

      function handleCreate() {
        openDrawer(true, {
          isUpdate: false,
        });
      }

      function handleEdit(record: Recordable) {
        openDrawer(true, {
          record,
          isUpdate: true,
        });
      }

      async function handleDelete(record: Recordable) {
        const data = await deleteData({ id: record.id });
        if (data) {
          reload();
        }
      }

      function handleSuccess() {
        reload();
      }

      return {
        registerTable,
        registerDrawer,
        hasPermission,
        handleCreate,
        handleEdit,
        handleDelete,
        handleSuccess,
        formatToDateTime,
        typeMap,
        normalizeAppType,
      };
    },
  });
</script>
