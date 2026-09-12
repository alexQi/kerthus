<template>
  <div class="m-4 mr-0 bg-white overflow-hidden">
    <div class="m-4">
      <Button class="mr-2" @click="fetchTree">
        <template #icon>
          <ReloadOutlined />
        </template>
        刷新
      </Button>
      <Checkbox :checked="includeChild" @change="handleChecked">包含子级</Checkbox>
    </div>
    <BasicTree
      title="组织架构"
      toolbar
      search
      treeWrapperClassName="h-[calc(100%-35px)] overflow-auto mt-2"
      ref="treeRef"
      :loading="loading"
      :treeData="orgData"
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
  import { defineComponent, nextTick, onMounted, ref, unref } from 'vue';
  import { Button, SelectProps, Tag, Checkbox } from 'ant-design-vue';
  import { ReloadOutlined } from '@ant-design/icons-vue';
  import { BasicTree, TreeActionType, TreeItem } from '/@/components/Tree';
  import { queryOrgTree } from '/@/api/basic/org';
  import { getChildrenIds } from '/@/utils/helper/treeHelper';

  export default defineComponent({
    name: 'ResourceTree',
    components: {
      Button,
      Tag,
      Checkbox,
      BasicTree,
      ReloadOutlined,
    },

    emits: ['select'],
    setup(_, { emit }) {
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
      const includeChild = ref<boolean>(false);
      const selectParams = ref<any>({});

      const treeRef = ref<Nullable<TreeActionType>>(null);
      const loading = ref<boolean>(false);
      const orgData = ref<TreeItem[]>([]);
      const options = ref<SelectProps['options']>([]);

      function getTree() {
        const tree = unref(treeRef);
        if (!tree) {
          throw new Error('tree is null!');
        }
        return tree;
      }

      function handleChecked(e) {
        includeChild.value = e.target.checked;
        selectParams.value = {
          ...selectParams.value,
          includeChild: e.target.checked,
        };
        emit('select', selectParams.value);
      }

      function handleSelect(keys) {
        const node = getTree().getSelectedNode(keys[0]);
        const childrenIds = getChildrenIds(node?.children);
        selectParams.value = {
          id: node?.id,
          name: node?.name,
          children: childrenIds,
          includeChild: includeChild.value,
        };
        emit('select', selectParams.value);
      }

      const filterOption = (input: string, option: any) => {
        return option.value.toLowerCase().indexOf(input.toLowerCase()) >= 0;
      };

      async function fetchTree() {
        loading.value = true;
        orgData.value = (await queryOrgTree()) as unknown as TreeItem[];
        loading.value = false;
        selectParams.value = {
          id: 0,
          name: '',
          children: [],
          includeChild: includeChild,
        };
        emit('select', selectParams.value);
        // 展开全部
        nextTick(() => {
          unref(treeRef)?.expandAll(true);
        });
      }

      onMounted(() => {
        fetchTree();
      });

      return {
        includeChild,
        treeRef,
        typeMap,
        loading,
        options,
        orgData,
        handleChecked,
        handleSelect,
        filterOption,
        fetchTree,
      };
    },
  });
</script>
