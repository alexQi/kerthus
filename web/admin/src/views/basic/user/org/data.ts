import { h } from 'vue';
import { FormSchema } from '/@/components/Table';
import { Input } from 'ant-design-vue';

export const formSchema: FormSchema[] = [
  {
    field: 'id',
    label: 'Id',
    required: false,
    component: 'Input',
    show: false,
  },
  {
    field: 'unit_id',
    label: '单位ID',
    required: true,
    defaultValue: 0,
    component: 'Input',
    show: false,
  },
  {
    field: 'parent_id',
    label: '父级ID',
    required: true,
    defaultValue: 0,
    component: 'Input',
    show: false,
  },
  {
    field: 'parent_name',
    label: '父机构',
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
  },
  {
    field: 'type',
    label: '类型',
    component: 'RadioButtonGroup',
    required: true,
    defaultValue: 'unit',
    componentProps: {
      options: [
        { label: '单位/门店', value: 'unit' },
        { label: '部门', value: 'section' },
      ],
    },
  },
  {
    field: 'name',
    label: '名称',
    component: 'Input',
    required: true,
  },
  {
    field: 'short_name',
    label: '简称',
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
    field: 'sort',
    label: '排序',
    defaultValue: 0,
    component: 'InputNumber',
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
