<template>
  <BasicDrawer
    v-bind="$attrs"
    @register="registerDrawer"
    showFooter
    :title="getTitle"
    width="min(560px, 100vw)"
    @ok="handleSubmit"
  >
    <BasicForm @register="registerForm" />
  </BasicDrawer>
</template>
<script lang="ts">
  import { computed, defineComponent, ref, unref } from 'vue';
  import { BasicForm, useForm } from '/@/components/Form/index';
  import { BasicDrawer, useDrawerInner } from '/@/components/Drawer';
  import { saveData } from '/@/api/application/application';
  import { externalUrl, normalizeAppType } from '/@/utils/externalUrl';
  import { formSchema } from './data';

  export default defineComponent({
    name: 'ApplicationForm',
    components: { BasicDrawer, BasicForm },
    emits: ['success', 'register'],
    setup(_, { emit }) {
      const isUpdate = ref(true);

      const [registerForm, { resetFields, setFieldsValue, validate }] = useForm({
        labelWidth: 90,
        baseColProps: { span: 24 },
        schemas: formSchema,
        showActionButtonGroup: false,
      });

      const [registerDrawer, { setDrawerProps, closeDrawer }] = useDrawerInner(async (data) => {
        await resetFields();
        setDrawerProps({ confirmLoading: false });
        isUpdate.value = !!data?.isUpdate;

        if (unref(isUpdate)) {
          await setFieldsValue({
            id: data.record.id,
            code: data.record.code,
            name: data.record.name,
            version: data.record.version,
            type: normalizeAppType(data.record.type),
            url: data.record.url || '',
            is_public: Number(data.record.is_public) === 1 ? 1 : 0,
            icon: data.record.icon ? [data.record.icon] : [],
            desc: data.record.desc || '',
            remark: data.record.remark || '',
          });
        }
      });

      const getTitle = computed(() => (unref(isUpdate) ? '编辑应用' : '新增应用'));

      async function handleSubmit() {
        try {
          const values = await validate();
          values.type = normalizeAppType(values.type);
          values.is_public = Number(values.is_public) === 1 ? 1 : 0;
          if (values.type === 'third') values.url = externalUrl(values.url);
          values.icon = Array.isArray(values.icon) ? values.icon[0] || '' : values.icon || '';
          setDrawerProps({ confirmLoading: true });
          const data = await saveData(values);
          if (data) {
            closeDrawer();
            emit('success');
          }
        } finally {
          setDrawerProps({ confirmLoading: false });
        }
      }

      return {
        registerDrawer,
        registerForm,
        getTitle,
        handleSubmit,
      };
    },
  });
</script>
