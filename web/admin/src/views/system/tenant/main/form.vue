<template>
  <BasicDrawer
    v-bind="$attrs"
    @register="registerDrawer"
    showFooter
    :title="getTitle"
    width="50%"
    @ok="handleSubmit"
  >
    <BasicForm @register="registerForm" />
  </BasicDrawer>
</template>
<script lang="ts">
  import { computed, defineComponent, ref, unref } from 'vue';
  import { BasicForm, useForm } from '/@/components/Form/index';
  import { BasicDrawer, useDrawerInner } from '/@/components/Drawer';
  import { saveData } from '/@/api/tenant/tenant';
  import { formSchema } from './data';

  export default defineComponent({
    name: 'ApplicationForm',
    components: { BasicDrawer, BasicForm },
    emits: ['register', 'success'],
    setup(_, { emit }) {
      const isUpdate = ref(true);
      const [registerForm, { resetFields, setFieldsValue, validate }] = useForm({
        labelWidth: 150,
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
            ...data.record,
            expiration_time:
              data.record.expiration_time > 0 ? String(data.record.expiration_time) : undefined,
            logo: data.record.logo ? [data.record.logo] : [],
          });
        }
      });

      const getTitle = computed(() => (unref(isUpdate) ? '编辑租户' : '新增租户'));

      async function handleSubmit() {
        try {
          const values = await validate();
          values.logo = Array.isArray(values.logo) ? values.logo[0] || '' : values.logo || '';
          values.expiration_time = values.expiration_time || 0;
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
