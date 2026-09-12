<template>
  <Card title="快捷导航">
    <CardGrid v-for="item in items" :key="item.path" @click="go(item.path)" class="cursor-pointer">
      <span class="flex flex-col items-center"
        ><Icon :icon="item.icon || 'ant-design:appstore-outlined'" size="20" /><span
          class="text-md mt-2 truncate"
          >{{ item.meta?.title || item.name }}</span
        ></span
      >
    </CardGrid>
  </Card>
</template>
<script lang="ts" setup>
  import { computed } from 'vue';
  import { Card, CardGrid } from 'ant-design-vue';
  import { Icon } from '/@/components/Icon';
  import { usePermissionStore } from '/@/store/modules/permission';
  import { externalUrl } from '/@/utils/externalUrl';
  import { useGo } from '/@/hooks/web/usePage';
  const store = usePermissionStore();
  const go = useGo();
  const items = computed(() =>
    store.getMenus
      .flatMap((menu) => menu.children || [menu])
      .filter(
        (menu) => !menu.meta?.hideMenu && (!!externalUrl(menu.path) || !menu.path.includes(':')),
      )
      .slice(0, 6),
  );
</script>
