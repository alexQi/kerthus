<template>
  <ScrollContainer>
    <div ref="wrapperRef" :class="prefixCls">
      <Alert
        v-if="noApplicationPage"
        type="info"
        show-icon
        message="当前应用暂无可访问页面，请联系管理员授权。"
        class="mb-4"
      />
      <Tabs tab-position="left" :tabBarStyle="tabBarStyle">
        <template v-for="item in settingList" :key="item.key">
          <TabPane :tab="item.name">
            <component :is="item.component" />
          </TabPane>
        </template>
      </Tabs>
    </div>
  </ScrollContainer>
</template>

<script lang="ts">
  import { defineComponent, computed } from 'vue';
  import { useUserStore } from '/@/store/modules/user';
  import { Tabs, Alert } from 'ant-design-vue';
  import { ScrollContainer } from '/@/components/Container';
  import { settingList } from './data';

  import BaseSetting from './BaseSetting.vue';
  import SecureSetting from './SecureSetting.vue';

  export default defineComponent({
    components: {
      Alert,
      ScrollContainer,
      Tabs,
      TabPane: Tabs.TabPane,
      BaseSetting,
      SecureSetting,
    },
    setup() {
      return {
        noApplicationPage: computed(() => useUserStore().getSaasConf.home === '/profile/setting'),
        prefixCls: 'account-setting',
        settingList,
        tabBarStyle: {
          width: '120px',
        },
      };
    },
  });
</script>
<style lang="less">
  .account-setting {
    margin: 12px;
    background-color: @component-background;

    .base-title {
      padding-left: 0;
    }

    .ant-tabs-tab-active {
      background-color: @item-active-bg;
    }
  }
</style>
