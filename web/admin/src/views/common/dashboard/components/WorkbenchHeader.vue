<template>
  <div class="workbench-user">
    <Avatar :src="getImageSrc(userinfo.avatar) || headerImg" :size="72" class="workbench-user__avatar" />
    <div class="workbench-user__text">
      <h1 class="md:text-lg text-md">你好，{{ userinfo.name }}</h1>
      <span class="text-secondary"> 管理租户、成员与已开通应用 </span>
    </div>
  </div>
</template>
<script lang="ts" setup>
  import { computed } from 'vue';
  import { Avatar } from 'ant-design-vue';
  import { useUserStore } from '/@/store/modules/user';
  import headerImg from '/@/assets/images/header.jpg';
  import { getImageSrc } from '/@/utils/file/resource';

  const userStore = useUserStore();
  const userinfo = computed(() => userStore.getUserInfo);
</script>
<style scoped>
  .workbench-user { display: flex; align-items: center; gap: 24px; }
  .workbench-user__avatar { flex: 0 0 auto; }
  .workbench-user__text { min-width: 0; }
  .workbench-user__text h1 { margin-bottom: 8px; }
  @media (max-width: 640px) {
    .workbench-user { gap: 16px; }
  }
</style>
