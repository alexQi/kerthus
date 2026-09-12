<template>
  <PageWrapper dense contentFullHeight contentClass="flex">
    <OrgTree
      class="w-2/7"
      ref="treeRef"
      @select="handleSelect"
      @add="handleAdd"
      @edit="handleEdit"
    />
    <OrgForm
      class="w-5/7"
      :orgId="orgParams.id"
      :parentName="orgParams.parent_name"
      :accessEdit="accessEdit"
      :parentItem="parentItem"
      @success="handleSuccess"
    />
  </PageWrapper>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { PageWrapper } from '/@/components/Page';
  import OrgForm from './form.vue';
  import OrgTree from './tree.vue';

  export default defineComponent({
    name: 'OrgIndex',
    components: { OrgForm, OrgTree, PageWrapper },
    setup() {
      const treeRef = ref<any>(null);
      const orgParams = ref<any>({ id: 0 });
      const parentItem = ref<any>({});
      const accessEdit = ref<boolean>(false);

      function handleAdd(val) {
        accessEdit.value = true;
        orgParams.value = { id: 0 };
        parentItem.value = val;
      }

      function handleEdit(val) {
        accessEdit.value = true;
        orgParams.value = { ...val };
      }

      function handleSuccess() {
        treeRef.value.fetchTree();
      }

      function handleSelect(val) {
        orgParams.value = val;
      }

      return {
        treeRef,
        orgParams,
        accessEdit,
        parentItem,
        handleAdd,
        handleEdit,
        handleSelect,
        handleSuccess,
      };
    },
  });
</script>
