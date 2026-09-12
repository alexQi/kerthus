import { h } from 'vue';
import { BasicColumn, FormSchema } from '/@/components/Table';
import { CodeEditor, MODE } from '/@/components/CodeEditor';
import { Input } from 'ant-design-vue';
import { externalUrl } from '/@/utils/externalUrl';
import { AppRoutes, AppServers } from '/@/api/app/info';

export const apiColumns: BasicColumn[] = [
  {
    title: '控制器',
    dataIndex: 'controller',
    key: 'controller',
    align: 'left',
  },
  {
    title: '接口地址',
    dataIndex: 'uri',
    key: 'uri',
    align: 'left',
  },
];

export const apiFormSchema: FormSchema[] = [
  {
    field: 'server',
    label: '接口服务',
    required: true,
    component: 'ApiSelect',
    componentProps: {
      api: AppServers,
      labelField: 'name',
      valueField: 'name',
    },
  },
  {
    field: 'interfaces',
    label: '已选接口',
    show: false,
    required: false,
    component: 'Select',
    componentProps: {
      options: [],
      mode: 'multiple',
      placeholder: '请选择接口地址',
    },
  },
  {
    field: 'controller',
    label: '控制器类',
    required: true,
    component: 'ApiSelect',
    componentProps: ({ formModel, formActionType }) => {
      return {
        api: (params) =>
          params?.server ? AppRoutes(params) : Promise.resolve({ classes: [], actions: {} }),
        labelField: 'name',
        valueField: 'name',
        resultField: 'classes',
        params: {
          server: formModel.server,
        },
        onValChange: (val, fetchVal) => {
          const { updateSchema } = formActionType;
          const options: any[] = [];
          const actions = fetchVal?.actions?.[val] || [];
          for (const key in actions) {
            options.push({
              label: actions[key].uri,
              value: actions[key].method + '#' + actions[key].action,
            });
          }
          updateSchema({
            field: 'uris',
            componentProps: {
              options: options,
              mode: 'multiple',
              placeholder: '请选择接口地址',
              onChange: (_, e) => {
                formModel.interfaces = e;
              },
            },
          });
        },
      };
    },
  },
  {
    field: 'uris',
    label: '接口地址',
    component: 'Select',
    required: true,
    componentProps: {
      options: [],
      mode: 'multiple',
      placeholder: '请选择接口地址',
    },
  },
];

