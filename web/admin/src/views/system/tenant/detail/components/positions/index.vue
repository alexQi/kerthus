<template>
  <div class="flex">
    <OrgTree class="w-4/17" @select="handleSelect" :fetchParams="fetchParams" />
    <List class="w-13/17" :orgParams="orgParams" :fetchParams="fetchParams" />
  </div>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import OrgTree from '../orgTree/index.vue';
  import List from './list.vue';

  export default defineComponent({
    name: 'Position',
    components: { List, OrgTree },
    props: {
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
    },
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
