<template>
  <div class="m-4 mr-0 bg-white">
    <BasicTable @register="registerTable">
      <template #toolbar>
        <a-button v-auth="['basic:user:role:save']" type="primary" @click="handleCreate">
          新增
        </a-button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                icon: 'ant-design:edit-outlined',
                color: 'success',
                onClick: handleEdit.bind(null, record),
                ifShow: hasPermission(['basic:user:role:save']),
              },
              {
                icon: 'ant-design:delete-outlined',
                color: 'error',
                ifShow: hasPermission(['basic:user:role:delete']),
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
    <Form @register="registerModal" @success="handleSuccess" />
  </div>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { getList } from '/@/api/tenant/role';
  import Form from './form.vue';
  import { columns, searchFormSchema } from './data';
  import { useModal } from '/@/components/Modal';
  import { deleteData } from '/@/api/tenant/role';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'RoleList',
    components: { BasicTable, TableAction, Form },
    emits: ['select'],
    setup(_, { emit }) {
      const { hasPermission } = usePermission();
      const [registerModal, { openModal }] = useModal();
      const [registerTable, { reload }] = useTable({
        title: '角色列表',
        api: getList,
        formConfig: {
          labelAlign: 'left',
          showResetButton: false,
          schemas: searchFormSchema,
        },
        useSearchForm: true,
        striped: true,
        columns: columns,
        bordered: true,
        showTableSetting: true,
        tableSetting: {
          redo: true,
          size: false,
          fullScreen: false,
        },
        showIndexColumn: false,
        rowKey: 'id',
        rowSelection: {
          type: 'radio',
          onSelect: onSelect,
          onChange: handleChange,
        },
        actionColumn: {
          width: 80,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
          ifShow: hasPermission(['basic:user:role:save', 'basic:user:role:delete']),
        },
      });

      const checkedKey = ref<number>(0);

      function handleChange(selectRowKeys, _) {
        if (selectRowKeys.length > 0) {
          checkedKey.value = parseInt(selectRowKeys[0]);
        } else {
          checkedKey.value = 0;
        }
        emit('select', checkedKey.value);
      }

      function onSelect(record, selected) {
        if (selected) {
          checkedKey.value = parseInt(record.id);
          emit('select', checkedKey.value);
        } else {
          emit('select', 0);
        }
      }

      function handleCreate() {
        openModal(true, {
          isUpdate: false,
        });
      }

      function handleEdit(record: Recordable) {
        openModal(true, {
          record,
          isUpdate: true,
        });
      }

      async function handleDelete(record: Recordable) {
        const data = await deleteData({ id: record.id });
        if (data) {
          if (checkedKey.value === Number(record.id)) {
            checkedKey.value = 0;
            emit('select', 0);
          }
          reload();
        }
      }

      function handleSuccess() {
        reload();
      }

      return {
        hasPermission,
        registerTable,
        registerModal,
        handleCreate,
        handleEdit,
        handleDelete,
        handleSuccess,
      };
    },
  });
</script>
