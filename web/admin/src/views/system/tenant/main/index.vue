<template>
  <div>
    <BasicTable @register="registerTable">
      <template #toolbar>
        <a-button type="primary" @click="handleCreate"> 新增</a-button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'register_type'">
          <span :style="'color:' + registerTypeMap[record.register_type].color">
            {{ registerTypeMap[record.register_type].text }}
          </span>
        </template>
        <template v-if="column.key === 'created_at'">
          {{ formatToDateTime(record.created_at * 1000) }}
        </template>
        <template v-if="column.key === 'verify_status'">
          <Badge
            :status="verifyStatusMap[record.verify_status].color"
            :text="verifyStatusMap[record.verify_status].text"
          />
        </template>
        <template v-if="column.key === 'expiration_time'">
          {{
            record.expiration_time === 0
              ? '永久有效'
              : formatToDateTime(record.expiration_time * 1000)
          }}
        </template>
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                icon: 'ant-design:check-outlined',
                ifShow: record.verify_status === 0 && hasPermission(['system:tenant:main:verify']),
                onClick: handleVerify.bind(null, record),
              },
              {
                icon: 'ant-design:eye-outlined',
                onClick: handleView.bind(null, record),
                ifShow: hasPermission(['system:tenant:main:view']),
              },
              {
                icon: 'clarity:note-edit-line',
                onClick: handleEdit.bind(null, record),
                ifShow: hasPermission(['system:tenant:main:edit']),
              },
              {
                icon: 'ant-design:delete-outlined',
                color: 'error',
                ifShow: hasPermission(['system:tenant:main:delete']),
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
    <EditForm @register="registerForm" @success="handleSuccess" />
    <Modal
      :visible="approvalVisible"
      title="审核并开通租户"
      :confirmLoading="approvalSaving"
      :cancelButtonProps="{ disabled: approvalSaving }"
      :closable="!approvalSaving"
      :maskClosable="false"
      destroyOnClose
      @ok="submitApproval"
      @cancel="closeApproval"
    >
      <p>新管理员账号需设置 10–72 字节初始密码；已有账号可留空，保留其原密码。</p>
      <InputPassword
        v-model:value="approvalPassword"
        autocomplete="new-password"
        placeholder="管理员初始密码"
        :disabled="approvalSaving"
        @pressEnter="submitApproval"
      />
    </Modal>
  </div>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { deleteItem, getList, setVerify } from '/@/api/tenant/tenant';
  import { formatToDateTime } from '/@/utils/dateUtil';
  import { useDrawer } from '/@/components/Drawer';
  import { useGo } from '/@/hooks/web/usePage';
  import { Badge, Input, Modal, message } from 'ant-design-vue';
  import EditForm from './form.vue';
  import { columns, searchFormSchema } from './data';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'ApplicationList',
    components: { Badge, BasicTable, EditForm, TableAction, Modal, InputPassword: Input.Password },
    setup() {
      const registerTypeMap = {
        create: {
          color: 'green',
          text: '创建',
        },
        register: {
          color: 'yellow',
          text: '注册',
        },
      };
      const verifyStatusMap: { color: 'warning' | 'success' | 'error'; text: string }[] = [
        {
          color: 'warning',
          text: '待审核',
        },
        {
          color: 'success',
          text: '已通过',
        },
        {
          color: 'error',
          text: '不通过',
        },
      ];
      const go = useGo();
      const { hasPermission } = usePermission();
      const [registerForm, { openDrawer }] = useDrawer();
      const [registerTable, { reload }] = useTable({
        title: '租户列表',
        api: getList,
        columns,
        formConfig: {
          labelWidth: 100,
          schemas: searchFormSchema,
          fieldMapToTime: [['range_time', ['start_time', 'end_time'], 'YYYY-MM-DD']],
        },
        useSearchForm: true,
        showTableSetting: true,
        bordered: true,
        showIndexColumn: false,
        actionColumn: {
          width: 150,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
          ifShow: hasPermission([
            'system:tenant:main:verify',
            'system:tenant:main:view',
            'system:tenant:main:edit',
            'system:tenant:main:delete',
          ]),
        },
      });

      function handleCreate() {
        openDrawer(true, {
          isUpdate: false,
        });
      }

      function handleView(record: Recordable) {
        go('/system/tenant/detail/' + record.id);
      }

      function handleEdit(record: Recordable) {
        openDrawer(true, {
          record,
          isUpdate: true,
        });
      }

      async function handleDelete(record: Recordable) {
        const data = await deleteItem({ id: record.id });
        if (data) {
          reload();
        }
      }

      const approvalVisible = ref(false);
      const approvalSaving = ref(false);
      const approvalPassword = ref('');
      const approvalTenantId = ref(0);

      function handleVerify(record: Recordable) {
        approvalTenantId.value = Number(record.id);
        approvalPassword.value = '';
        approvalVisible.value = true;
      }

      function closeApproval() {
        if (approvalSaving.value) return;
        approvalVisible.value = false;
        approvalPassword.value = '';
        approvalTenantId.value = 0;
      }

      async function submitApproval() {
        if (!approvalVisible.value || approvalSaving.value) return;
        const password = approvalPassword.value;
        const length = new TextEncoder().encode(password).length;
        if (password && (length < 10 || length > 72)) {
          message.error('密码长度应为 10–72 字节');
          return;
        }
        approvalSaving.value = true;
        try {
          await setVerify({
            id: approvalTenantId.value,
            verify_status: 1,
            admin_password: password,
          });
          approvalVisible.value = false;
          approvalPassword.value = '';
          approvalTenantId.value = 0;
          await reload();
        } catch {
          // The API reports validation/conflict errors; keep the form open for correction.
        } finally {
          approvalSaving.value = false;
        }
      }

      function handleSuccess() {
        reload();
      }

      return {
        hasPermission,
        registerTable,
        registerForm,
        handleCreate,
        handleView,
        handleEdit,
        handleDelete,
        handleVerify,
        approvalVisible,
        approvalSaving,
        approvalPassword,
        submitApproval,
        closeApproval,
        handleSuccess,
        formatToDateTime,
        registerTypeMap,
        verifyStatusMap,
      };
    },
  });
</script>
