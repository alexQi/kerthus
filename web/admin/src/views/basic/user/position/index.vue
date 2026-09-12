<template>
  <PageWrapper dense contentFullHeight contentClass="flex">
    <OrgTree class="w-4/17" @select="handleSelect" />
    <List class="w-13/17" :orgParams="orgParams" />
  </PageWrapper>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { PageWrapper } from '/@/components/Page';
  import List from './list.vue';
  import OrgTree from '../components/orgTree/index.vue';

  export default defineComponent({
    name: 'Position',
    components: { List, OrgTree, PageWrapper },
    setup() {
      const orgParams = ref<any>({});

      function handleSelect(val) {
        orgParams.value = val;
      }

      return {
        orgParams,
        handleSelect,
      };
    },
  });
</script>
