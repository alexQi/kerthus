import { BasicColumn, FormSchema } from '/@/components/Table';
import { h } from 'vue';
import { Image, Switch } from 'ant-design-vue';
import { useMessage } from '/@/hooks/web/useMessage';
import { setStatus } from '/@/api/application/application';
import { usePermission } from '/@/hooks/web/usePermission';
import { uploadApi } from '/@/api/app/upload';
import { externalUrl } from '/@/utils/externalUrl';
import { getImageSrc } from '/@/utils/file/resource';

export const columns: BasicColumn[] = [
  {
    title: 'ID',
    dataIndex: 'id',
    key: 'id',
    width: 80,
  },
  {
    title: '应用标识',
    dataIndex: 'code',
    key: 'code',
    align: 'left',
  },
  {
    title: '图标',
    dataIndex: 'icon',
    width: 80,
    customRender: ({ record }) =>
      h(Image, {
        src: getImageSrc(record.icon) || '/resource/img/logo.png',
        fallback: '/resource/img/logo.png',
        width: 40,
        height: 40,
        preview: false,
        style: { objectFit: 'contain' },
      }),
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
  { title: '类型', dataIndex: 'type', key: 'type', width: 100 },
  { title: '公共应用', dataIndex: 'is_public', key: 'is_public', width: 100 },
  {
    title: '状态',
    dataIndex: 'status',
    key: 'status',
    width: 150,
    customRender: ({ record }) => {
      if (!Reflect.has(record, 'pendingStatus')) {
        record.pendingStatus = false;
      }
      const { hasPermission } = usePermission();
      return h(Switch, {
        disabled: !hasPermission(['system:application:main:setStatus']),
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
              createMessage.success(`已成功修改应用状态`);
            })
            .catch((e) => {
              createMessage.error(e.message ? e.message : '修改应用状态失败');
            })
            .finally(() => {
              record.pendingStatus = false;
            });
        },
      });
    },
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    field: 'code',
    component: 'Input',
    label: '应用标识',
    colProps: { span: 6 },
  },
  {
    field: 'name',
    component: 'Input',
    label: '应用名称',
    colProps: { span: 6 },
  },
];

export const formSchema: FormSchema[] = [
  {
    field: 'icon',
    label: '应用图标',
    component: 'DefaultUpload',
    defaultValue: [],
    componentProps: {
      api: uploadApi,
      multiple: false,
      maxNumber: 1,
      accept: ['image/*'],
      width: '96px',
      height: '96px',
      objectFit: 'contain',
      uploadParams: { directory: 'application', param: 'icon' },
    },
  },
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
    field: 'code',
    label: '应用CODE',
    required: true,
    component: 'Input',
    componentProps: ({ formModel }) => {
      return {
        disabled: formModel.id > 0,
      };
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
    defaultValue: 'self',
    required: true,
    componentProps: {
      options: [
        { label: '自建应用', value: 'self' },
        { label: '第三方应用', value: 'third' },
      ],
    },
  },
  {
    field: 'url',
    label: '应用地址',
    component: 'Input',
    show: ({ model }) => model.type === 'third',
    dynamicRules: ({ model }) =>
      model.type === 'third'
        ? [
            {
              required: true,
              validator: async (_rule, value) => {
                if (!externalUrl(value)) throw new Error('请输入完整的 HTTP(S) 应用地址');
              },
            },
          ]
        : [],
    helpMessage: '第三方应用在新窗口打开，仍需有效的租户授权。',
  },
  {
    field: 'is_public',
    label: '公共应用',
    component: 'RadioGroup',
    defaultValue: 0,
    componentProps: {
      options: [
        { label: '否', value: 0 },
        { label: '是', value: 1 },
      ],
    },
    helpMessage: '仅标记应用属性，不会自动授予租户访问权限。',
  },
  { field: 'desc', label: '简介', component: 'InputTextArea' },
  { field: 'remark', label: '备注', component: 'InputTextArea' },
];
