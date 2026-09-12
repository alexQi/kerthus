<template>
  <PageWrapper title="租户详情" dense>
    <template #footer>
      <Tabs :active="activeTab" @change="handleTabChange">
        <TabPane key="info" tab="信息" />
        <TabPane key="apps" tab="应用" />
        <TabPane key="orgs" tab="组织架构" />
        <TabPane key="positions" tab="岗位" />
        <TabPane key="roles" tab="角色" />
        <TabPane key="employees" tab="员工" />
      </Tabs>
    </template>

    <div v-if="fetchParams.id > 0">
      <Info v-if="activeTab === 'info'" :fetchParams="fetchParams" />
      <Apps v-if="activeTab === 'apps'" :fetchParams="fetchParams" />
      <Employees v-if="activeTab === 'employees'" :fetchParams="fetchParams" />
      <Orgs v-if="activeTab === 'orgs'" :fetchParams="fetchParams" />
      <Positions v-if="activeTab === 'positions'" :fetchParams="fetchParams" />
      <Roles v-if="activeTab === 'roles'" :fetchParams="fetchParams" />
    </div>
  </PageWrapper>
</template>
<script lang="ts">
  import { defineComponent, onMounted, ref, unref } from 'vue';
  import { PageWrapper } from '/@/components/Page';
  import { TabPane, Tabs } from 'ant-design-vue';
  import { useRouter } from 'vue-router';
  import Info from './components/info/index.vue';
  import Apps from './components/apps/index.vue';
  import Employees from './components/employees/index.vue';
  import Orgs from './components/orgs/index.vue';
  import Positions from './components/positions/index.vue';
  import Roles from './components/roles/index.vue';

  export default defineComponent({
    components: {
      PageWrapper,
      Tabs,
      TabPane,
      Info,
      Apps,
      Employees,
      Orgs,
      Positions,
      Roles,
    },
    setup() {
      const activeTab = ref('info');
      const fetchParams = ref<any>({ id: 0 });
      const { currentRoute } = useRouter();

      onMounted(() => {
        fetchParams.value = unref(currentRoute).params;
      });

      const handleTabChange = (val) => {
        activeTab.value = val;
      };
      return {
        activeTab,
        fetchParams,
        handleTabChange,
      };
    },
  });
</script>
