import { BasicColumn, FormSchema } from '/@/components/Table';
import { h } from 'vue';
import { Switch } from 'ant-design-vue';
import { useMessage } from '/@/hooks/web/useMessage';
import { setStatus } from '/@/api/basic/employee';
import { queryOrgTree } from '/@/api/basic/org';
import { getItems } from '/@/api/basic/position';
import { getTenantApps } from '/@/api/basic/employee';
import { usePermission } from '/@/hooks/web/usePermission';

export const columns: BasicColumn[] = [
  {
    title: '用户',
    dataIndex: 'name',
    key: 'name',
    align: 'left',
    width: 80,
  },
  {
    title: '手机号码',
    dataIndex: 'phone',
    key: 'phone',
    align: 'left',
    width: 120,
  },
  {
    title: '所属部门',
    dataIndex: 'orgs',
    key: 'orgs',
    align: 'left',
  },
  {
    title: '岗位',
    dataIndex: 'positions',
    key: 'positions',
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
        disabled: !hasPermission(['system:tenant:detail:employee:setStatus']),
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
              createMessage.success(`已成功修改成员状态`);
            })
            .catch((e) => {
              createMessage.error(e.message ? e.message : '修改成员状态失败');
            })
            .finally(() => {
              record.pendingStatus = false;
            });
        },
      });
    },
  },
  {
    title: '创建时间',
    dataIndex: 'created_at',
    key: 'created_at',
    width: 200,
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    field: 'name',
    component: 'Input',
    label: '姓名',
    colProps: { span: 6 },
  },
  {
    field: 'phone',
    component: 'Input',
    label: '手机号码',
    colProps: { span: 6 },
  },
];

export const formSchema: FormSchema[] = [
  { field: 'avatar', label: '头像', component: 'Input', show: false },
  {
    field: 'id',
    label: 'Id',
    required: false,
    component: 'Input',
    show: false,
  },
  {
    field: 'tenant_id',
    label: '租户ID',
    required: true,
    show: false,
    component: 'Input',
  },
  {
    field: 'divider-account',
    component: 'Divider',
    label: '账号信息',
    colProps: {
      span: 24,
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'phone',
    label: '手机号码',
    required: true,
    component: 'Input',
    dynamicRules: () => {
      return [
        {
          required: true,
          validator: (_, value) => {
            const pattern =
              /^(13[0-9]|14[01456879]|15[0-35-9]|16[2567]|17[0-8]|18[0-9]|19[0-35-9])\d{8}$/;

            if (!pattern.test(value)) {
              return Promise.reject('电话号码格式不正确');
            }
            return Promise.resolve();
          },
        },
      ];
    },
    colProps: {
      span: 24,
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'password',
    label: '密码',
    show: ({ model }) => {
      return !model.id;
    },
    required: ({ model }) => {
      return !model.id;
    },
    component: 'InputPassword',
    colProps: {
      span: 24,
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'divider-basic',
    component: 'Divider',
    label: '基础信息',
    colProps: {
      span: 24,
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'name',
    label: '姓名',
    required: true,
    component: 'Input',
  },
  {
    field: 'sex',
    label: '性别',
    defaultValue: 0,
    component: 'RadioGroup',
    required: true,
    componentProps: {
      options: [
        { label: '未设置', value: 0 },
        { label: '男', value: 1 },
        { label: '女', value: 2 },
      ],
    },
  },
  {
    field: 'email',
    label: '邮箱',
    required: false,
    component: 'Input',
  },
  {
    field: 'status',
    show: false,
    label: '状态',
    defaultValue: 1,
    component: 'Select',
    required: true,
    componentProps: {
      options: [
        { label: '禁用', value: 0 },
        { label: '正常', value: 1 },
      ],
    },
  },
  {
    field: 'divider-membership',
    component: 'Divider',
    label: '员工信息',
    colProps: {
      span: 24,
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'app_id',
    label: '默认应用',
    required: true,
    component: 'ApiSelect',
    componentProps: {
      api: getTenantApps,
      immediate: false,
      labelField: 'name',
      valueField: 'id',
    },
    colProps: {
      span: 12,
    },
  },
  {
    field: 'org_ids',
    label: '所属机构',
    required: true,
    defaultValue: [],
    component: 'ApiTreeSelect',
    componentProps: {
      api: queryOrgTree,
      immediate: false,
      multiple: true,
      showCheckedStrategy: 'SHOW_PARENT',
      treeCheckable: true,
      treeDefaultExpandAll: true,
    },
    colProps: {
      span: 24,
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'position_ids',
    label: '所属岗位',
    required: true,
    component: 'ApiSelect',
    componentProps: {
      api: getItems,
      immediate: false,
      mode: 'multiple',
      labelField: 'name',
      valueField: 'id',
    },
  },
];
