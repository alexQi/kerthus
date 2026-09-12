<template>
  <Card title="我的应用" v-bind="$attrs" :loading="loading">
    <template #extra>
      <BasicHelp text="点击应用进行切换应用" />
    </template>
    <CardGrid
      v-for="item in apps"
      :key="item.id"
      :class="'!md:w-1/3 !w-full ' + (currentSaasConf.appId === item.id ? 'card-select' : '')"
      :hoverable="currentSaasConf.appId !== item.id"
      @click="switchApp(item)"
      style="cursor: pointer"
    >
      <a
        v-if="isThirdPartyApp(item) && externalUrl(item.url)"
        :href="externalUrl(item.url)"
        target="_blank"
        rel="noopener noreferrer"
        referrerpolicy="no-referrer"
        class="block"
        @click.stop
      >
        <div class="flex mb-4" style="align-items: center">
          <Image
            :preview="false"
            :src="getImageSrc(item.icon) || '/resource/img/logo.png'"
            :fallback="'/resource/img/logo.png'"
            :width="50"
            :height="50"
            style="border-radius: 1rem"
          />
          <div class="text-lg ml-4">{{ item.name }}</div>
        </div>
        <div class="flex mt-2 h-10 text-secondary">{{ item.desc }}</div>
      </a>
      <template v-else>
        <div class="flex mb-4" style="align-items: center">
          <Image
            :preview="false"
            :src="getImageSrc(item.icon) || '/resource/img/logo.png'"
            :fallback="'/resource/img/logo.png'"
            :width="50"
            :height="50"
            style="border-radius: 1rem"
          />
          <div class="text-lg ml-4">{{ item.name }}</div>
        </div>
        <div class="flex mt-2 h-10 text-secondary">{{ item.desc }}</div>
      </template>
    </CardGrid>
  </Card>
</template>
<script lang="ts">
  import { createConfirmDialog } from '/@/components/Modal/src/confirmDialog';
  import { createVNode, defineComponent, onMounted, ref } from 'vue';
  import { Card, CardGrid, Image } from 'ant-design-vue';
  import { ExclamationCircleOutlined } from '@ant-design/icons-vue';
  import { BasicHelp } from '/@/components/Basic';
  import { getTenantApps, hasApp } from '/@/api/basic/employee';
  import { usePermissionStore } from '/@/store/modules/permission';
  import { useUserStore } from '/@/store/modules/user';
  import { usePermission } from '/@/hooks/web/usePermission';
  import { getSaasConf } from '/@/utils/auth';
  import { externalUrl, isThirdPartyApp, openExternalUrl } from '/@/utils/externalUrl';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { getImageSrc } from '/@/utils/file/resource';

  export default defineComponent({
    components: { BasicHelp, Card, CardGrid, Image },
    setup() {
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

      onMounted(() => {
        fetchData();
        currentSaasConf.value = getSaasConf();
      });

      async function switchApp(item) {
        if (isThirdPartyApp(item)) {
          if (!openExternalUrl(item.url)) createMessage.error('应用地址无效，请联系管理员');
          return;
        }
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
          onCancel() {},
        });
      }

      return {
        apps,
        loading,
        currentSaasConf,
        getImageSrc,
        switchApp,
        externalUrl,
        isThirdPartyApp,
      };
    },
  });
</script>
