import { BasicColumn, FormSchema } from '/@/components/Table';

export const columns: BasicColumn[] = [
  {
    title: 'ID',
    dataIndex: 'id',
    key: 'id',
    width: 80,
  },
  {
    title: '租户',
    dataIndex: 'tenant_name',
    key: 'tenant_name',
    align: 'left',
  },
  {
    title: '应用名称',
    dataIndex: 'app_name',
    key: 'app_name',
    align: 'left',
  },
  {
    title: '过期时间',
    dataIndex: 'expiration_time',
    key: 'expiration_time',
    align: 'left',
    width: 120,
  },
  {
    title: '授权时间',
    dataIndex: 'updated_at',
    key: 'updated_at',
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    field: 'tenant_name',
    component: 'Input',
    label: '租户',
    colProps: { span: 6 },
  },
  {
    field: 'app_name',
    component: 'Input',
    label: '应用名称',
    colProps: { span: 6 },
  },
];
