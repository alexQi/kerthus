<template>
  <PageWrapper>
    <template #headerContent>
      <WorkbenchHeader />
    </template>
    <div class="lg:flex">
      <div class="lg:w-7/10 w-full !mr-4 enter-y">
        <AppsPanels class="enter-y" />
      </div>
      <div class="lg:w-3/10 w-full enter-y">
        <QuickNav class="enter-y" />

        <Card class="!my-4 enter-y">
          <img class="xl:h-50 h-30 mx-auto" src="../../../assets/svg/illustration.svg" />
        </Card>
      </div>
    </div>
  </PageWrapper>
</template>
<script lang="ts" setup>
  import { Card } from 'ant-design-vue';
  import { PageWrapper } from '/@/components/Page';
  import WorkbenchHeader from './components/WorkbenchHeader.vue';
  import AppsPanels from './components/AppsPanels.vue';
  import QuickNav from './components/QuickNav.vue';
</script>
