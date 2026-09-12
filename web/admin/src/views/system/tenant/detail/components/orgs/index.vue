<template>
  <div class="flex">
    <OrgTree
      class="w-2/7"
      ref="treeRef"
      @select="handleSelect"
      @add="handleAdd"
      @edit="handleEdit"
      :fetchParams="fetchParams"
    />
    <OrgForm
      class="w-5/7"
      :orgId="orgParams.id"
      :parentName="orgParams.parent_name"
      :accessEdit="accessEdit"
      :parentItem="parentItem"
      :fetchParams="fetchParams"
      @success="handleSuccess"
    />
  </div>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import OrgForm from './form.vue';
  import OrgTree from './tree.vue';

  export default defineComponent({
    name: 'OrgIndex',
    components: { OrgForm, OrgTree },
    props: {
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
    },
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
