<template>
  <div :class="prefixCls">
    <Popover
      title=""
      trigger="click"
      placement="bottomRight"
      v-model:visible="visible"
      :overlayClassName="`${prefixCls}__overlay`"
      @click="handleShow"
    >
      <Badge :count="0" dot>
        <AppstoreOutlined />
      </Badge>
      <template #content>
        <div class="bg-white" :loading="loading">
          <Card>
            <CardGrid
              v-for="item in apps"
              :key="item.id"
              :class="
                '!md:w-1/1 !w-full ' + (currentSaasConf.appId === item.id ? 'card-select' : '')
              "
              :hoverable="currentSaasConf.appId !== item.id"
              @click="switchApp(item)"
              style="cursor: pointer; padding: 16px"
            >
              <a
                v-if="isThirdPartyApp(item) && externalUrl(item.url)"
                :href="externalUrl(item.url)"
                target="_blank"
                rel="noopener noreferrer"
                referrerpolicy="no-referrer"
                class="flex"
                style="align-items: center"
                @click.stop
              >
                <div class="flex" style="align-items: center">
                  <Image
                    :preview="false"
                    :src="getImageSrc(item.icon) || '/resource/img/logo.png'"
                    :fallback="'/resource/img/logo.png'"
                    :width="50"
                    :height="50"
                    style="border-radius: 1rem"
                  />
                  <div class="flex-1 ml-4">
                    <div class="text-n">{{ item.name }}</div>
                    <div class="flex text-secondary">{{ item.desc }}</div>
                  </div>
                </div>
              </a>
              <div v-else class="flex" style="align-items: center">
                <Image
                  :preview="false"
                  :src="getImageSrc(item.icon) || '/resource/img/logo.png'"
                  :fallback="'/resource/img/logo.png'"
                  :width="50"
                  :height="50"
                  style="border-radius: 1rem"
                />
                <div class="flex-1 ml-4">
                  <div class="text-n">{{ item.name }}</div>
                  <div class="flex text-secondary">{{ item.desc }}</div>
                </div>
              </div>
            </CardGrid>
          </Card>
        </div>
      </template>
    </Popover>
  </div>
</template>
<script lang="ts">
  import { createConfirmDialog } from '/@/components/Modal/src/confirmDialog';
  import { createVNode, defineComponent, ref } from 'vue';
  import { Badge, Card, CardGrid, Image, Popover } from 'ant-design-vue';
  import { AppstoreOutlined, ExclamationCircleOutlined } from '@ant-design/icons-vue';
  import { useDesign } from '/@/hooks/web/useDesign';
  import { usePermission } from '/@/hooks/web/usePermission';
  import { useUserStore } from '/@/store/modules/user';
  import { usePermissionStore } from '/@/store/modules/permission';
  import { getTenantApps, hasApp } from '/@/api/basic/employee';
  import { getSaasConf } from '/@/utils/auth';
  import { externalUrl, isThirdPartyApp, openExternalUrl } from '/@/utils/externalUrl';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { getImageSrc } from '/@/utils/file/resource';

  export default defineComponent({
    components: { Popover, AppstoreOutlined, Card, CardGrid, Image, Badge },
    setup() {
      const { prefixCls } = useDesign('header-apps');
      const visible = ref<boolean>(false);

      const permission = usePermission();
      const { createMessage } = useMessage();
      const userStore = useUserStore();
      const permissionStore = usePermissionStore();
      const currentSaasConf = ref<Recordable>({});
      const apps = ref<any[]>([]);
      const loading = ref<boolean>(false);

      const fetchData = async () => {
        loading.value = true;
        try {
          apps.value = (await getTenantApps()) || [];
        } finally {
          loading.value = false;
        }
      };

      const handleShow = () => {
        fetchData();
        currentSaasConf.value = getSaasConf();
      };

      async function switchApp(item) {
        if (isThirdPartyApp(item)) {
          if (!openExternalUrl(item.url)) createMessage.error('应用地址无效，请联系管理员');
          return;
        }
        visible.value = false;
        createConfirmDialog({
          // title: '确定要切换到应用：【基础平台】， 并重新加载其资源吗？',
          icon: createVNode(ExclamationCircleOutlined),
          content: '确定要切换到应用：' + item.name + '， 并重新加载其资源吗?',
          async onOk() {
            const data = await hasApp({ app_id: item.id });
            if (data) {
              permissionStore.setDynamicAddedRoute(false);
              userStore.setSaasConf({
                ...currentSaasConf.value,
                appId: item.id,
                appCode: item.code,
                home: '',
              });
              currentSaasConf.value = {
                ...currentSaasConf.value,
                appId: item.id,
                appCode: item.code,
                home: '',
              };
              await permission.refreshMenu(true);
            }
          },
          // eslint-disable-next-line @typescript-eslint/no-empty-function
          onCancel() {
            visible.value = true;
          },
        });
      }

      return {
        visible,
        prefixCls,
        apps,
        loading,
        currentSaasConf,
        getImageSrc,
        handleShow,
        switchApp,
        externalUrl,
        isThirdPartyApp,
      };
    },
  });
</script>
<style lang="less">
  @prefix-cls: ~'@{system-prefix}-header-apps';

  .@{prefix-cls} {
    padding-top: 2px;

    &__overlay {
      max-width: 20%;

      .card-select {
        background: #e8f4ff;
      }
    }

    .ant-badge {
      font-size: 18px;

      .ant-badge-multiple-words {
        padding: 0 4px;
      }

      svg {
        width: 0.9em;
      }
    }
  }
</style>
