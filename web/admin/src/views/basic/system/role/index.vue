<template>
  <PageWrapper dense contentFullHeight contentClass="flex">
    <List class="w-4/11" @select="handleSelectRole" />
    <div class="w-7/11 m-4 mr-0 bg-white">
      <Card :tabList="tabList" :activeTabKey="activeTab" @tab-change="handleTabChange">
        <template #tabBarExtraContent>
          <a-button
            v-auth="['basic:user:role:authRoleResource']"
            :disabled="
              activeTab === 'user' ||
              !Number(roleId) ||
              resourcesLoading ||
              resourcesReadonly ||
              saving
            "
            :loading="saving"
            class="ml-2"
            type="primary"
            @click="handleSubmit"
          >
            保存
          </a-button>
        </template>
        <Resources
          v-if="activeTab === 'resource'"
          :roleId="roleId"
          @select="handleSelectResources"
          @loading="resourcesLoading = $event"
          @readonly="resourcesReadonly = $event"
        />
        <Users v-if="activeTab === 'user'" :roleId="roleId" />
      </Card>
    </div>
  </PageWrapper>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { Card } from 'ant-design-vue';
  import { PageWrapper } from '/@/components/Page';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { authRoleResource } from '/@/api/tenant/role';
  import List from './list.vue';
  import Resources from './resources.vue';
  import Users from './users.vue';

  export default defineComponent({
    name: 'RoleResource',
    components: { List, Resources, Users, Card, PageWrapper },
    setup() {
      const activeTab = ref<string>('resource');
      const tabList = ref<any[]>([
        {
          key: 'resource',
          tab: '应用资源',
        },
        {
          key: 'user',
          tab: '用户',
        },
      ]);

      const handleTabChange = (value: string) => {
        if (value === 'resource') resourcesLoading.value = true;
        activeTab.value = value;
      };

      const roleId = ref<number | string>(0);
      const roleState = ref<any>({});
      const resourcesLoading = ref(true);
      const resourcesReadonly = ref(false);
      const saving = ref(false);

      function handleSelectRole(id) {
        if (roleId.value !== id) {
          resourcesLoading.value = true;
          roleState.value = {};
        }
        roleId.value = id;
      }

      function handleSelectResources(val) {
        roleState.value = val;
      }

      async function handleSubmit() {
        if (!roleId.value || resourcesLoading.value || resourcesReadonly.value || saving.value)
          return;
        const params = {
          role_id: roleId.value,
          resource_map: {},
        };
        for (const appId in roleState.value) {
          params.resource_map[appId] = {
            ids: roleState.value[appId].checkedList,
            scope: roleState.value[appId].scope,
          };
        }
        saving.value = true;
        try {
          const data = await authRoleResource(params);
          if (data) {
            const { createMessage } = useMessage();
            createMessage.success('应用授权成功');
          }
        } finally {
          saving.value = false;
        }
      }

      return {
        tabList,
        activeTab,
        roleId,
        resourcesLoading,
        resourcesReadonly,
        saving,
        handleTabChange,
        handleSelectRole,
        handleSelectResources,
        handleSubmit,
      };
    },
  });
</script>
