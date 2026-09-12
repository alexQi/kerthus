import { BasicColumn, FormSchema } from '/@/components/Table';
import { h } from 'vue';
import { Switch } from 'ant-design-vue';
import { useMessage } from '/@/hooks/web/useMessage';
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
              createMessage.success(`已成功修改角色状态`);
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
  {
    title: '创建时间',
    dataIndex: 'created_at',
    key: 'created_at',
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    field: 'field1',
    component: 'Input',
    label: '用户名',
    colProps: { span: 6 },
    componentProps: {
      onChange: (e: any) => {
        console.log(e);
      },
    },
  },
  {
    field: 'field2',
    component: 'Input',
    label: '昵称',
    colProps: { span: 6 },
  },
  {
    field: 'field3',
    component: 'Input',
    label: '邮箱',
    colProps: { span: 6 },
    //  componentProps: {
    //    options: [
    //      { label: '选项1', value: '1', key: '1' },
    //      { label: '选项2', value: '2', key: '2' },
    //    ],
    //  },
  },
  {
    field: 'field4',
    component: 'Input',
    label: '手机',
    colProps: { span: 6 },
  },
  {
    field: 'field5',
    component: 'Input',
    label: '身份证',
    colProps: { span: 6 },
  },
  {
    field: 'fieldTime',
    component: 'RangePicker',
    label: '时间字段',
    colProps: { span: 6 },
  },
];

export const formSchema: FormSchema[] = [
  {
    field: 'divider-basic',
    component: 'Divider',
    label: '登录信息',
    colProps: {
      span: 24,
    },
  },
  {
    field: 'roleName',
    label: '用户名',
    required: true,
    component: 'Input',
    colProps: {
      span: 12,
    },
  },
  {
    field: 'roleValue',
    label: '邮箱',
    required: false,
    component: 'Input',
    colProps: {
      span: 12,
    },
  },
  {
    field: 'roleValue',
    label: '手机',
    required: true,
    component: 'Input',
    colProps: {
      span: 12,
    },
  },
  {
    field: 'roleValue',
    label: '身份证',
    required: false,
    component: 'Input',
    colProps: {
      span: 12,
    },
  },
  {
    field: 'divider-basic',
    component: 'Divider',
    label: '基础信息',
    colProps: {
      span: 24,
    },
  },
  {
    field: 'roleValue',
    label: '昵称',
    required: true,
    component: 'Input',
    colProps: {
      span: 24,
    },
  },
  {
    field: '性别',
    label: '性别',
    component: 'CheckboxGroup',
    // 默认选项
    // defaultValue: '0',
    componentProps: {
      options: [
        { label: '男', value: '0' },
        { label: '女', value: '1' },
      ],
    },
    colProps: {
      span: 12,
    },
  },
  {
    field: 'status',
    label: '状态',
    component: 'CheckboxGroup',
    // 默认选项
    // defaultValue: '0',
    componentProps: {
      options: [
        { label: '启用', value: '0' },
        { label: '停用', value: '1' },
      ],
    },
    colProps: {
      span: 12,
    },
  },
  {
    field: '工作描述',
    label: '工作描述',
    component: 'InputTextArea',
    colProps: {
      span: 24,
    },
  },
];
