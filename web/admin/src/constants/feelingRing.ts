export const memberStatus = [
  {
    color: 'success',
    label: '正常',
    value: 1,
  },
  {
    color: 'warning',
    label: '锁定',
    value: 2,
  },
  {
    color: 'error',
    label: '注销',
    value: 3,
  },
];

export const activityStatus = [
  {
    color: '#fadb14',
    status: 'yellow',
    label: '待上线',
    value: 0,
  },
  {
    color: '#14aafa',
    status: 'cyan',
    label: '筹备中',
    value: 1,
  },
  {
    color: '#43de13',
    status: 'green',
    label: '已启动',
    value: 2,
  },
  {
    color: '#ff253e',
    status: 'red',
    label: '已结束',
    value: 4,
  },
  {
    color: '#7136ff',
    status: 'purple',
    label: '精彩回顾',
    value: 5,
  },
  {
    color: '#fa7414',
    status: 'orange',
    label: '已重置',
    value: 10,
  },
];

export const activityCodes = [
  {
    color: '#14aafa',
    status: 'cyan',
    label: '通用',
    value: 'general',
  },
  {
    color: '#fadb14',
    status: 'yellow',
    label: '遇见号列车',
    value: 'trip',
  },
  {
    color: '#43de13',
    status: 'green',
    label: '心动号飞船',
    value: 'spaceship',
  },
  {
    color: '#fa7414',
    status: 'orange',
    label: '完美陌生人',
    value: 'stranger',
  },
  {
    color: '#ff253e',
    status: 'red',
    label: '爱情买卖',
    value: 'shopping',
  },
  {
    color: '#7136ff',
    status: 'purple',
    label: '校园时光',
    value: 'school',
  },
];

export const groupStatus = [
  {
    color: '#fadb14',
    status: 'yellow',
    label: '暂停',
    value: 0,
  },
  {
    color: '#43de13',
    status: 'green',
    label: '正常',
    value: 1,
  },
  {
    color: '#14aafa',
    status: 'cyan',
    label: '互选中',
    value: 20,
  },
  {
    color: '#7136ff',
    status: 'purple',
    label: '互选结束',
    value: 30,
  },
  {
    color: '#ff253e',
    status: 'red',
    label: '结束',
    value: 100,
  },
];

export const paidType = [
  {
    color: '#14aafa',
    status: 'cyan',
    label: '支付宝',
    value: 'alipay',
  },
  {
    color: '#43de13',
    status: 'green',
    label: '微信',
    value: 'wechat',
  },
];

export const genderType = [
  {
    color: '#8a8a8a',
    status: 'default',
    label: '通用',
    value: 0,
  },
  {
    color: '#13c2c2',
    status: 'cyan',
    label: '男生',
    value: 1,
  },
  {
    color: '#eb2f96',
    status: 'pink',
    label: '女生',
    value: 2,
  },
];

export const topicType = [
  {
    color: '#8a8a8a',
    status: 'default',
    label: '通用',
    value: 'general',
  },
  {
    color: '#43de13',
    status: 'green',
    label: '小程序',
    value: 'weapp',
  },
  {
    color: '#1677ff',
    status: 'blue',
    label: '链接',
    value: 'link',
  },
];

export const socialType = [
  {
    color: '#2db7f5',
    status: 'cyan',
    label: '微信小程序',
    value: 'wechat@weapp',
  },
  {
    color: '#87d068',
    status: 'green',
    label: '微信公众号',
    value: 'wechat@official',
  },
  {
    color: '#108ee9',
    status: 'green',
    label: '企业微信',
    value: 'wechat@work',
  },
];

export const orderStatus = [
  {
    color: '#fadb14',
    status: 'yellow',
    label: '已取消',
    value: 0,
  },
  {
    color: '#fa7414',
    status: 'orange',
    label: '待付款',
    value: 10,
  },
  {
    color: '#43de13',
    status: 'green',
    label: '已支付',
    value: 20,
  },
  {
    color: '#ff253e',
    status: 'red',
    label: '已退款',
    value: 100,
  },
];

export const refundStatus = [
  {
    color: '#13deaf',
    status: 'yellow',
    label: '未退款',
    value: 0,
  },
  {
    color: '#fadb14',
    status: 'orange',
    label: '待审核',
    value: 10,
  },
  {
    color: '#14aafa',
    status: 'cyan',
    label: '退款中',
    value: 20,
  },
  {
    color: '#fa7414',
    status: 'green',
    label: '已退款',
    value: 40,
  },
  {
    color: '#ff253e',
    status: 'red',
    label: '退款失败',
    value: 50,
  },
  {
    color: '#7136ff',
    status: 'purple',
    label: '取消退款',
    value: 100,
  },
];

export const itemStatus = {
  1: [
    {
      color: '#ff253e',
      status: 'red',
      label: '未邀约',
      value: 0,
    },
    {
      color: '#fab114',
      status: 'orange',
      label: '待邀约',
      value: 10,
    },
    {
      color: '#7136ff',
      status: 'purple',
      label: '已邀约',
      value: 20,
    },
    {
      color: '#14aafa',
      status: 'cyan',
      label: '已签到',
      value: 25,
    },
    {
      color: '#43de13',
      status: 'green',
      label: '已匹配',
      value: 30,
    },
  ],
  2: [
    {
      color: '#ff253e',
      status: 'red',
      label: '未加入',
      value: 0,
    },
    {
      color: '#fa7414',
      status: 'orange',
      label: '参与中',
      value: 10,
    },
    {
      color: '#7136ff',
      status: 'purple',
      label: '待匹配',
      value: 20,
    },
    {
      color: '#43de13',
      status: 'green',
      label: '已匹配',
      value: 30,
    },
  ],
};
