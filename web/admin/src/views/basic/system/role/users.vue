<template>
  <BasicTable @register="registerTable" class="role-users" :searchInfo="searchInfo">
    <template v-if="roleId > 0" #toolbar>
      <a-button v-auth="['basic:user:role:relateEmployee']" type="primary" @click="handleRelate">
        绑定员工
      </a-button>
    </template>
    <template #bodyCell="{ column, record }">
      <template v-if="column.key === 'action'">
        <TableAction
          :actions="[
            {
              icon: 'ant-design:unlock-outlined',
              color: 'error',
              ifShow: hasPermission(['basic:user:role:relateEmployee']),
              popConfirm: {
                title: '是否确认取消关联',
                placement: 'left',
                confirm: handleDelete.bind(null, record),
              },
            },
          ]"
        />
      </template>
    </template>
  </BasicTable>
  <UserRelate @register="registerModal" :roleId="roleId" @success="handleSuccess" />
</template>
<script lang="ts">
  import { defineComponent, onMounted, PropType, reactive, watch } from 'vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { getList } from '/@/api/basic/employee';
  import { useModal } from '/@/components/Modal';
  import UserRelate from './userRelate.vue';
  import { userColumns } from './data';
  import { relateEmployee } from '/@/api/tenant/role';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'RoleUser',
    components: { BasicTable, TableAction, UserRelate },
    props: {
      roleId: {
        type: [Number, String] as PropType<number | string>,
        default: 0,
      },
    },
    setup(props) {
      const searchInfo = reactive<Recordable>({ role_id: props.roleId });
      const { hasPermission } = usePermission();
      const [registerModal, { openModal }] = useModal();
      const [registerTable, { reload }] = useTable({
        api: getList,
        columns: userColumns,
        striped: true,
        bordered: true,
        immediate: false,
        showTableSetting: true,
        showIndexColumn: true,
        actionColumn: {
          width: 80,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
          ifShow: hasPermission(['basic:user:role:relateEmployee']),
        },
      });

      function handleRelate() {
        openModal(true);
      }

      onMounted(() => {
        props.roleId && reload();
      });

      async function handleDelete(record: Recordable) {
        const params = {
          action: 'del',
          role_id: props.roleId,
          user_ids: [record.id],
        };
        const data = await relateEmployee(params);
        if (data) {
          reload();
        }
      }

      function handleSuccess() {
        reload();
      }

      watch(
        () => props.roleId,
        (roleId: any) => {
          searchInfo.role_id = roleId;
          reload();
        },
      );

      return {
        hasPermission,
        registerTable,
        registerModal,
        handleRelate,
        handleDelete,
        handleSuccess,
        searchInfo,
      };
    },
  });
</script>
<style lang="less" scoped>
  .role-users {
    border: 1px solid rgb(217, 217, 217);
  }
</style>
