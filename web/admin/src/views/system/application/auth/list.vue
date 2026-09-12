<template>
  <div class="m-4 mr-0 bg-white">
    <BasicTable @register="registerTable" />
  </div>
</template>
<script lang="ts">
  import { defineComponent, onMounted, ref, unref } from 'vue';
  import { BasicTable, useTable } from '/@/components/Table';
  import { getList } from '/@/api/tenant/tenant';
  import { columns, searchFormSchema } from './data';
  import { useRouter } from 'vue-router';

  export default defineComponent({
    name: 'TenantList',
    components: { BasicTable },
    emits: ['select'],
    setup(_, { emit }) {
      const [registerTable, { reload, setProps }] = useTable({
        title: '租户列表',
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
        immediate: false,
        showIndexColumn: false,
        rowKey: 'id',
        rowSelection: {
          type: 'radio',
          onSelect: onSelect,
          onChange: handleChange,
        },
      });
      const checkedKey = ref<number>(0);
      const { currentRoute } = useRouter();

      onMounted(() => {
        if (unref(currentRoute).query.hasOwnProperty('id')) {
          setProps({
            searchInfo: { id: unref(currentRoute).query.id },
          });
        }
        reload();
      });

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
        }
      }

      return {
        checkedKey,
        registerTable,
      };
    },
  });
</script>
