<template>
  <PageWrapper dense contentFullHeight contentClass="flex">
    <ResourceTree
      class="w-1/4"
      ref="treeRef"
      @select="handleSelect"
      @add="handleAdd"
      @edit="handleEdit"
      @change="clearSelection"
      @delete="clearSelection"
    />
    <div class="w-3/4 m-4 mr-0 bg-white">
      <Alert v-if="saveError" class="mb-3" type="error" show-icon :message="saveError" />
      <Card :tabList="tabList" :activeTabKey="activeTab" @tab-change="activeTab = $event">
        <template #tabBarExtraContent>
          <a-button
            v-if="accessEdit"
            class="ml-2"
            type="primary"
            :loading="saving"
            :disabled="loading"
            @click="handleSubmit"
            >保存</a-button
          >
        </template>
        <ResourceForm
          v-show="activeTab === 'resource'"
          ref="formRef"
          :accessEdit="accessEdit && !loading && !saving"
        />
        <ResourceApis
          v-show="activeTab === 'apis'"
          ref="apisRef"
          :resourceId="resourceId"
          :appId="resourceAppId"
          :accessEdit="accessEdit && !loading && !saving"
        />
      </Card>
    </div>
  </PageWrapper>
</template>
<script lang="ts">
  import { defineComponent, ref, toRaw } from 'vue';
  import { Alert, Card } from 'ant-design-vue';
  import { PageWrapper } from '/@/components/Page';
  import { saveResource } from '/@/api/application/application';
  import ResourceTree from './tree.vue';
  import ResourceForm from './form.vue';
  import ResourceApis from './apis.vue';
  import { usePermission } from '/@/hooks/web/usePermission';
  import { useMessage } from '/@/hooks/web/useMessage';

  export default defineComponent({
    name: 'ResourceManager',
    components: { ResourceForm, ResourceTree, ResourceApis, PageWrapper, Card, Alert },
    setup() {
      const activeTab = ref('resource');
      const tabList = [
        { key: 'resource', tab: '应用资源' },
        { key: 'apis', tab: '关联接口' },
      ];
      const treeRef = ref<any>(null);
      const formRef = ref<any>(null);
      const apisRef = ref<any>(null);
      const resourceId = ref<number | string>(0);
      const resourceAppId = ref(0);
      const saveError = ref('');
      const accessEdit = ref(false);
      const loading = ref(false);
      const saving = ref(false);
      let generation = 0;
      const { hasPermission } = usePermission();
      const { createMessage } = useMessage();

      async function clearSelection() {
        ++generation;
        resourceAppId.value = 0;
        saveError.value = '';
        accessEdit.value = false;
        resourceId.value = 0;
        activeTab.value = 'resource';
        loading.value = false;
        apisRef.value?.clear();
        await formRef.value?.clear();
      }

      async function handleAdd(parent) {
        if (saving.value || !hasPermission(['system:application:resource:save'])) return;
        ++generation;
        resourceAppId.value = Number(parent.app_id);
        saveError.value = '';
        resourceId.value = 0;
        activeTab.value = 'resource';
        accessEdit.value = true;
        loading.value = true;
        apisRef.value.clear();
        try {
          await formRef.value.newResource(parent);
        } finally {
          loading.value = false;
        }
      }

      async function loadSelection(node, edit: boolean) {
        if (saving.value || !node?.id) return;
        const version = ++generation;
        accessEdit.value = edit && hasPermission(['system:application:resource:save']);
        resourceId.value = node.id;
        resourceAppId.value = Number(node.app_id);
        saveError.value = '';
        activeTab.value = 'resource';
        loading.value = true;
        try {
          const results = await Promise.allSettled([
            formRef.value.loadResource(node.id, node.parent_name),
            apisRef.value.load(node.id),
          ]);
          const error = results.find((result) => result.status === 'rejected');
          if (error && version === generation) {
            accessEdit.value = false;
            createMessage.error('资源加载失败，请重新选择');
          }
        } finally {
          if (version === generation) loading.value = false;
        }
      }
      function handleEdit(node) {
        return loadSelection(node, true);
      }
      function handleSelect(node) {
        return loadSelection(node, false);
      }

      async function handleSubmit() {
        if (saving.value || loading.value || !accessEdit.value) return;
        saving.value = true;
        saveError.value = '';
        const version = generation;
        let requestStarted = false;
        try {
          const data = await formRef.value.getFormData();
          if (!Number.isInteger(data.app_id) || data.app_id <= 0)
            throw new Error('请先选择应用后新增资源');
          const apis = apisRef.value.getDataSource();
          requestStarted = true;
          const id = await saveResource({ ...data, apis: toRaw(apis).filter(Boolean) });
          if (id) {
            if (version === generation) {
              resourceId.value = Number(id);
              await formRef.value.setSavedId(Number(id));
            }
            await treeRef.value.fetchTree();
            createMessage.success('资源保存成功');
          }
        } catch (error: any) {
          if (error?.errorFields) {
            activeTab.value = 'resource';
            const messages = error.errorFields.flatMap((field) => field.errors || []);
            saveError.value = messages[0] || '请检查资源必填信息';
            createMessage.error(saveError.value);
          } else {
            saveError.value = error?.message || '资源保存失败，请检查后重试';
            if (!requestStarted) createMessage.error(saveError.value);
          }
          // The API interceptor displays server errors. Keep the draft for correction.
        } finally {
          saving.value = false;
        }
      }
      return {
        activeTab,
        tabList,
        treeRef,
        formRef,
        apisRef,
        resourceId,
        resourceAppId,
        saveError,
        accessEdit,
        loading,
        saving,
        clearSelection,
        handleAdd,
        handleEdit,
        handleSelect,
        handleSubmit,
      };
    },
  });
</script>
