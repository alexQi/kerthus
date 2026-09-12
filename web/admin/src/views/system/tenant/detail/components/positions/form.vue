<template>
  <BasicDrawer
    v-bind="$attrs"
    @register="registerDrawer"
    showFooter
    :title="getTitle"
    width="30%"
    @ok="handleSubmit"
  >
    <BasicForm @register="registerForm" />
  </BasicDrawer>
</template>
<script lang="ts">
  import { computed, defineComponent, PropType, ref, unref } from 'vue';
  import { BasicForm, useForm } from '/@/components/Form';
  import { BasicDrawer, useDrawerInner } from '/@/components/Drawer';
  import { saveData } from '/@/api/basic/position';
  import { formSchema } from './data';

  export default defineComponent({
    name: 'PositionForm',
    components: { BasicDrawer, BasicForm },
    props: {
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
    },
    emits: ['success', 'register'],
    setup(props, { emit }) {
      const isUpdate = ref(true);
      const tenantSchemas = computed(() => formSchema.map((schema) => schema.field === 'org_id' ? {
        ...schema,
        componentProps: {
          ...(typeof schema.componentProps === 'object' ? schema.componentProps : {}),
          immediate: true,
          params: { tenant_id: props.fetchParams.id },
        },
      } : schema));

      const [registerForm, { resetFields, setFieldsValue, validate }] = useForm({
        labelWidth: 90,
        baseColProps: { span: 24 },
        schemas: tenantSchemas,
        showActionButtonGroup: false,
      });

      const [registerDrawer, { setDrawerProps, closeDrawer }] = useDrawerInner(async (data) => {
        await resetFields();
        setDrawerProps({ confirmLoading: false });
        isUpdate.value = !!data?.isUpdate;

        if (unref(isUpdate)) {
          await setFieldsValue({
            ...data.record,
            status: data.record.status === 1,
          });
        }
      });

      const getTitle = computed(() => (unref(isUpdate) ? '编辑岗位' : '新增岗位'));

      async function handleSubmit() {
        try {
          const values = await validate();
          values.status = values.status ? 1 : 0;
          setDrawerProps({ confirmLoading: true });
          const data = await saveData({ ...values, tenant_id: props.fetchParams.id });
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
