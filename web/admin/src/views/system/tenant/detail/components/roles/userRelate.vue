<template>
  <BasicModal
    v-bind="$attrs"
    width="50%"
    title="角色绑定员工"
    :showCancelBtn="false"
    okText="完成"
    destroyOnClose
    @register="registerModal"
    @ok="handleSubmit"
  >
    <p class="px-4 pt-2 text-secondary">绑定与取消绑定即时生效，完成后关闭窗口即可。</p>
    <BasicTable @register="registerTable" class="role-users" :searchInfo="searchInfo">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                color: record.has_add ? 'error' : 'success',
                label: record.has_add ? '取消绑定' : '绑定员工',
                onClick: handleRelate.bind(null, record),
              },
            ]"
          />
        </template>
      </template>
    </BasicTable>
  </BasicModal>
</template>
<script lang="ts">
  import { defineComponent, PropType, reactive, unref, watch } from 'vue';
  import { BasicModal, useModalInner } from '/@/components/Modal';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { searchUserFormSchema, userRelateColumns } from './data';
  import { getList } from '/@/api/basic/employee';
  import { relateEmployee } from '/@/api/tenant/role';

  export default defineComponent({
    name: 'UserRelate',
    components: { BasicModal, TableAction, BasicTable },
    props: {
      roleId: {
        type: [Number, String] as PropType<number | string>,
        default: 0,
      },
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
    },
    emits: ['success', 'register'],
    setup(props, { emit }) {
      const searchInfo = reactive<Recordable>({
        check_role: 1,
        role_id: props.roleId,
        tenant_id: props.fetchParams.id,
      });
      const [registerModal, { setModalProps, closeModal, redoModalHeight }] = useModalInner(
        async () => {
          setModalProps({ confirmLoading: false });
        },
      );
      const [registerTable, { reload }] = useTable({
        api: getList,
        columns: userRelateColumns,
        formConfig: {
          labelAlign: 'left',
          schemas: searchUserFormSchema,
        },
        isCanResizeParent: true,
        useSearchForm: true,
        striped: true,
        bordered: true,
        immediate: true,
        showTableSetting: true,
        showIndexColumn: true,
        afterFetch: function () {
          redoModalHeight();
        },
        actionColumn: {
          width: 80,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
        },
      });

      watch(
        () => unref(props.roleId),
        (roleId: any) => {
          roleId && (searchInfo.role_id = roleId);
          reload();
        },
      );

      async function handleRelate(record) {
        const params = {
          action: record.has_add ? 'del' : 'add',
          role_id: props.roleId,
          user_ids: [record.id],
          tenant_id: props.fetchParams.id,
        };
        const data = await relateEmployee(params);
        if (data) {
          reload();
        }
      }

      async function handleSubmit() {
        try {
          closeModal();
          emit('success');
        } finally {
          setModalProps({ confirmLoading: false });
        }
      }

      return { registerModal, registerTable, searchInfo, handleRelate, handleSubmit };
    },
  });
</script>
