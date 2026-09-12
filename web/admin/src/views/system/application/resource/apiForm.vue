<template>
  <BasicModal
    v-bind="$attrs"
    @register="registerModal"
    width="min(720px, 100vw)"
    title="添加接口"
    destroyOnClose
    @ok="handleSubmit"
  >
    <Alert
      v-if="emptyContract"
      class="mb-4"
      type="info"
      show-icon
      message="当前应用尚未注册接口契约，暂无可关联接口。"
    />
    <BasicForm @register="registerForm" layout="vertical" />
  </BasicModal>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { Alert } from 'ant-design-vue';
  import { BasicModal, useModalInner } from '/@/components/Modal';
  import { BasicForm, useForm } from '/@/components/Form';
  import { AppRoutes, AppServers } from '/@/api/app/info';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { apiFormSchema } from './data';

  export default defineComponent({
    name: 'ResourceApiForm',
    components: { BasicModal, BasicForm, Alert },
    props: { appId: { type: Number, default: 0 } },
    emits: ['success', 'register'],
    setup(props, { emit }) {
      const emptyContract = ref(false);
      const { createMessage } = useMessage();
      const [registerForm, { resetFields, setFieldsValue, updateSchema, validate }] = useForm({
        labelWidth: 100,
        baseColProps: { span: 24 },
        schemas: apiFormSchema.map((schema) =>
          schema.field === 'server'
            ? {
                ...schema,
                component: 'Select',
                componentProps: { disabled: true, options: [] },
              }
            : schema,
        ),
        showActionButtonGroup: false,
      });
      const [registerModal, { setModalProps, closeModal }] = useModalInner(
        async (data: { appId: number }) => {
          emptyContract.value = false;
          setModalProps({
            confirmLoading: false,
            loading: true,
            okButtonProps: { disabled: true },
          });
          try {
            await resetFields();
            const servers = await AppServers();
            const current = servers.find(
              (server) => Number(server.id) === Number(data.appId ?? props.appId),
            );
            await updateSchema({
              field: 'server',
              component: 'Select',
              componentProps: {
                disabled: true,
                options: current ? [{ label: current.name, value: current.name }] : [],
              },
            });
            await setFieldsValue({ server: current?.name });
            const directory = current
              ? await AppRoutes({ server: current.name })
              : { classes: [], actions: {} };
            emptyContract.value = !(directory as unknown as { classes?: unknown[] }).classes
              ?.length;
            setModalProps({ okButtonProps: { disabled: emptyContract.value } });
          } catch (error: any) {
            emptyContract.value = true;
            createMessage.error(error?.message || '应用接口加载失败');
          } finally {
            setModalProps({ loading: false });
          }
        },
      );

      async function handleSubmit() {
        if (emptyContract.value) return;
        try {
          const values = await validate();
          setModalProps({ confirmLoading: true });
          const params: Recordable = {};
          for (const item of values.interfaces || []) {
            const [method, action] = item.value.split('#');
            params[method.toUpperCase() + ' ' + item.label] = {
              server: values.server,
              controller: values.controller,
              uri: item.label,
              method,
              action,
            };
          }
          if (!Object.keys(params).length) {
            createMessage.error('请选择要关联的接口');
            return;
          }
          closeModal();
          emit('success', params);
        } catch (error: any) {
          if (error?.errorFields) createMessage.error('请选择当前应用的控制器与接口地址');
          else createMessage.error(error?.message || '接口选择失败');
        } finally {
          setModalProps({ confirmLoading: false });
        }
      }
      return { registerModal, registerForm, emptyContract, handleSubmit };
    },
  });
</script>
