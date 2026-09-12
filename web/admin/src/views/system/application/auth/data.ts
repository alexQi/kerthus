import { BasicColumn, FormSchema } from '/@/components/Table';

export const columns: BasicColumn[] = [
  {
    title: '名称',
    dataIndex: 'name',
    key: 'name',
    align: 'left',
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    label: '租户',
    field: 'name',
    component: 'Input',
    colProps: { span: 16 },
  },
];
