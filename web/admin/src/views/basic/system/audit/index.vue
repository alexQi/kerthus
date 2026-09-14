<template>
  <div class="m-4"><BasicTable @register="registerTable" /></div>
</template>
<script lang="ts" setup>
  import { h } from 'vue';
  import { BasicTable, useTable } from '/@/components/Table';
  import { getAuditLogList } from '/@/api/system/log';
  import { formatToDateTime } from '/@/utils/dateUtil';
  const entities: Record<string, string> = {
    user: '账号', tenant: '租户', member: '成员', organization: '组织',
    position: '岗位', application: '应用', resource: '资源', role: '角色',
  };
  const historicalActions: Record<string, [string, string]> = {
    'identity.profile.save': ['保存账号资料', '账号'],
    'identity.password.change': ['修改账号密码', '账号'],
    'membership.write': ['保存成员', '成员'],
    'organization.write': ['保存组织或岗位', '对象'],
    'access.write': ['保存角色', '角色'],
    'catalog.save': ['保存应用', '应用'],
    'catalog.resource.save': ['保存应用资源', '资源'],
    'catalog.grant': ['开通租户应用', '租户'],
    'catalog.revoke': ['撤销应用开通', '开通记录'],
    'access.resources.replace': ['调整角色资源授权', '角色'],
    'access.members.change': ['调整角色成员', '角色'],
    'file.upload': ['上传文件', '文件'],
    'status.change': ['变更状态', '对象'],
    'entity.delete': ['删除对象', '对象'],
  };
  function describeAction(code?: string): [string, string] {
    if (!code) return ['—', '对象'];
    if (historicalActions[code]) return historicalActions[code];
    const [entity, operation] = code.split('.');
    const verbs: Record<string, string> = { save: '保存', enable: '启用', disable: '停用', delete: '删除', approve: '审核通过', reject: '审核驳回' };
    return entities[entity] && verbs[operation]
      ? [verbs[operation] + entities[entity], entities[entity]]
      : [code, '对象'];
  }
  const identityLabel = (label: string, value: unknown) => Number(value) > 0 ? `${label} #${value}` : '—';
  const [registerTable] = useTable({
    title: '审计日志', rowKey: 'id', bordered: true, showTableSetting: true,
    api: getAuditLogList,
    columns: [
      { title: '事件编号', dataIndex: 'id', width: 90 },
      { title: '时间', dataIndex: 'created_at', width: 180,
        customRender: ({ text }) => Number(text) > 0 ? formatToDateTime(Number(text) * 1000) : '—' },
      { title: '操作', dataIndex: 'action', width: 170,
        customRender: ({ text }) => h('span', { title: text || '' }, describeAction(text)[0]) },
      { title: '目标', dataIndex: 'target_id', width: 140,
        customRender: ({ text, record }) => identityLabel(describeAction(record.action)[1], text) },
      { title: '操作者', dataIndex: 'actor_id', width: 110,
        customRender: ({ text }) => identityLabel('账号', text) },
      { title: '租户', dataIndex: 'tenant_id', width: 110,
        customRender: ({ text }) => identityLabel('租户', text) },
      { title: '请求编号', dataIndex: 'request_id', width: 240,
        customRender: ({ text }) => text || '—' },
    ],
  });
</script>
