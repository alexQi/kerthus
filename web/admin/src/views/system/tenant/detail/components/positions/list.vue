<template>
  <div>
    <BasicTable @register="registerTable" :searchInfo="searchInfo">
      <template #tableTitle>
        <span class="ml-1" style="font-size: 16px">
          {{ tableTitle }}
        </span>
      </template>
      <template #toolbar>
        <a-button
          v-auth="['system:tenant:detail:position:create']"
          type="primary"
          @click="handleCreate"
        >
          新增
        </a-button>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'created_at'">
          {{ formatToDateTime(record.created_at * 1000) }}
        </template>
        <template v-if="column.key === 'org_name'">
          <Tag color="success">{{ record.org_name }}</Tag>
        </template>
        <template v-if="column.key === 'action'">
          <TableAction
            :actions="[
              {
                icon: 'clarity:note-edit-line',
                onClick: handleEdit.bind(null, record),
                ifShow: hasPermission(['system:tenant:detail:position:edit']),
              },
              {
                icon: 'ant-design:delete-outlined',
                color: 'error',
                ifShow: hasPermission(['system:tenant:detail:position:delete']),
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
    <EditForm @register="registerDrawer" @success="handleSuccess" :fetchParams="fetchParams" />
  </div>
</template>
<script lang="ts">
  import { defineComponent, PropType, reactive, ref, unref, watch } from 'vue';
  import { Tag } from 'ant-design-vue';
  import { BasicTable, TableAction, useTable } from '/@/components/Table';
  import { deleteData, getList } from '/@/api/basic/position';
  import { formatToDateTime } from '/@/utils/dateUtil';
  import { useDrawer } from '/@/components/Drawer';
  import EditForm from './form.vue';
  import { columns, searchFormSchema } from './data';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'PositionList',
    components: { BasicTable, EditForm, TableAction, Tag },
    props: {
      orgParams: {
        type: [Object] as PropType<any>,
        default: function () {
          return {};
        },
      },
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
    },
    setup(props) {
      const tableTitle = ref<string>('岗位列表');
      const searchInfo = reactive<Recordable>({});
      const { hasPermission } = usePermission();
      const [registerDrawer, { openDrawer }] = useDrawer();
      const [registerTable, { reload }] = useTable({
        api: getList,
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
          width: 80,
          title: '操作',
          dataIndex: 'action',
          fixed: undefined,
          ifShow: hasPermission([
            'system:tenant:detail:position:edit',
            'system:tenant:detail:position:delete',
          ]),
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
        const data = await deleteData({ id: record.id, tenant_id: props.fetchParams.id });
        if (data) {
          reload();
        }
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
            tableTitle.value = '[' + orgParams.name + '] 的岗位列表';
          } else {
            tableTitle.value = '岗位列表';
          }
          reload();
        },
      );

      return {
        tableTitle,
        searchInfo,
        formatToDateTime,
        hasPermission,
        registerTable,
        registerDrawer,
        handleCreate,
        handleEdit,
        handleDelete,
        handleSuccess,
      };
    },
  });
</script>
