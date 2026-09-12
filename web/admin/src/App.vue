<template>
  <ConfigProvider :locale="getAntdLocale">
    <AppProvider>
      <RouterView />
    </AppProvider>
  </ConfigProvider>
</template>

<script lang="ts" setup>
  import { ConfigProvider } from 'ant-design-vue';
  import { AppProvider } from '/@/components/Application';
  import { useTitle } from '/@/hooks/web/useTitle';
  import { useLocale } from '/@/locales/useLocale';
  import { useAppConfig } from '/@/hooks/setting/useAppConfig';
  import dayjs from 'dayjs';
  import weekday from 'dayjs/plugin/weekday';
  import 'dayjs/locale/zh-cn';
  dayjs.extend(weekday);
  dayjs.locale('zh-cn');

  // support Multi-language
  const { getAntdLocale } = useLocale();

  // Listening to page changes and dynamically changing site titles
  useTitle();
  useAppConfig();
</script>
