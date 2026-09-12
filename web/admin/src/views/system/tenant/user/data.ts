import { BasicColumn, FormSchema } from '/@/components/Table';
import { h } from 'vue';
import { Switch } from 'ant-design-vue';
import { useMessage } from '/@/hooks/web/useMessage';
import { formatToDateTime } from '/@/utils/dateUtil';
import { setStatus } from '/@/api/user/user';

export const columns: BasicColumn[] = [
  {
    title: '序号',
    dataIndex: 'id',
    key: 'id',
  },
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
  {
    title: '状态',
    dataIndex: 'status',
    key: 'status',
    customRender: ({ record }) => {
      if (!Reflect.has(record, 'pendingStatus')) {
        record.pendingStatus = false;
      }
      return h(Switch, {
        checked: record.status === 1,
        checkedChildren: '已启用',
        unCheckedChildren: '已禁用',
        loading: record.pendingStatus,
        onChange(checked: boolean) {
          record.pendingStatus = true;
          const newStatus = checked ? 1 : 0;
          const { createMessage } = useMessage();
          setStatus(record.id, newStatus)
            .then(() => {
              record.status = newStatus;
              createMessage.success(`已成功修改用户状态`);
            })
            .catch((e) => {
              createMessage.error(e.message ? e.message : '修改用户状态失败');
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
    customRender: ({ record }) =>
      Number(record.created_at) > 0 ? formatToDateTime(Number(record.created_at) * 1000) : '—',
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    field: 'keyword',
    component: 'Input',
    label: '姓名',
    componentProps: { placeholder: '请输入姓名' },
    colProps: { span: 12 },
  },
];

export const formSchema: FormSchema[] = [
  { field: 'id', label: 'ID', component: 'Input', show: false },
  { field: 'avatar', label: '头像', component: 'Input', show: false },
  { field: 'name', label: '姓名', component: 'Input', required: true },
  { field: 'phone', label: '手机号', component: 'Input', componentProps: { disabled: true } },
  { field: 'email', label: '邮箱', component: 'Input' },
  {
    field: 'sex',
    label: '性别',
    component: 'RadioGroup',
    componentProps: {
      options: [
        { label: '未设置', value: 0 },
        { label: '男', value: 1 },
        { label: '女', value: 2 },
      ],
    },
  },
];
