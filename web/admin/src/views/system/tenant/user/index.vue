<template>
  <div>
    <BasicTable @register="registerTable">
      <template #toolbar>
        <a-button type="primary" @click="handleManageMembers">管理租户成员</a-button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                icon: 'clarity:note-edit-line',
                onClick: handleEdit.bind(null, record),
              },
            ]"
          />
        </template>
      </template>
    </BasicTable>
    <ApplicationForm @register="registerDrawer" @success="handleSuccess" />
  </div>
</template>
<script lang="ts">
  import { defineComponent } from 'vue';
  import { useRouter } from 'vue-router';

  import { BasicTable, useTable, TableAction } from '/@/components/Table';
  import { getList } from '/@/api/user/user';

  import { useGo } from '/@/hooks/web/usePage';
  import { useDrawer } from '/@/components/Drawer';
  import ApplicationForm from './form.vue';
  import { columns, searchFormSchema } from './data';

  export default defineComponent({
    name: 'UserList',
    components: { BasicTable, ApplicationForm, TableAction },
    setup() {
      const go = useGo();
      const router = useRouter();
      const [registerDrawer, { openDrawer }] = useDrawer();
      const [registerTable, { reload }] = useTable({
        title: '用户列表',
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
          // slots: { customRender: 'action' },
          fixed: undefined,
        },
      });

      function handleManageMembers() {
        const paths = ['/system/tenants', '/system/tenant/index'];
        const routes = router.getRoutes();
        go(paths.find((path) => routes.some((route) => route.path === path)) || paths[0]);
      }

      function handleEdit(record: Recordable) {
        openDrawer(true, {
          record,
          isUpdate: true,
        });
      }

      function handleSuccess() {
        reload();
      }

      return {
        registerTable,
        registerDrawer,
        handleManageMembers,
        handleEdit,
        handleSuccess,
      };
    },
  });
</script>
