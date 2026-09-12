<template>
  <BasicDrawer
    v-bind="$attrs"
    @register="registerDrawer"
    showFooter
    title="应用详情"
    width="30%"
    @ok="handleSubmit"
  >
    <BasicForm @register="registerForm" />
  </BasicDrawer>
</template>
<script lang="ts">
  import { defineComponent, h, ref, unref } from 'vue';
  import { Image } from 'ant-design-vue';
  import { normalizeAppType } from '/@/utils/externalUrl';
  import { getImageSrc } from '/@/utils/file/resource';
  import { BasicForm, useForm } from '/@/components/Form';
  import { BasicDrawer, useDrawerInner } from '/@/components/Drawer';
  import { formSchema } from './data';

  export default defineComponent({
    name: 'ApplicationDetail',
    components: { BasicDrawer, BasicForm },
    emits: ['register'],
    setup() {
      const isUpdate = ref(true);

      const [registerForm, { resetFields, setFieldsValue }] = useForm({
        labelWidth: 90,
        baseColProps: { span: 24 },
        schemas: formSchema.map((schema) => ({
          ...schema,
          required: false,
          rules: [],
          dynamicRules: undefined,
          render:
            schema.field === 'icon'
              ? ({ model }) =>
                  h(Image, {
                    src: getImageSrc(model.icon) || '/resource/img/logo.png',
                    fallback: '/resource/img/logo.png',
                    width: 96,
                    height: 96,
                    style: { objectFit: 'contain' },
                  })
              : schema.render,
        })),
        showActionButtonGroup: false,
        disabled: true,
      });

      const [registerDrawer, { setDrawerProps, closeDrawer }] = useDrawerInner(async (data) => {
        await resetFields();
        setDrawerProps({ confirmLoading: false });
        isUpdate.value = !!data?.isUpdate;

        if (unref(isUpdate)) {
          await setFieldsValue({
            ...data.record,
            type: normalizeAppType(data.record.type),
            icon: data.record.icon || '',
            is_public: String(data.record.is_public ?? 0),
          });
        }
      });

      async function handleSubmit() {
        closeDrawer();
        setDrawerProps({ confirmLoading: false });
      }

      return {
        registerDrawer,
        registerForm,
        handleSubmit,
      };
    },
  });
</script>
