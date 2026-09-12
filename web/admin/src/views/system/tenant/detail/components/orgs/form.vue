<template>
  <div class="m-4 mr-0 bg-white">
    <Card :title="titleRef">
      <template v-if="accessEdit && hasPermission(['system:tenant:detail:org:save'])" #extra>
        <a-button @click="resetForm">重置</a-button>
        <a-button
          class="ml-2"
          type="primary"
          :loading="saving"
          :disabled="loading"
          @click="handleSubmit"
          >保存</a-button
        >
      </template>
      <BasicForm @register="registerForm" @submit="handleSubmit" />
    </Card>
  </div>
</template>
<script lang="ts">
  import { defineComponent, onMounted, PropType, ref, unref, watch } from 'vue';
  import { Card } from 'ant-design-vue';
  import { BasicForm, useForm } from '/@/components/Form';
  import { formSchema } from './data';
  import { getInfo, saveItem } from '/@/api/basic/org';
  import { usePermission } from '/@/hooks/web/usePermission';

  export default defineComponent({
    name: 'ResourceForm',
    components: { Card, BasicForm },
    props: {
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
      orgId: {
        type: [Number, String] as PropType<number | string>,
        default: 0,
      },
      parentName: { type: String, default: '' },
      parentItem: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
      accessEdit: {
        type: Boolean as PropType<boolean>,
        default: false,
      },
    },
    emits: ['success'],
    setup(props, { emit }) {
      const { hasPermission } = usePermission();
      const titleRef = ref<string>('未选择组织');
      const currentRecord = ref<Recordable>({});
      const loading = ref(false);
      const saving = ref(false);
      let loadVersion = 0;
      const [registerForm, { resetFields, setFieldsValue, validate, setProps }] = useForm({
        labelWidth: 100,
        schemas: formSchema,
        showActionButtonGroup: false,
        resetButtonOptions: {
          text: '重置',
        },
        submitButtonOptions: {
          text: '提交',
        },
        disabled: !props.accessEdit,
        baseColProps: { lg: 24, md: 24 },
        actionColOptions: { span: 12 },
      });

      onMounted(async () => {
        await resetFields();
      });

      async function resetForm() {
        await resetFields();
        await setFieldsValue(currentRecord.value);
      }

      async function fetchInfo() {
        const version = ++loadVersion;
        loading.value = true;
        await setProps({ disabled: true });
        try {
          const data = await getInfo({ id: props.orgId, tenant_id: props.fetchParams.id });
          if (!data || version !== loadVersion) return;
          currentRecord.value = {
            ...data,
            parent_name:
              Number(data.parent_id) > 0 ? props.parentName || data.parent_name || '—' : '根节点',
            status: data.status === 1,
            type: data.type || (Number(data.parent_id) === 0 ? 'unit' : 'section'),
          };
          await resetForm();
          titleRef.value = data.name;
        } finally {
          if (version === loadVersion) {
            loading.value = false;
            await setProps({ disabled: !props.accessEdit });
          }
        }
      }

      async function handleSubmit() {
        if (saving.value || loading.value || !props.accessEdit) return;
        saving.value = true;
        try {
          const values = await validate();
          values.status = values.status ? 1 : 0;
          const data = await saveItem({ ...values, tenant_id: props.fetchParams.id });
          if (data) {
            currentRecord.value = { ...values, status: values.status === 1, id: data };
            await setFieldsValue({ id: data });
            titleRef.value = values.name;
            emit('success');
          }
        } finally {
          saving.value = false;
        }
      }

      watch(
        () => unref(props.parentItem),
        async (parentItem: any) => {
          ++loadVersion;
          loading.value = false;
          currentRecord.value = {
            ...parentItem,
            type: Number(parentItem.parent_id) === 0 ? 'unit' : 'section',
          };
          titleRef.value = '新增组织';
          await resetForm();
          await setProps({ disabled: !props.accessEdit });
        },
      );

      watch(
        () => props.accessEdit,
        async (accessEdit: boolean) => {
          await setProps({
            showActionButtonGroup: false,
            disabled: loading.value || !accessEdit,
          });
        },
      );

      watch(
        () => props.orgId,
        (orgId: string | number) => {
          orgId && fetchInfo();
        },
      );

      return { titleRef, hasPermission, registerForm, resetForm, saving, loading, handleSubmit };
    },
  });
</script>
