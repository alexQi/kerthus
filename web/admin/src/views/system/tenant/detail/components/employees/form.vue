<template>
  <BasicDrawer
    v-bind="$attrs"
    @register="registerDrawer"
    showFooter
    width="min(760px, 100vw)"
    destroyOnClose
    :title="getTitle"
    @ok="handleSubmit"
  >
    <BasicForm @register="registerForm" />
  </BasicDrawer>
</template>
<script lang="ts">
  import { isThirdPartyApp } from '/@/utils/externalUrl';
  import { computed, defineComponent, ref, unref } from 'vue';
  import { BasicForm, useForm } from '/@/components/Form';
  import { formSchema } from './data';
  import { BasicDrawer, useDrawerInner } from '/@/components/Drawer';
  import { getInfo, getTenantApps, saveData } from '/@/api/basic/employee';

  export default defineComponent({
    name: 'UserForm',
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
      const originalAppId = ref(0);
      async function getDefaultAppOptions(params: Recordable = {}) {
        const { original_app_id: _originalAppId, ...query } = params;
        const options = await getTenantApps(query);
        const items = Array.isArray(options)
          ? options.filter((item) => !isThirdPartyApp(item))
          : [];
        const appId = originalAppId.value;
        if (appId && !items.some((item) => Number(item.id) === appId)) {
          items.push({ id: appId, name: '原默认应用（当前不可用）', disabled: true });
        }
        return items;
      }
      const tenantSchemas = computed(() =>
        formSchema.map((schema) =>
          ['app_id', 'org_ids', 'position_ids'].includes(schema.field)
            ? {
                ...schema,
                componentProps: {
                  ...(typeof schema.componentProps === 'object' ? schema.componentProps : {}),
                  immediate: true,
                  ...(schema.field === 'app_id' ? { api: getDefaultAppOptions } : {}),
                  params: {
                    tenant_id: props.fetchParams.id,
                    ...(schema.field === 'app_id' ? { original_app_id: originalAppId.value } : {}),
                  },
                },
              }
            : schema,
        ),
      );
      const [registerForm, { resetFields, setFieldsValue, updateSchema, validate, setProps }] =
        useForm({
          labelWidth: 90,
          baseColProps: { span: 12 },
          schemas: tenantSchemas,
          showActionButtonGroup: false,
        });

      const [registerDrawer, { setDrawerProps, closeDrawer }] = useDrawerInner(async (data) => {
        await setProps({ disabled: true });
        setDrawerProps({ loading: true, confirmLoading: false });
        try {
          originalAppId.value = 0;
          await resetFields();
          isUpdate.value = !!data?.isUpdate;
          await updateSchema({
            field: 'password',
            show: !unref(isUpdate),
            required: !unref(isUpdate),
          });
          await updateSchema({ field: 'phone', componentProps: { disabled: unref(isUpdate) } });
          await updateSchema({ field: 'email', componentProps: { disabled: unref(isUpdate) } });
          await setFieldsValue({ tenant_id: props.fetchParams.id });
          if (unref(isUpdate)) await fetchInfo(data.employeeId);
        } finally {
          await setProps({ disabled: false });
          setDrawerProps({ loading: false });
        }
      });

      const getTitle = computed(() => (!unref(isUpdate) ? '新增用户' : '编辑用户'));

      async function fetchInfo(id) {
        const data = await getInfo({ id, tenant_id: props.fetchParams.id });
        if (data) {
          originalAppId.value = Number(data.app_id) || 0;
          await setFieldsValue({ ...data, sex: data.sex ?? 0 });
        }
      }

      async function handleSubmit() {
        try {
          const values = await validate();
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
