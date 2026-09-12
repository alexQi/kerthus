<template>
  <div>
    <Alert
      v-if="unsupportedRoute"
      class="m-4"
      type="warning"
      show-icon
      :message="`原打开方式为${
        unsupportedRoute === 'inside'
          ? '内链'
          : unsupportedRoute === 'outside'
          ? '外链'
          : unsupportedRoute
      }，不是支持的打开方式；已保留原值，此资源暂不可编辑。`"
    />
    <BasicForm @register="registerForm" />
  </div>
</template>
<script lang="ts">
  import { defineComponent, PropType, ref, watch } from 'vue';
  import { Alert } from 'ant-design-vue';
  import { BasicForm, useForm } from '/@/components/Form';
  import { getResource } from '/@/api/application/application';
  import { formSchema } from './data';

  export default defineComponent({
    name: 'ResourceForm',
    components: { BasicForm, Alert },
    props: { accessEdit: { type: Boolean as PropType<boolean>, default: false } },
    setup(props) {
      let generation = 0;
      const unsupportedRoute = ref('');
      let loadedResource: Recordable = {};
      const [registerForm, { resetFields, setFieldsValue, validate, setProps }] = useForm({
        labelWidth: 100,
        schemas: formSchema,
        disabled: !props.accessEdit,
        mergeDynamicData: { resourceEditable: props.accessEdit },
        showActionButtonGroup: false,
        baseColProps: { lg: 12, md: 24 },
      });

      async function clear() {
        ++generation;
        loadedResource = {};
        unsupportedRoute.value = '';
        await resetFields();
      }

      async function loadResource(id: number | string, parentName = '') {
        const version = ++generation;
        await resetFields();
        const data = await getResource({ id });
        if (version !== generation || !data) return;
        loadedResource = data;
        unsupportedRoute.value =
          !data.open_with || ['component', 'route', 'inside', 'outside'].includes(data.open_with)
            ? ''
            : data.open_with;
        await setProps({
          disabled: !props.accessEdit || !!unsupportedRoute.value,
          mergeDynamicData: { resourceEditable: props.accessEdit && !unsupportedRoute.value },
        });
        await setFieldsValue({
          ...data,
          app_id: Number(data.app_id),
          parent_id: Number(data.parent_id) || 0,
          parent_name:
            Number(data.parent_id) > 0 ? parentName || data.parent_name || '—' : '根节点',
          status: data.status === 1,
          is_public: data.is_public === 1,
          open_with: data.open_with === 'route' || !data.open_with ? 'component' : data.open_with,
          meta: data.meta || {},
        });
      }

      async function newResource(parent: Recordable) {
        const version = ++generation;
        loadedResource = {};
        unsupportedRoute.value = '';
        await resetFields();
        if (version !== generation) return;
        await setProps({
          disabled: !props.accessEdit,
          mergeDynamicData: { resourceEditable: props.accessEdit },
        });
        await setFieldsValue({
          ...parent,
          id: 0,
          app_id: Number(parent.app_id),
          parent_id: Number(parent.parent_id) || 0,
          type: 'menu',
          icon: 'ant-design:appstore-outlined',
          status: true,
          is_public: false,
          open_with: 'component',
          component: 'LAYOUT',
          sort: 0,
          meta: {},
        });
      }

      async function getFormData() {
        if (unsupportedRoute.value) throw new Error('不支持此资源的原打开方式，请联系管理员');
        const values = await validate();
        return {
          ...loadedResource,
          ...values,
          id: Number(values.id) || 0,
          app_id: Number(values.app_id),
          parent_id: Number(values.parent_id) || 0,
          status: values.status ? 1 : 0,
          is_public: values.is_public ? 1 : 0,
        };
      }

      async function setSavedId(id: number) {
        await setFieldsValue({ id });
      }
      watch(
        () => props.accessEdit,
        async (accessEdit) => {
          await setProps({
            disabled: !accessEdit || !!unsupportedRoute.value,
            mergeDynamicData: { resourceEditable: accessEdit && !unsupportedRoute.value },
            showActionButtonGroup: false,
          });
        },
      );
      return {
        unsupportedRoute,
        registerForm,
        clear,
        loadResource,
        newResource,
        getFormData,
        setSavedId,
      };
    },
  });
</script>
