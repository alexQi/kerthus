<template>
  <div :class="prefixCls" :style="getWrapStyle">
    <div class="flex items-center justify-between px-4 py-2 bg-white">
      <span class="text-secondary">如页面无法显示，请在新窗口打开。</span>
      <a
        v-if="safeUrl"
        :href="safeUrl"
        target="_blank"
        rel="noopener noreferrer"
        referrerpolicy="no-referrer"
        >在新窗口打开 ↗</a
      >
    </div>
    <Alert
      v-if="!safeUrl"
      type="error"
      show-icon
      message="内链地址无效，请联系管理员。"
      class="m-4"
    />
    <Spin v-else :spinning="loading" size="large">
      <iframe
        :key="safeUrl"
        :src="safeUrl"
        :class="`${prefixCls}__main`"
        :style="{ height: `${frameHeight}px` }"
        title="应用内链"
        referrerpolicy="no-referrer"
        sandbox="allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox"
        @load="hideLoading"
        @error="hideLoading"
      ></iframe>
    </Spin>
  </div>
</template>
<script lang="ts" setup>
  import { ref, computed, watch, onBeforeUnmount } from 'vue';
  import { Alert, Spin } from 'ant-design-vue';
  import { useWindowSizeFn } from '/@/hooks/event/useWindowSizeFn';
  import { useDesign } from '/@/hooks/web/useDesign';
  import { useLayoutHeight } from '/@/layouts/default/content/useContentViewHeight';
  import { externalUrl } from '/@/utils/externalUrl';

  const props = defineProps({ frameSrc: { type: String, default: '' } });
  const safeUrl = computed(() => externalUrl(props.frameSrc));
  const loading = ref(false);
  const height = ref(window.innerHeight);
  const { headerHeightRef } = useLayoutHeight();
  const { prefixCls } = useDesign('iframe-page');
  const frameHeight = computed(() => Math.max(200, height.value - headerHeightRef.value - 72));
  const getWrapStyle = computed(() => ({ height: `${frameHeight.value + 48}px` }));
  useWindowSizeFn(
    () => {
      height.value = window.innerHeight;
    },
    150,
    { immediate: true },
  );
  let loadingTimeout: ReturnType<typeof setTimeout> | undefined;
  function hideLoading() {
    loading.value = false;
    clearTimeout(loadingTimeout);
  }
  watch(
    safeUrl,
    (url) => {
      clearTimeout(loadingTimeout);
      loading.value = !!url;
      // A denied cross-origin frame cannot be inspected reliably. Keep the escape link available.
      if (url) loadingTimeout = setTimeout(hideLoading, 10000);
    },
    { immediate: true },
  );
  onBeforeUnmount(() => clearTimeout(loadingTimeout));
</script>
<style lang="less" scoped>
  @prefix-cls: ~'@{system-prefix}-iframe-page';
  .@{prefix-cls} {
    &__main {
      width: 100%;
      border: 0;
      background-color: @component-background;
    }
  }
</style>
