import { BasicColumn, FormSchema } from '/@/components/Table';
import { h } from 'vue';
import { Image } from 'ant-design-vue';
import { getImageSrc } from '/@/utils/file/resource';

export const columns: BasicColumn[] = [
  {
    title: 'ID',
    dataIndex: 'id',
    key: 'id',
    width: 80,
  },
  {
    title: '应用名称',
    dataIndex: 'name',
    key: 'name',
    align: 'left',
  },
  {
    title: '版本',
    dataIndex: 'version',
    key: 'version',
    width: 120,
  },
  {
    title: '类型',
    dataIndex: 'type',
    key: 'type',
    width: 120,
  },
  {
    title: '过期时间',
    dataIndex: 'expiration_time',
    key: 'expiration_time',
    width: 200,
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    field: 'name',
    component: 'Input',
    label: '应用名称',
    colProps: { span: 6 },
  },
];

export const formSchema: FormSchema[] = [
  {
    field: 'id',
    label: 'Id',
    required: false,
    component: 'Input',
    show: false,
    colProps: {
      span: 24,
    },
  },
  {
    field: 'icon',
    label: '图标',
    required: true,
    component: 'Input',
    render: ({ model }) => {
      return h(Image, {
        src: getImageSrc(model.icon),
        fallback: '/resource/img/logo.png',
      });
    },
  },
  {
    field: 'name',
    label: '应用名称',
    required: true,
    component: 'Input',
  },
  {
    field: 'version',
    label: '版本',
    required: true,
    component: 'Input',
  },
  {
    field: 'type',
    label: '应用类型',
    component: 'Select',
    required: true,
    defaultValue: 'self',
    componentProps: {
      options: [
        {
          label: '自建应用',
          value: 'self',
        },
        {
          label: '第三方应用',
          value: 'third',
        },
      ],
    },
  },
  {
    field: 'url',
    label: '应用地址',
    component: 'Input',
    show: ({ model }) => model.type === 'third',
  },
  {
    field: 'desc',
    label: '简介',
    component: 'Input',
  },
  {
    field: 'remark',
    label: '备注',
    component: 'InputTextArea',
  },
];
