<template>
  <CollapseContainer title="安全设置" :canExpan="false">
    <List>
      <template v-for="item in list" :key="item.key">
        <ListItem>
          <ListItemMeta>
            <template #title>
              {{ item.title }}
              <a-button type="link" class="extra" v-if="item.extra" @click="handleClick(item.key)">
                {{ item.extra }}
              </a-button>
            </template>
            <template #description>
              <div>{{ item.description }}</div>
            </template>
          </ListItemMeta>
        </ListItem>
      </template>
    </List>
    <Password @register="registerPassword" />
  </CollapseContainer>
</template>
<script lang="ts">
  import { List } from 'ant-design-vue';
  import { computed, defineComponent } from 'vue';
  import { CollapseContainer } from '/@/components/Container';
  import { useModal } from '/@/components/Modal';
  import Password from './Password.vue';
  import { useUserStore } from '/@/store/modules/user';
  import type { ListItem } from './data';

  export default defineComponent({
    components: {
      CollapseContainer,
      Password,
      List,
      ListItem: List.Item,
      ListItemMeta: List.Item.Meta,
    },
    setup() {
      const [registerPassword, passwordModal] = useModal();

      const userStore = useUserStore();
      const list = computed<ListItem[]>(() => {
        const { phone, email } = userStore.getUserInfo;
        const maskedPhone = phone ? phone.replace(/^(\d{3})\d+(\d{4})$/, '$1****$2') : '未设置';
        const maskedEmail = email ? email.replace(/^(.)([^@]*)(@.*)$/, '$1***$3') : '未设置';
        return [
          {
            key: 'password',
            title: '账户密码',
            description: '修改密码需验证当前密码，修改后请重新登录。',
            extra: '修改',
          },
          { key: 'phone', title: '登录手机号', description: maskedPhone },
          { key: 'email', title: '登录邮箱', description: maskedEmail },
        ];
      });
      const handleClick = (key: string) => {
        if (key === 'password') passwordModal.openModal(true, {});
      };
      return {
        registerPassword,
        handleClick,
        list,
      };
    },
  });
</script>
<style lang="less" scoped>
  .extra {
    float: right;
    margin-top: 10px;
    margin-right: 30px;
    font-weight: normal;
    color: #1890ff;
    cursor: pointer;
  }
</style>
