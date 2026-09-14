import type { BasicColumn, FormSchema } from '/@/components/Table';

export const columns: BasicColumn[] = [
  { title: '名称 / 标识', dataIndex: 'name', key: 'name', align: 'left', width: 190 },
  { title: '类型', dataIndex: 'provider', key: 'provider', width: 120 },
  { title: '接口地址', dataIndex: 'endpoint', key: 'endpoint', width: 220 },
  { title: '默认模型', dataIndex: 'model', key: 'model', width: 160 },
  { title: '状态', dataIndex: 'enabled', key: 'status', width: 110 },
  { title: '支持模型', dataIndex: 'models', key: 'models', width: 110 },
];

export const searchFormSchema: FormSchema[] = [
  {
    field: 'keyword',
    component: 'Input',
    label: '关键词',
    componentProps: { placeholder: '名称、标识或接口地址', allowClear: true },
    colProps: { span: 8 },
  },
];

export const formSchema: FormSchema[] = [
  { field: '_id', label: 'Id', component: 'Input', show: false },
  { field: 'models', label: '模型列表', component: 'Input', show: false },
  {
    field: 'name',
    label: '名称',
    component: 'Input',
    required: true,
    componentProps: { placeholder: '例如 OpenAI 主账号' },
  },
  {
    field: 'code',
    label: '标识',
    component: 'Input',
    required: true,
    componentProps: { placeholder: '例如 openai-main' },
  },
  {
    field: 'provider',
    label: '服务商类型',
    component: 'Select',
    required: true,
    componentProps: { options: [{ value: 'openai', label: 'OpenAI 兼容' }] },
  },
  {
    field: 'endpoint',
    label: '接口地址',
    component: 'Input',
    required: true,
    componentProps: { placeholder: 'https://api.openai.com' },
  },
  {
    field: 'api_key',
    label: 'API 密钥',
    component: 'InputPassword',
    componentProps: { placeholder: '留空保持原密钥' },
  },
  {
    field: 'model',
    label: '默认模型',
    component: 'Select',
    componentProps: ({ formModel }) => ({
      options: (formModel.models || []).map((model: any) => ({
        value: model.id || model,
        label: model.name || model.id || model,
      })),
      placeholder: '请先同步模型',
      showSearch: true,
      allowClear: true,
    }),
  },
  { field: 'enabled', label: '启用状态', component: 'Switch', defaultValue: true },
  { field: 'default', label: '设为默认', component: 'Switch', defaultValue: false },
];
