<template>
  <Tooltip title="帮助中心" placement="bottom" :mouseEnterDelay="0.5">
    <button type="button" aria-label="帮助中心" class="cursor-pointer" @click="handleClick">
      <QuestionCircleFilled />
    </button>
  </Tooltip>
</template>
<script lang="ts">
  import { defineComponent, h } from 'vue';
  import { Tooltip } from 'ant-design-vue';
  import { QuestionCircleFilled } from '@ant-design/icons-vue';
  import { useMessage } from '/@/hooks/web/useMessage';

  export default defineComponent({
    name: 'HelpCenter',
    components: { QuestionCircleFilled, Tooltip },

    setup() {
      const { createInfoModal } = useMessage();
      let helpOpen = false;
      const handleClick = () => {
        if (helpOpen) return;
        helpOpen = true;
        createInfoModal({
          title: '帮助中心 · 使用指南',
          width: 560,
          closable: true,
          okText: '我知道了',
          afterClose: () => {
            helpOpen = false;
          },
          content: h('div', { style: { lineHeight: '1.8' } }, [
            h('p', '企业管理员可按以下顺序设置企业管理：'),
            h('ol', { style: { paddingLeft: '20px', listStyleType: 'decimal' } }, [
              h('li', '建立组织，再创建岗位并选择所属组织。'),
              h('li', '添加成员，设置成员所属组织和岗位。'),
              h(
                'li',
                '创建角色，勾选需要的菜单与操作权限，并设置数据范围。每项权限需单独选择，全部授权时可使用全选。',
              ),
              h('li', '在角色的人员管理中关联成员，再用对应成员账号确认可访问的页面和数据。'),
            ]),
            h('p', { style: { marginTop: '16px' } }, [
              h('strong', '平台管理员：'),
              '在“租户应用授权”中选择租户、应用资源和有效期，保存后租户才能使用相应应用。',
            ]),
            h('p', [
              h('strong', '找不到页面或无法操作：'),
              '请确认当前租户和应用，并联系管理员核对应用开通情况、角色权限及有效期。',
            ]),
            h(
              'p',
              { style: { marginBottom: 0 } },
              '个人资料和密码可在右上角用户菜单的个人中心中修改。',
            ),
          ]),
        });
      };

      return {
        handleClick,
      };
    },
  });
</script>
