<template>
  <div class="m-4 mr-0 bg-white overflow-hidden">
    <div class="m-4">
      <Select
        :value="currentApp"
        show-search
        placeholder="请选择应用"
        style="width: 100%; margin-bottom: 1rem"
        :options="options"
        :filter-option="filterOption"
        @change="handleChange"
      />
      <Button v-auth="['system:application:resource:save']" class="mr-2" @click="handleAdd">
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
      title="资源列表"
      toolbar
      search
      checkable
      treeWrapperClassName="h-[calc(100%-35px)] overflow-auto mt-2"
      :clickRowToExpand="false"
      :loading="itemLoading"
      :treeData="resourceData"
      :actionList="hasPermission(['system:application:resource:save']) ? actionList : []"
      @select="handleSelect"
    >
      <template #title="record">
        <Icon v-if="record.icon" class="mr-1" :icon="record.icon" style="font-size: 16px" />
        <Tag :color="resourceTypeMap[record.type].color">
          {{ resourceTypeMap[record.type].text }}
        </Tag>
        {{ record.title }}
      </template>
    </BasicTree>
  </div>
</template>
<script lang="ts">
  import { h, defineComponent, onMounted, ref, watch } from 'vue';
  import { Button, Select, SelectProps, Tag } from 'ant-design-vue';
  import { Icon } from '/@/components/Icon';
  import {
    DeleteOutlined,
    PlusOutlined,
    ReloadOutlined,
    EditOutlined,
    PlusSquareOutlined,
  } from '@ant-design/icons-vue';
  import { BasicTree, TreeActionItem, TreeItem } from '/@/components/Tree';
  import { getAppItems, getAppResources, removeResource } from '/@/api/application/application';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'ResourceTree',
    components: {
      Select,
      Button,
      Tag,
      Icon,
      BasicTree,
      PlusOutlined,
      ReloadOutlined,
    },
    emits: ['select', 'add', 'edit', 'delete', 'change'],
    setup(_, { emit }) {
      const resourceTypeMap = {
        menu: {
          color: 'processing',
          text: '菜单',
        },
        view: {
          color: 'warning',
          text: '视图',
        },
        action: {
          color: 'error',
          text: '功能',
        },
        field: {
          color: 'success',
          text: '字段',
        },
      };
      const itemLoading = ref<boolean>(false);
      const currentApp = ref<number | string>(0);
      const resourceData = ref<TreeItem[]>([]);
      let loadVersion = 0;
      const options = ref<SelectProps['options']>([]);

      const actionList: TreeActionItem[] = [
        {
          render: (node) => {
            if (node.open_with === 'outside') return null;
            return h(PlusSquareOutlined, {
              class: 'mr-1',
              onClick: (e) => {
                emit('add', {
                  parent_id: node.id,
                  app_id: node.app_id,
                  parent_name: node.name,
                  parent_type: node.type,
                  code: node.code + ':',
                  path: node.path + '/',
                });
                e.stopPropagation();
              },
            });
          },
        },
        {
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

      const { hasPermission } = usePermission();

      const handleAdd = () => {
        if (!currentApp.value) return;
        emit('add', {
          app_id: currentApp.value,
          parent_id: 0,
          parent_name: '根节点',
          parent_type: 'menu',
        });
      };

      const handleDelete = async (node) => {
        const data = await removeResource({ resource_id: node.id });
        if (data) {
          emit('delete', node.id);
          await fetchTree();
        }
      };

      const handleChange = (value: string) => {
        currentApp.value = value;
      };

      function withParentNames(nodes, parentName = '') {
        return nodes.map((node) => ({
          ...node,
          parent_name:
            Number(node.parent_id) > 0 ? parentName || node.parent_name || '—' : '根节点',
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
        const node = findNode(resourceData.value, keys[0]);
        if (node) emit('select', node);
      }

      const filterOption = (input: string, option: any) => {
        return String(option?.label ?? '')
          .toLowerCase()
          .includes(input.toLowerCase());
      };

      async function fetchApp() {
        const appItems = await getAppItems();
        const tempVal: any[] = [];
        for (const appItemsKey in appItems) {
          tempVal.push({
            value: appItemsKey,
            label: appItems[appItemsKey],
          });
        }
        currentApp.value = tempVal[0]?.value ?? 0;
        options.value = tempVal;
      }

      async function fetchTree() {
        const version = ++loadVersion;
        itemLoading.value = true;
        try {
          const data = currentApp.value ? await getAppResources({ app_id: currentApp.value }) : [];
          if (version === loadVersion) resourceData.value = withParentNames(data || []);
        } finally {
          if (version === loadVersion) itemLoading.value = false;
        }
      }

      onMounted(() => {
        fetchApp();
      });

      watch(
        () => currentApp.value,
        () => {
          resourceData.value = [];
          emit('change');
          fetchTree();
        },
      );

      return {
        resourceTypeMap,
        itemLoading,
        currentApp,
        options,
        resourceData,
        actionList,
        hasPermission,
        handleAdd,
        handleChange,
        handleSelect,
        filterOption,
        fetchTree,
      };
    },
  });
</script>
