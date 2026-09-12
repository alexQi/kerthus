<template>
  <div>
    <BasicTable @register="registerTable" :searchInfo="searchInfo">
      <template #tableTitle>
        <span class="ml-1" style="font-size: 16px">
          {{ tableTitle }}
        </span>
      </template>
      <template #toolbar>
        <a-button v-auth="['basic:user:main:create']" type="primary" @click="handleCreate">
          新增
        </a-button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'orgs'">
          <Tag v-for="(item, index) in record.orgs" :key="index" color="success">
            {{ typeof item === 'string' ? item : item.name }}
          </Tag>
        </template>
        <template v-if="column.key === 'positions'">
          <Tag v-for="(item, index) in record.positions" :key="index" color="warning">
            {{ typeof item === 'string' ? item : item.name }}
          </Tag>
        </template>
        <template v-if="column.key === 'created_at'">
          {{ record.created_at ? formatToDateTime(record.created_at * 1000) : '—' }}
        </template>
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                icon: 'clarity:note-edit-line',
                onClick: handleEdit.bind(null, record),
                ifShow: hasPermission(['basic:user:main:edit']),
              },
              {
                icon: 'ant-design:interaction-outlined',
                color: 'error',
                ifShow: isPlatformAdmin,
                onClick: handleResetPassword.bind(null, record),
              },
            ]"
          />
        </template>
      </template>
    </BasicTable>
    <EmployeeForm @register="registerDrawer" @success="handleSuccess" />
  </div>
</template>
<script lang="ts">
  import { createConfirmDialog } from '/@/components/Modal/src/confirmDialog';
  import { defineComponent, h, PropType, reactive, ref, unref, watch } from 'vue';
  import { Tag, Input } from 'ant-design-vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { getList } from '/@/api/basic/employee';
  import { useDrawer } from '/@/components/Drawer';
  import { formatToDateTime } from '/@/utils/dateUtil';
  import EmployeeForm from './form.vue';
  import { columns, searchFormSchema } from './data';
  import { usePermission } from '/@/hooks/web/usePermission';
  import { resetPassword } from '/@/api/user/user';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { usePermissionStore } from '/@/store/modules/permission';

  export default defineComponent({
    name: 'EmployList',
    components: { BasicTable, EmployeeForm, TableAction, Tag },
    props: {
      orgParams: {
        type: [Object] as PropType<any>,
        default: function () {
          return {};
        },
      },
    },
    setup(props) {
      const tableTitle = ref<string>('员工列表');
      const searchInfo = reactive<Recordable>({});
      const { hasPermission } = usePermission();
      const { createMessage } = useMessage();
      const [registerDrawer, { openDrawer }] = useDrawer();
      const [registerTable, { reload }] = useTable({
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
          ifShow: hasPermission(['basic:user:main:edit']),
        },
      });

      function handleCreate() {
        openDrawer(true, {
          isUpdate: false,
          employeeId: 0,
        });
      }

      function handleEdit(record: Recordable) {
        openDrawer(true, {
          isUpdate: true,
          employeeId: record.id,
        });
      }

      async function handleResetPassword(record: Recordable) {
        const password = ref('');
        createConfirmDialog({
          title: '重置全局账号密码',
          content: () =>
            h(Input.Password, {
              value: password.value,
              'onUpdate:value': (value: string) => {
                password.value = value;
              },
              autocomplete: 'new-password',
              placeholder: '新密码（10–72 字节）',
            }),
          async onOk() {
            await resetPassword({ id: record.id, password: password.value });
            createMessage.success('密码已更新，该账号的所有会话已失效');
          },
        });
      }

      function handleSuccess() {
        reload();
      }

      watch(
        () => unref(props.orgParams),
        (orgParams) => {
          searchInfo.org_id = orgParams?.id;
          searchInfo.children_ids = orgParams?.children;
          searchInfo.with_child = orgParams?.includeChild ? 1 : 0;
          if (orgParams?.id) {
            tableTitle.value = '[' + orgParams.name + '] 的员工列表';
          } else {
            tableTitle.value = '员工列表';
          }
          reload();
        },
      );

      return {
        isPlatformAdmin: usePermissionStore().getIsPlatformAdmin,
        tableTitle,
        searchInfo,
        registerTable,
        registerDrawer,
        handleCreate,
        handleEdit,
        handleSuccess,
        handleResetPassword,
        hasPermission,
        formatToDateTime,
      };
    },
  });
</script>
