<template>
  <BasicDrawer
    v-bind="$attrs"
    @register="registerDrawer"
    showFooter
    :title="getTitle"
    width="560px"
    @ok="handleSubmit"
  >
    <BasicForm @register="registerForm" />
    <div class="model-actions">
      <Button :loading="syncing" @click="syncModels"
        ><Icon icon="ant-design:sync-outlined" />同步模型</Button
      >
      <Select
        v-model:value="selectedModel"
        :options="modelOptions"
        placeholder="选择模型后测试"
        allow-clear
      />
      <Button :loading="testing" @click="testModel"
        ><Icon icon="ant-design:api-outlined" />测试连通</Button
      >
    </div>
  </BasicDrawer>
</template>
<script lang="ts">
  import { computed, defineComponent, ref, unref } from 'vue';
  import { Button, Select, message } from 'ant-design-vue';
  import { BasicForm, useForm } from '/@/components/Form';
  import { BasicDrawer, useDrawerInner } from '/@/components/Drawer';
  import { getSaasConf } from '/@/utils/auth';
  import { saveData } from '/@/api/tenant/tenant';
  import { syncProviderModels, testProviderModel } from '/@/api/agent/provider';
  import { formSchema } from './data';
  import { Icon } from '/@/components/Icon';

  export default defineComponent({
    name: 'ModelProviderForm',
    components: { BasicDrawer, BasicForm, Button, Select, Icon },
    emits: ['register', 'success'],
    setup(_, { emit }) {
      const isUpdate = ref(false);
      const syncing = ref(false);
      const testing = ref(false);
      const selectedModel = ref('');
      const providers = ref<any[]>([]);
      const tenantId = ref(0);
      const draft = ref<any>({});
      const [registerForm, { resetFields, setFieldsValue, validate }] = useForm({
        labelWidth: 110,
        baseColProps: { span: 24 },
        schemas: formSchema,
        showActionButtonGroup: false,
      });
      const [registerDrawer, { setDrawerProps, closeDrawer }] = useDrawerInner(async (data) => {
        await resetFields();
        const conf: any = getSaasConf() || {};
        tenantId.value = Number(conf.tenantId || conf.tenant_id || 0);
        providers.value = data?.providers || [];
        isUpdate.value = !!data?.isUpdate;
        draft.value = isUpdate.value
          ? JSON.parse(JSON.stringify(data.record))
          : {
              name: '',
              code: '',
              provider: 'openai',
              endpoint: '',
              api_key: '',
              model: '',
              models: [],
              enabled: true,
              default: !providers.value.length,
            };
        selectedModel.value = draft.value.model || '';
        await setFieldsValue(draft.value);
        if (data?.autoTest) void testModel();
      });
      const getTitle = computed(() => (unref(isUpdate) ? '编辑模型提供商' : '新增模型提供商'));
      const modelOptions = computed(() =>
        (draft.value.models || []).map((model: any) => ({
          value: model.id || model,
          label: model.name || model.id || model,
        })),
      );
      async function syncModels() {
        syncing.value = true;
        try {
          const values: any = await validate();
          const models = await syncProviderModels(values);
          draft.value.models = Array.isArray(models) ? models : [];
          await setFieldsValue({ models: draft.value.models });
          if (!draft.value.model && draft.value.models[0]) {
            draft.value.model = draft.value.models[0].id;
            await setFieldsValue({ model: draft.value.model });
          }
          message.success(`已同步 ${draft.value.models.length} 个模型`);
        } catch (error: any) {
          message.error(error?.message || '模型同步失败');
        } finally {
          syncing.value = false;
        }
      }
      async function testModel() {
        testing.value = true;
        try {
          const values: any = await validate();
          await testProviderModel({ ...values, model: selectedModel.value || values.model });
          message.success(`模型 ${selectedModel.value || values.model} 连通正常`);
        } catch (error: any) {
          message.error(error?.message || '模型连通性测试失败');
        } finally {
          testing.value = false;
        }
      }
      async function handleSubmit() {
        try {
          const values: any = await validate();
          const target = { ...draft.value, ...values, model: selectedModel.value || values.model };
          const next = JSON.parse(JSON.stringify(providers.value));
          const index = next.findIndex((item: any) => item._id === target._id);
          if (index >= 0) next[index] = target;
          else next.push({ ...target, _id: String(Date.now()) });
          const defaultIndex = target.default
            ? index >= 0
              ? index
              : next.length - 1
            : next.findIndex((item: any) => item.default && item.enabled);
          const enabledDefault =
            defaultIndex >= 0 ? defaultIndex : next.findIndex((item: any) => item.enabled);
          next.forEach((item: any, itemIndex: number) => {
            item.default = itemIndex === enabledDefault;
          });
          setDrawerProps({ confirmLoading: true });
          await saveData({
            id: tenantId.value,
            name: '当前租户',
            agent_providers: JSON.stringify(next),
          });
          closeDrawer();
          emit('success', next);
        } catch (error: any) {
          message.error(error?.message || '模型提供商保存失败');
        } finally {
          setDrawerProps({ confirmLoading: false });
        }
      }
      return {
        registerDrawer,
        registerForm,
        getTitle,
        handleSubmit,
        syncModels,
        testModel,
        syncing,
        testing,
        selectedModel,
        modelOptions,
      };
    },
  });
</script>
<style scoped>
  .model-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px solid var(--border-color, #f0f0f0);
  }
  .model-actions .ant-select {
    flex: 1;
  }
</style>
