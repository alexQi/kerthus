import { h } from 'vue';
import { BasicColumn, FormSchema } from '/@/components/Table';
import { Switch } from 'ant-design-vue';
import { useMessage } from '/@/hooks/web/useMessage';
import { setStatus } from '/@/api/tenant/role';
import { usePermission } from '/@/hooks/web/usePermission';

export const columns: BasicColumn[] = [
  {
    title: '名称',
    dataIndex: 'name',
    key: 'name',
    align: 'left',
    width: 120,
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
        disabled: !hasPermission(['system:tenant:detail:role:setStatus']),
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
              createMessage.success(`修改角色状态成功`);
            })
            .catch((e) => {
              createMessage.error(e.message ? e.message : '修改角色状态失败');
            })
            .finally(() => {
              record.pendingStatus = false;
            });
        },
      });
    },
  },
];

export const userColumns: BasicColumn[] = [
  {
    title: '用户',
    dataIndex: 'name',
    key: 'name',
  },
  {
    title: '手机号码',
    dataIndex: 'phone',
    key: 'phone',
  },
  {
    title: '邮箱',
    dataIndex: 'email',
    key: 'email',
  },
];

export const userRelateColumns: BasicColumn[] = [
  {
    title: '用户',
    dataIndex: 'name',
    key: 'name',
  },
  {
    title: '手机号码',
    dataIndex: 'phone',
    key: 'phone',
  },
  {
    title: '邮箱',
    dataIndex: 'email',
    key: 'email',
  },
];

export const searchUserFormSchema: FormSchema[] = [
  {
    field: 'name',
    component: 'Input',
    label: '姓名',
    colProps: { span: 8, style: 'padding-right:10px' },
  },
  {
    field: 'phone',
    component: 'Input',
    label: '手机号码',
    colProps: { span: 8, style: 'padding-right:10px' },
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    label: '角色名称',
    field: 'name',
    component: 'Input',
    componentProps: {
      placeholder: '请输入关键字',
    },
    colProps: { span: 16 },
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
    field: 'name',
    label: '名称',
    component: 'Input',
    required: true,
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
    required: false,
    component: 'InputTextArea',
    componentProps: {
      resize: 'none',
    },
  },
];
