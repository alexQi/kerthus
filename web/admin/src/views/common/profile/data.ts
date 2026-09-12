import { FormSchema } from '/@/components/Form';

export interface ListItem {
  key: string;
  title: string;
  description: string;
  extra?: string;
  avatar?: string;
  color?: string;
}

// tab的list
export const settingList = [
  {
    key: '1',
    name: '基本设置',
    component: 'BaseSetting',
  },
  {
    key: '2',
    name: '安全设置',
    component: 'SecureSetting',
  },
];

// 基础设置 form
export const baseSetschemas: FormSchema[] = [
  {
    field: 'phone',
    component: 'Input',
    label: '联系电话',
    componentProps: {
      disabled: true,
    },
    colProps: { span: 18 },
  },
  {
    field: 'email',
    component: 'Input',
    label: '登录邮箱',
    rules: [{ type: 'email', message: '请输入正确的邮箱地址' }],
    colProps: { span: 18 },
  },
  {
    field: 'name',
    component: 'Input',
    label: '昵称',
    required: true,
    componentProps: { maxlength: 100 },
    colProps: { span: 18 },
  },
];

export const passwordFormSchema: FormSchema[] = [
  {
    field: 'old_password',
    label: '当前密码',
    component: 'InputPassword',
    required: true,
  },
  {
    field: 'password',
    label: '新密码',
    component: 'StrengthMeter',
    componentProps: {
      placeholder: '请输入 10 至 72 字节的新密码',
    },
    rules: [
      {
        required: true,
        message: '请输入新密码',
      },
      {
        validator: (_, value) => {
          const length = new TextEncoder().encode(value || '').length;
          return length >= 10 && length <= 72
            ? Promise.resolve()
            : Promise.reject('密码长度应为 10 至 72 字节');
        },
      },
    ],
  },
  {
    field: 'confirmPassword',
    label: '确认密码',
    component: 'InputPassword',

    dynamicRules: ({ values }) => {
      return [
        {
          required: true,
          validator: (_, value) => {
            if (!value) {
              return Promise.reject('密码不能为空');
            }
            if (value !== values.password) {
              return Promise.reject('两次输入的密码不一致!');
            }
            return Promise.resolve();
          },
        },
      ];
    },
  },
];
