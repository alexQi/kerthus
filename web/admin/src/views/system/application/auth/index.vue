<template>
  <PageWrapper dense contentFullHeight contentClass="flex">
    <List class="w-1/4" @select="handleSelectTenant" />
    <div class="w-3/4 m-4 mr-0 bg-white">
      <Card title="租户应用授权">
        <template v-if="tenantId > 0" #extra>
          <a-button
            class="ml-2"
            type="primary"
            :disabled="resourcesLoading"
            :loading="saving"
            @click="handleSubmit"
            >保存</a-button
          >
        </template>
        <Resources
          ref="resourcesRef"
          :tenantId="tenantId"
          @select="handleSelectResources"
          @loading="resourcesLoading = $event"
        />
      </Card>
    </div>
  </PageWrapper>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { Card } from 'ant-design-vue';
  import { PageWrapper } from '/@/components/Page';
  import List from './list.vue';
  import Resources from './resources.vue';
  import { authorizeApp } from '/@/api/application/authorize';
  import { useMessage } from '/@/hooks/web/useMessage';

  export default defineComponent({
    name: 'Employees',
    components: { List, Resources, Card, PageWrapper },
    setup() {
      const tenantId = ref<number | string>(0);
      const appState = ref<any>({});
      const resourcesRef = ref<any>(null);
      const resourcesLoading = ref(true);
      const saving = ref(false);

      function handleSelectTenant(id) {
        if (tenantId.value !== id) {
          resourcesLoading.value = true;
          appState.value = {};
        }
        tenantId.value = id;
      }

      function handleSelectResources(val) {
        appState.value = val;
      }

      async function handleSubmit() {
        if (!tenantId.value || resourcesLoading.value || saving.value) return;
        const { createMessage } = useMessage();
        const params = {
          tenant_ids: [tenantId.value],
          resource_map: {},
        };
        for (const appId in appState.value) {
          const state = appState.value[appId];
          if (
            state.thirdParty
              ? !state.selectedApp
              : !state.entitlementId && !state.checkedList.length
          )
            continue;
          if (state.hasTTL && !state.ttl) {
            createMessage.error('请选择应用授权有效期');
            return;
          }
          params.resource_map[appId] = {
            ttl: appState.value[appId].hasTTL ? appState.value[appId].ttl : 0,
            ids: appState.value[appId].checkedList,
          };
        }
        if (!Object.keys(params.resource_map).length) {
          createMessage.error('请先选择要授权的应用或资源');
          return;
        }
        saving.value = true;
        try {
          const data = await authorizeApp(params);
          if (data) {
            await resourcesRef.value?.reload();
            createMessage.success('应用授权成功');
          }
        } finally {
          saving.value = false;
        }
      }

      return {
        tenantId,
        resourcesRef,
        resourcesLoading,
        saving,
        handleSelectTenant,
        handleSelectResources,
        handleSubmit,
      };
    },
  });
</script>
