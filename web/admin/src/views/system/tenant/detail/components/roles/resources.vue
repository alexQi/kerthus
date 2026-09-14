<template>
  <RoleResources
    :roleId="roleId"
    :tenantId="fetchParams.id"
    @select="$emit('select', $event)"
    @loading="$emit('loading', $event)"
    @readonly="$emit('readonly', $event)"
  />
</template>
<script lang="ts">
  import { defineComponent, PropType } from 'vue';
  import RoleResources from '/@/views/basic/system/role/resources.vue';

  export default defineComponent({
    components: { RoleResources },
    props: {
      roleId: { type: [Number, String] as PropType<number | string>, default: 0 },
      fetchParams: { type: Object as PropType<any>, default: () => ({}) },
    },
    emits: ['select', 'loading', 'readonly'],
  });
</script>