export const formSchema: FormSchema[] = [
  {
    field: 'divider-basic',
    component: 'Divider',
    label: '基础信息',
    helpMessage: '定义资源基本属性',
    colProps: {
      span: 24,
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'id',
    label: 'Id',
    required: false,
    component: 'Input',
    show: false,
  },
  {
    field: 'app_id',
    label: 'app_id',
    required: true,
    component: 'Input',
    show: false,
  },
  {
    field: 'parent_id',
    defaultValue: 0,
    label: '父级ID',
    required: true,
    component: 'Input',
    show: false,
  },
  {
    field: 'type',
    label: '菜单类型',
    component: 'RadioGroup',
    required: true,
    defaultValue: 'menu',
    componentProps: ({ formModel }) => {
      return {
        options: [
          { label: '菜单', value: 'menu' },
          { label: '视图', value: 'view' },
          { label: '功能', value: 'action' },
          { label: '字段', value: 'field' },
        ],
        onChange: (e) => {
          const type = typeof e === 'object' ? e.target.value : e;
          switch (type) {
            case 'menu':
              formModel.icon = 'ant-design:appstore-outlined';
              break;
            case 'view':
              formModel.icon = 'mdi:eye-settings';
              break;
            case 'action':
              formModel.icon = 'mdi:alpha-c-box';
              break;
            case 'field':
              formModel.icon = 'mdi:focus-field';
              break;
          }
        },
      };
    },
    colProps: {
      lg: 12,
      md: 12,
      span: 12,
    },
  },
  {
    field: 'parent_name',
    label: '父资源',
    component: 'Input',
    required: false,
    render: ({ model, field }) => {
      return h(Input, {
        placeholder: '请输入',
        disabled: true,
        value: model.parent_id > 0 ? model[field] : '根节点',
        onChange: (e: ChangeEvent) => {
          model[field] = e.target.value;
        },
      });
    },
    colProps: {
      span: 12,
    },
  },
  {
    field: 'code',
    label: '编码',
    component: 'Input',
    required: true,
    colProps: {
      span: 12,
    },
  },
  {
    field: 'name',
    label: '名称',
    component: 'Input',
    required: true,
    colProps: {
      span: 12,
    },
  },
  {
    field: 'icon',
    label: '图标',
    component: 'Input',
    defaultValue: 'ant-design:appstore-outlined',
    componentProps: { placeholder: '例如 ant-design:appstore-outlined' },
    show: ({ model }) => {
      return model.type === 'menu';
    },
    required: ({ model }) => {
      return model.type === 'menu';
    },
  },
  {
    field: 'sort',
    label: '排序',
    defaultValue: 0,
    component: 'InputNumber',
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
    field: 'is_public',
    label: '公共资源',
    defaultValue: false,
    component: 'Switch',
    required: false,
  },
  {
    field: 'divider-special',
    component: 'Divider',
    label: '特性信息',
    show: ({ model }) => {
      return ['menu', 'view'].indexOf(model.type) >= 0;
    },
    helpMessage: '每种类型拥有不同的字段',
    colProps: {
      span: 24,
    },
  },
  {
    field: 'open_with',
    label: '打开方式',
    helpMessage: '组件加载站内页面；内链在页面内打开网站；外链在新窗口打开网站。',
    component: 'RadioGroup',
    show: ({ model }) => {
      return ['menu', 'view'].indexOf(model.type) >= 0;
    },
    required: ({ model }) => {
      return ['menu', 'view'].indexOf(model.type) >= 0;
    },
    defaultValue: 'component',
    componentProps: ({ formModel }) => {
      return {
        options: [
          { label: '组件', value: 'component' },
          { label: '内链', value: 'inside' },
          { label: '外链', value: 'outside' },
        ],
        onChange: (e: any) => {
          if (e.hasOwnProperty('target')) {
            if (e.target.value === 'inside') {
              if (!externalUrl(formModel.component)) formModel.component = '';
            } else if (e.target.value === 'component' && externalUrl(formModel.component)) {
              formModel.component = 'LAYOUT';
            }
          }
        },
      };
    },
    colProps: {
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'path',
    label: '路由/外链地址',
    helpMessage: '组件和内链填写站内路径；外链填写完整的 HTTP(S) 地址。',
    dynamicRules: ({ model }) =>
      ['menu', 'view'].includes(model.type)
        ? [
            {
              required: true,
              validator: async (_rule, value) => {
                if (model.open_with === 'outside') {
                  if (!externalUrl(value)) throw new Error('请输入完整的 HTTP(S) 外链地址');
                } else if (
                  typeof value !== 'string' ||
                  !value.startsWith('/') ||
                  value.startsWith('//') ||
                  /[\\\s?#]/.test(value)
                ) {
                  throw new Error('请输入以 / 开头的站内路由路径');
                }
              },
            },
          ]
        : [],
    component: 'Input',
    required: ({ model }) => {
      return ['menu', 'view'].indexOf(model.type) >= 0;
    },
    show: ({ model }) => {
      return ['menu', 'view'].indexOf(model.type) >= 0;
    },
  },
  {
    field: 'component',
    label: '组件/内链地址',
    helpMessage: '组件填写组件路径；内链填写完整的 HTTP(S) 网站地址。',
    dynamicRules: ({ model }) =>
      ['menu', 'view'].includes(model.type) && model.open_with !== 'outside'
        ? [
            {
              required: true,
              validator: async (_rule, value) => {
                if (model.open_with === 'inside' ? !externalUrl(value) : !value)
                  throw new Error(
                    model.open_with === 'inside'
                      ? '请输入完整的 HTTP(S) 内链地址'
                      : '请输入组件路径',
                  );
              },
            },
          ]
        : [],
    component: 'Input',
    required: ({ model }) => {
      return ['menu', 'view'].includes(model.type) && model.open_with !== 'outside';
    },
    defaultValue: 'LAYOUT',
    show: ({ model }) => {
      return ['menu', 'view'].includes(model.type) && model.open_with !== 'outside';
    },
  },
  {
    field: 'redirect',
    label: '重定向',
    component: 'Input',
    show: ({ model }) => {
      return ['menu', 'view'].indexOf(model.type) >= 0;
    },
  },
  {
    field: 'divider-extend',
    component: 'Divider',
    label: '扩展信息',
    helpMessage: '包含元数据及备注',
    colProps: {
      span: 24,
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'meta',
    label: '元数据',
    component: 'Checkbox',
    componentProps: {},
    render: ({ model, values }) => {
      const editable = !!values.resourceEditable;
      return h(CodeEditor, {
        key: editable ? 'editable-meta' : 'readonly-meta',
        readonly: !editable,
        value: model.meta ? model.meta : {},
        mode: MODE.JSON,
        style: 'border: 1px solid #d9d9d9;padding: 10px;',
        onChange(val) {
          if (editable) model.meta = val;
        },
      });
    },
    colProps: {
      lg: 24,
      md: 24,
    },
  },
  {
    field: 'remark',
    label: '备注',
    required: false,
    component: 'InputTextArea',
    componentProps: {
      resize: 'none',
    },
    colProps: {
      lg: 24,
      md: 24,
    },
  },
];
