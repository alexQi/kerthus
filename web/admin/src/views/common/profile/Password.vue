<template>
  <BasicModal v-bind="$attrs" @register="registerModal" title="修改当前用户密码" @ok="handleSubmit">
    <BasicForm @register="registerForm" />
  </BasicModal>
</template>
<script lang="ts">
  import { defineComponent, ref } from 'vue';
  import { BasicForm, useForm } from '/@/components/Form';
  import { BasicModal, useModalInner } from '/@/components/Modal';
  import { passwordFormSchema } from './data';
  import { modifyPassword } from '/@/api/user/user';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { useUserStore } from '/@/store/modules/user';

  export default defineComponent({
    name: 'ChangePassword',
    components: { BasicForm, BasicModal },
    emits: ['success', 'register'],
    setup(_, { emit }) {
      const { createMessage } = useMessage();
      const [registerForm, { validate, resetFields }] = useForm({
        size: 'large',
        baseColProps: { span: 24 },
        labelWidth: 100,
        showActionButtonGroup: false,
        schemas: passwordFormSchema,
      });

      const submitting = ref(false);
      const [registerModal, { setModalProps, closeModal }] = useModalInner(async () => {
        await resetFields();
        await setModalProps({ confirmLoading: false });
      });

      async function handleSubmit() {
        if (submitting.value) return;
        try {
          const values = await validate();
          submitting.value = true;
          await setModalProps({ confirmLoading: true });
          await modifyPassword({ old_password: values.old_password, password: values.password });
          await resetFields();
          closeModal();
          emit('success');
          createMessage.success('密码已修改，请重新登录');
          await useUserStore().logout(false);
        } finally {
          submitting.value = false;
          await setModalProps({ confirmLoading: false });
        }
      }

      return { registerForm, registerModal, resetFields, handleSubmit };
    },
  });
</script>
