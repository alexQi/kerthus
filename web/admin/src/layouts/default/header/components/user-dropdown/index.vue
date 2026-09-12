<template>
  <Dropdown placement="bottomLeft" :overlayClassName="`${prefixCls}-dropdown-overlay`">
    <span :class="[prefixCls, `${prefixCls}--${theme}`]" class="flex">
      <Avatar
        v-if="getUserInfo.avatar"
        :size="24"
        :class="`${prefixCls}__header__avatar`"
        :src="getImageSrc(getUserInfo.avatar)"
      />
      <Avatar v-else :size="24" :class="`${prefixCls}__header__avatar`">
        {{ getUserInfo.name }}
      </Avatar>
      <span :class="`${prefixCls}__info hidden md:block`">
        <span :class="`${prefixCls}__name  `" class="truncate">
          {{ getUserInfo.name }}
        </span>
      </span>
    </span>

    <template #overlay>
      <Menu @click="handleMenuClick">
        <MenuItem
          key="profile"
          :text="t('layout.header.userCenter')"
          icon="ant-design:user-outlined"
        />
        <MenuDivider />
        <MenuItem
          v-if="getUseLockPage"
          key="lock"
          :text="t('layout.header.tooltipLock')"
          icon="ion:lock-closed-outline"
        />
        <MenuItem
          key="logout"
          :text="t('layout.header.dropdownItemLoginOut')"
          icon="ion:power-outline"
        />
      </Menu>
    </template>
  </Dropdown>
  <LockAction @register="register" />
</template>
<script lang="ts">
  // components
  import { Avatar, Dropdown, Menu } from 'ant-design-vue';
  import type { MenuInfo } from 'ant-design-vue/lib/menu/src/interface';
  import { computed, defineComponent } from 'vue';
  import { DOC_URL } from '/@/settings/siteSetting';
  import { useUserStore } from '/@/store/modules/user';
  import { useHeaderSetting } from '/@/hooks/setting/useHeaderSetting';
  import { useI18n } from '/@/hooks/web/useI18n';
  import { useDesign } from '/@/hooks/web/useDesign';
  import { useModal } from '/@/components/Modal';
  import { propTypes } from '/@/utils/propTypes';
  import { openWindow } from '/@/utils';
  import { getImageSrc } from '/@/utils/file/resource';
  import { createAsyncComponent } from '/@/utils/factory/createAsyncComponent';
  import { useGo } from '/@/hooks/web/usePage';

  type MenuEvent = 'profile' | 'logout' | 'doc' | 'lock';

  export default defineComponent({
    name: 'UserDropdown',
    components: {
      Avatar,
      Dropdown,
      Menu,
      MenuItem: createAsyncComponent(() => import('./DropMenuItem.vue')),
      MenuDivider: Menu.Divider,
      LockAction: createAsyncComponent(() => import('../lock/LockModal.vue')),
    },
    props: {
      theme: propTypes.oneOf(['dark', 'light']),
    },
    setup() {
      const { prefixCls } = useDesign('header-user-dropdown');
      const { t } = useI18n();
      const { getShowDoc, getUseLockPage } = useHeaderSetting();
      const userStore = useUserStore();

      const getUserInfo = computed(() => {
        const { name = '', avatar } = userStore.getUserInfo || {};
        return { name, avatar };
      });

      const go = useGo();
      const [register, { openModal }] = useModal();

      function handleLock() {
        openModal(true, {});
      }

      //  login out
      function handleLoginOut() {
        userStore.confirmLoginOut();
      }

      // open doc
      function openDoc() {
        openWindow(DOC_URL);
      }

      function handleMenuClick(e: MenuInfo) {
        switch (e.key as MenuEvent) {
          case 'logout':
            handleLoginOut();
            break;
          case 'profile':
            go('/profile');
            break;
          case 'doc':
            openDoc();
            break;
          case 'lock':
            handleLock();
            break;
        }
      }

      return {
        t,
        prefixCls,
        getImageSrc,
        getUserInfo,
        handleMenuClick,
        getShowDoc,
        register,
        getUseLockPage,
      };
    },
  });
</script>
<style lang="less">
  @prefix-cls: ~'@{system-prefix}-header-user-dropdown';

  .@{prefix-cls} {
    height: @header-height;
    padding: 0 0 0 10px;
    padding-right: 10px;
    overflow: hidden;
    font-size: 12px;
    cursor: pointer;
    align-items: center;

    &__header {
      border-radius: 50%;

      &__avatar {
        margin-right: 12px;
      }
    }

    &__name {
      font-size: 14px;
    }

    &--dark {
      &:hover {
        background-color: @header-dark-bg-hover-color;
      }
    }

    &--light {
      &:hover {
        background-color: @header-light-bg-hover-color;
      }

      .@{prefix-cls}__name {
        color: @text-color-base;
      }

      .@{prefix-cls}__desc {
        color: @header-light-desc-color;
      }
    }

    &-dropdown-overlay {
      .ant-dropdown-menu-item {
        min-width: 160px;
      }
    }
  }
</style>
