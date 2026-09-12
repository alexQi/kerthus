<template>
  <div>
    <BasicTable @register="registerTable">
      <template v-if="accessEdit" #toolbar>
        <a-button ghost color="success" @click="handleCreate"> 新增</a-button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'uri'">
          <Tag color="success">{{ record.method }}</Tag>
          {{ record.uri }}
        </template>
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                icon: 'ant-design:delete-outlined',
                ifShow: accessEdit,
                color: 'error',
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
    <ApiForm @register="registerModal" :appId="appId" @success="handleSuccess" />
  </div>
</template>
<script lang="ts">
  import { defineComponent, PropType, ref } from 'vue';
  import { Tag } from 'ant-design-vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { queryResourceApis } from '/@/api/application/application';
  import { formatToDateTime } from '/@/utils/dateUtil';
  import { useModal } from '/@/components/Modal';
  import { apiColumns } from './data';
  import ApiForm from './apiForm.vue';

  export default defineComponent({
    name: 'ResourceApis',
    components: { BasicTable, TableAction, Tag, ApiForm },
    props: {
      appId: { type: Number, default: 0 },
      resourceId: {
        type: [Number, String] as PropType<number | string>,
        default: 0,
      },
      accessEdit: {
        type: Boolean as PropType<boolean>,
        default: false,
      },
    },
    setup(props) {
      let generation = 0;
      const draftOperations = ref<Recordable[]>([]);
      const [registerModal, { openModal, closeModal }] = useModal();
      const [registerTable] = useTable({
        dataSource: draftOperations,
        pagination: false,
        rowKey: (record) => record.method.toUpperCase() + ' ' + record.uri,
        columns: apiColumns,
        striped: true,
        bordered: true,
        showTableSetting: true,
        showIndexColumn: true,
        actionColumn: {
          width: 80,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
        },
      });

      function handleCreate() {
        if (props.accessEdit) openModal(true, { appId: props.appId });
      }

      function clear() {
        ++generation;
        closeModal();
        draftOperations.value = [];
      }

      function getDataSource() {
        return draftOperations.value;
      }

      async function load(id: number | string) {
        const version = ++generation;
        closeModal();
        draftOperations.value = [];
        if (!id) return;
        const result = await queryResourceApis({ resource_id: id });
        if (version === generation)
          draftOperations.value = Array.isArray(result) ? result : result?.items || [];
      }

      const operationKey = (item: Recordable) => item.method.toUpperCase() + ' ' + item.uri;

      function handleDelete(record: Recordable) {
        if (!props.accessEdit) return;
        draftOperations.value = getDataSource().filter(
          (item) => operationKey(item) !== operationKey(record),
        );
      }

      function handleSuccess(params) {
        if (!props.accessEdit) return;
        const tableData = getDataSource().filter(
          (item) => !Object.prototype.hasOwnProperty.call(params, operationKey(item)),
        );
        draftOperations.value = tableData.concat(Object.values(params));
      }

      return {
        registerTable,
        registerModal,
        handleCreate,
        handleDelete,
        handleSuccess,
        load,
        clear,
        getDataSource,
        formatToDateTime,
      };
    },
  });
</script>
