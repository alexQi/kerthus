<template>
  <CollapseContainer title="基本设置" :canExpan="false">
    <a-row :gutter="24">
      <a-col :xs="24" :md="14">
        <BasicForm @register="register" />
      </a-col>
      <a-col :xs="24" :md="10">
        <div class="change-avatar" @error.capture="handleAvatarError">
          <div class="mb-2">头像</div>
          <CropperAvatar
            :uploadApi="userUploadApi"
            :value="avatar"
            btnText="更换头像"
            :btnProps="{ preIcon: 'ant-design:cloud-upload-outlined' }"
            @change="updateAvatar"
            width="150"
          />
        </div>
      </a-col>
    </a-row>
    <Button type="primary" :loading="saving" @click="handleSubmit"> 更新基本信息</Button>
  </CollapseContainer>
</template>
<script lang="ts">
  import { Button, Col, Row } from 'ant-design-vue';
  import { computed, defineComponent, onMounted, ref, watch } from 'vue';
  import { BasicForm, useForm } from '/@/components/Form/index';
  import { CollapseContainer } from '/@/components/Container';
  import { CropperAvatar } from '/@/components/Cropper';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { baseSetschemas } from './data';
  import { useUserStore } from '/@/store/modules/user';
  import { usePermissionStore } from '/@/store/modules/permission';
  import defaultAvatar from '/@/assets/icons/dynamic-avatar-1.svg';
  import { uploadApi } from '/@/api/app/upload';
  import { getImageSrc } from '/@/utils/file/resource';
  import { modifyInfo } from '/@/api/user/user';
  import { UploadFileParams } from '/#/axios';

  export default defineComponent({
    components: {
      BasicForm,
      CollapseContainer,
      Button,
      ARow: Row,
      ACol: Col,
      CropperAvatar,
    },
    setup() {
      const { createMessage } = useMessage();
      const userStore = useUserStore();

      const permissionStore = usePermissionStore();
      const saving = ref(false);
      const avatarFailed = ref(false);
      const [register, { setFieldsValue, validate }] = useForm({
        labelWidth: 120,
        schemas: [
          ...baseSetschemas,
          {
            field: 'current_password',
            component: 'InputPassword',
            label: '当前密码',
            componentProps: { autocomplete: 'current-password', placeholder: '修改登录邮箱时验证' },
            colProps: { span: 18 },
            ifShow: ({ values }) =>
              !permissionStore.getIsPlatformAdmin &&
              (values.email || '').trim() !== (userStore.getUserInfo.email || ''),
            required: true,
          },
        ],
        showActionButtonGroup: false,
      });

      onMounted(async () => {
        await setFieldsValue(userStore.getUserInfo);
      });
      watch(
        () => userStore.getUserInfo.avatar,
        () => {
          avatarFailed.value = false;
        },
      );
      const avatar = computed(() =>
        avatarFailed.value
          ? defaultAvatar
          : getImageSrc(userStore.getUserInfo.avatar) || defaultAvatar,
      );
      function handleAvatarError(event: Event) {
        if (event.target instanceof HTMLImageElement) avatarFailed.value = true;
      }

      async function handleSubmit() {
        if (saving.value) return;
        try {
          const values = await validate();
          saving.value = true;
          const current = userStore.getUserInfo;
          const updated = {
            ...current,
            name: values.name.trim(),
            email: (values.email || '').trim(),
          };
          const emailChanged =
            updated.email !== (current.email || '') && !permissionStore.getIsPlatformAdmin;
          await modifyInfo({
            id: current.id,
            name: updated.name,
            email: updated.email,
            avatar: current.avatar || '',
            sex: current.sex || 0,
            ...(emailChanged ? { password: values.current_password } : {}),
          });
          userStore.setUserInfo(updated);
          await setFieldsValue({ current_password: '' });
          createMessage.success(emailChanged ? '登录邮箱已更新，请重新登录' : '更新成功！');
          if (emailChanged) await userStore.logout(false);
        } finally {
          saving.value = false;
        }
      }

      async function updateAvatar(upload) {
        const current = userStore.getUserInfo;
        const updated = { ...current, avatar: upload.data };
        await modifyInfo({
          id: current.id,
          name: current.name,
          email: current.email || '',
          avatar: updated.avatar,
          sex: current.sex || 0,
        });
        userStore.setUserInfo(updated);
        createMessage.success('头像已更新');
      }

      function userUploadApi(
        uploadParams: UploadFileParams,
        onUploadProgress: (progressEvent: ProgressEvent) => void,
      ) {
        return uploadApi(
          { ...uploadParams, data: { directory: 'saas', param: 'user' } },
          onUploadProgress,
        );
      }

      return {
        avatar,
        saving,
        handleAvatarError,
        register,
        userUploadApi: userUploadApi as any,
        updateAvatar,
        handleSubmit,
      };
    },
  });
</script>

<style lang="less" scoped>
  .change-avatar {
    img {
      display: block;
      margin-bottom: 15px;
      border-radius: 50%;
    }
  }
</style>
