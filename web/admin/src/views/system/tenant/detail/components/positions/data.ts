import { h } from 'vue';
import { BasicColumn, FormSchema } from '/@/components/Table';
import { Switch } from 'ant-design-vue';
import { useMessage } from '/@/hooks/web/useMessage';
import { setStatus } from '/@/api/basic/position';
import { queryOrgTree } from '/@/api/basic/org';
import { usePermission } from '/@/hooks/web/usePermission';

export const columns: BasicColumn[] = [
  {
    title: 'ID',
    dataIndex: 'id',
    key: 'id',
    width: 80,
  },
  {
    title: '名称',
    dataIndex: 'name',
    key: 'name',
    align: 'left',
    width: 120,
  },
  {
    title: '所属机构',
    dataIndex: 'org_name',
    key: 'org_name',
    align: 'left',
    width: 240,
  },
  {
    title: '状态',
    dataIndex: 'status',
    key: 'status',
    width: 120,
    customRender: ({ record }) => {
      if (!Reflect.has(record, 'pendingStatus')) {
        record.pendingStatus = false;
      }
      const { hasPermission } = usePermission();
      return h(Switch, {
        disabled: !hasPermission(['system:tenant:detail:position:setStatus']),
        checked: record.status === 1,
        checkedChildren: '已启用',
        unCheckedChildren: '已禁用',
        loading: record.pendingStatus,
        onChange(checked: boolean) {
          record.pendingStatus = true;
          const newStatus = checked ? 1 : 0;
          const { createMessage } = useMessage();
          setStatus(record.id, newStatus, record.tenant_id)
            .then(() => {
              record.status = newStatus;
              createMessage.success(`已成功修改岗位状态`);
            })
            .catch((e) => {
              createMessage.error(e.message ? e.message : '修改岗位状态失败');
            })
            .finally(() => {
              record.pendingStatus = false;
            });
        },
      });
    },
  },
  {
    title: '备注',
    dataIndex: 'remark',
    key: 'remark',
    align: 'left',
  },
  {
    title: '创建时间',
    dataIndex: 'created_at',
    key: 'created_at',
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    field: 'name',
    component: 'Input',
    label: '岗位名称',
    colProps: { span: 8 },
  },
];

export const formSchema: FormSchema[] = [
  {
    field: 'id',
    label: 'Id',
    required: false,
    component: 'Input',
    show: false,
  },
  {
    field: 'org_id',
    label: '所属机构',
    required: true,
    component: 'ApiTreeSelect',
    componentProps: {
      api: queryOrgTree,
      immediate: false,
    },
  },
  {
    field: 'name',
    label: '名称',
    required: true,
    component: 'Input',
  },
  {
    field: 'status',
    label: '状态',
    defaultValue: true,
    component: 'Switch',
    required: true,
  },
  {
    field: 'remark',
    label: '备注',
    component: 'InputTextArea',
  },
];
