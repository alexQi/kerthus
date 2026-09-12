<template>
  <div class="m-4 mr-0 bg-white overflow-hidden">
    <div class="m-4">
      <Button
        v-auth="['system:tenant:detail:org:save']"
        type="primary"
        class="mr-2"
        @click="handleAdd"
      >
        <template #icon>
          <PlusOutlined />
        </template>
        新增节点
      </Button>
      <Button class="mr-2" @click="fetchTree">
        <template #icon>
          <ReloadOutlined />
        </template>
        刷新
      </Button>
    </div>
    <BasicTree
      title="组织架构"
      toolbar
      search
      checkable
      treeWrapperClassName="h-[calc(100%-35px)] overflow-auto mt-2"
      ref="treeRef"
      :loading="loading"
      :treeData="orgData"
      :actionList="actionList"
      @select="handleSelect"
    >
      <template #title="record">
        <Tag :color="typeMap[record.type].color">
          {{ typeMap[record.type].text }}
        </Tag>
        {{ record.title }}
      </template>
    </BasicTree>
  </div>
</template>
<script lang="ts">
  import { defineComponent, h, nextTick, onMounted, ref, unref } from 'vue';
  import { Button, SelectProps, Tag } from 'ant-design-vue';
  import {
    DeleteOutlined,
    EditOutlined,
    PlusOutlined,
    PlusSquareOutlined,
    ReloadOutlined,
  } from '@ant-design/icons-vue';
  import { BasicTree, TreeActionItem, TreeActionType, TreeItem } from '/@/components/Tree';
  import { usePermission } from '/@/hooks/web/usePermission';
  import { deleteItem, queryOrgTree } from '/@/api/basic/org';

  export default defineComponent({
    name: 'ResourceTree',
    components: {
      Button,
      Tag,
      BasicTree,
      PlusOutlined,
      ReloadOutlined,
    },
    props: {
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
    },
    emits: ['select', 'add', 'edit', 'delete'],
    setup(props, { emit }) {
      const typeMap = {
        unit: {
          color: 'error',
          text: '单位/门店',
        },
        section: {
          color: 'processing',
          text: '部门',
        },
      };
      const { hasPermission } = usePermission();
      const treeRef = ref<Nullable<TreeActionType>>(null);
      const loading = ref<boolean>(false);
      const orgData = ref<TreeItem[]>([]);
      const options = ref<SelectProps['options']>([]);

      const actionList: TreeActionItem[] = [
        {
          show: hasPermission(['system:tenant:detail:org:save']),
          render: (node) => {
            return h(PlusSquareOutlined, {
              class: 'mr-1',
              onClick: (e) => {
                emit('add', {
                  unit_id: node.unit_id === 0 ? node.id : node.unit_id,
                  parent_id: node.id,
                  parent_name: node.name,
                });
                e.stopPropagation();
              },
            });
          },
        },
        {
          show: hasPermission(['system:tenant:detail:org:save']),
          render: (node) => {
            return h(EditOutlined, {
              class: 'mr-1',
              onClick: (e) => {
                emit('edit', node);
                e.stopPropagation();
              },
            });
          },
        },
        {
          show: hasPermission(['system:tenant:detail:org:delete']),
          render: (node) => {
            return h(DeleteOutlined, {
              class: 'mr-3',
              style: 'color:#ed6f6f',
              onClick: (e) => {
                handleDelete(node);
                e.stopPropagation();
              },
            });
          },
        },
      ];

      const handleAdd = () => {
        emit('add', {
          unit_id: 0,
          parent_id: 0,
          parent_name: '根节点',
        });
      };

      const handleDelete = async (node) => {
        const data = await deleteItem({ id: node.id, tenant_id: props.fetchParams.id });
        if (data) {
          fetchTree();
        }
      };

      function withParentNames(nodes, parentName = '') {
        return nodes.map((node) => ({
          ...node,
          parent_name: Number(node.parent_id) > 0 ? parentName || node.parent_name || '—' : '根节点',
          children: node.children
            ? withParentNames(node.children, node.name || node.title)
            : undefined,
        }));
      }

      function findNode(nodes, id) {
        for (const node of nodes) {
          if (String(node.id) === String(id)) return node;
          const child = node.children && findNode(node.children, id);
          if (child) return child;
        }
      }

      function handleSelect(keys) {
        const node = findNode(orgData.value, keys[0]);
        if (node) emit('select', node);
      }

      const filterOption = (input: string, option: any) => {
        return option.value.toLowerCase().indexOf(input.toLowerCase()) >= 0;
      };

      async function fetchTree() {
        loading.value = true;
        orgData.value = withParentNames(
          await queryOrgTree({
            tenant_id: props.fetchParams.id,
          }),
        ) as TreeItem[];
        loading.value = false;
        // 展开全部
        nextTick(() => {
          unref(treeRef)?.expandAll(true);
        });
      }

      onMounted(() => {
        fetchTree();
      });

      return {
        treeRef,
        typeMap,
        loading,
        options,
        orgData,
        actionList,
        handleAdd,
        handleSelect,
        filterOption,
        fetchTree,
      };
    },
  });
</script>
