export const customerOptions = {
  is_deal: [
    {
      color: 'warning',
      text: '未成交',
    },
    {
      color: 'success',
      text: '已成交',
    },
    {
      color: 'error',
      text: '已退费',
    },
  ],
  contract_status: [
    {
      value: 0,
      color: '#faad14',
      status: 'warning',
      label: '未成交',
    },
    {
      value: 1,
      color: '#52c41a',
      status: 'success',
      label: '生效中',
    },
    {
      value: 2,
      color: '#ff4d4f',
      status: 'error',
      label: '已终止',
    },
  ],
  follow_status: [
    {
      label: '新客户',
      value: 'F000',
      color: '#1890FF',
    },
    {
      label: '未接通，待跟进',
      value: 'F010',
      color: '#FF85C0',
    },
    {
      label: '已接通，未深沟',
      value: 'F020',
      color: '#EB2F96',
    },
    {
      label: '已接通，已深沟',
      value: 'F030',
      color: '#C41D7F',
    },
    {
      label: '确定到店时间',
      value: 'F040',
      color: '#FAAD14',
    },
    {
      label: '未到店，再跟进',
      value: 'F050',
      color: '#36CFC9',
    },
    {
      label: '已到店，未签约',
      value: 'F060',
      color: '#13A8A8',
    },
    {
      label: '已到店，已签约',
      value: 'F070',
      color: '#52C41A',
      notice: '当前客户已签约，请去创建合同',
      action: 'handleAddContract',
    },
    {
      label: '无效资源，放弃',
      value: 'F100',
      color: '#F5222D',
      notice: '当前客户已被标记为无效资源，是否放入公海',
      action: 'handlePutInto',
    },
  ],
  price_type: [
    {
      label: '全款',
      value: 'totalPayment',
    },
    {
      label: '定金',
      value: 'earnest',
    },
    {
      label: '尾款',
      value: 'finalPayment',
    },
    {
      label: '部分',
      value: 'part',
    },
  ],
  level: [
    {
      label: 'S+级',
      value: 'S+',
    },
    {
      label: 'S级',
      value: 'S',
    },
    {
      label: 'A级',
      value: 'A',
    },
    {
      label: 'B级',
      value: 'B',
    },
    {
      label: 'C级',
      value: 'C',
    },
    {
      label: 'D级',
      value: 'D',
    },
    {
      label: 'N级',
      value: 'N',
    },
  ],
  education: [
    {
      label: '高中及以下',
      value: '高中及以下',
    },
    {
      label: '大学专科',
      value: '大学专科',
    },
    {
      label: '大学本科',
      value: '大学本科',
    },
    {
      label: '研究生',
      value: '研究生',
    },
    {
      label: '硕士',
      value: '硕士',
    },
    {
      label: '博士',
      value: '博士',
    },
  ],
  marital_status: [
    {
      label: '未婚',
      value: '未婚',
    },
    {
      label: '离异',
      value: '离异',
    },
    {
      label: '丧偶',
      value: '丧偶',
    },
  ],
  has_house: [
    {
      label: '-',
      value: -1,
    },
    {
      label: '无房',
      value: 0,
    },
    {
      label: '有房',
      value: 1,
    },
  ],
  has_car: [
    {
      label: '-',
      value: -1,
    },
    {
      label: '无车',
      value: 0,
    },
    {
      label: '有车',
      value: 1,
    },
  ],
  income: [
    // {
    //   label: '-',
    //   value: '-1',
    // },
    {
      label: '5w以下',
      value: '0-50000',
    },
    {
      label: '5w-10w',
      value: '50000-100000',
    },
    {
      label: '10w-20w',
      value: '100000-200000',
    },
    {
      label: '20w-30w',
      value: '200000-300000',
    },
    {
      label: '30w以上',
      value: '300000',
    },
    // {
    //   label: '不限',
    //   value: '0',
    // },
  ],
  personality: [
    {
      label: '活泼开朗',
      value: '活泼开朗',
    },
    {
      label: '安静内敛',
      value: '安静内敛',
    },
    {
      label: '理智冷静',
      value: '理智冷静',
    },
    {
      label: '温柔沉静',
      value: '温柔沉静',
    },
    {
      label: '严肃认真',
      value: '严肃认真',
    },
    {
      label: '情感丰富',
      value: '情感丰富',
    },
  ],
  want_child: [
    {
      label: '-',
      value: 0,
    },
    {
      label: '视情况而定',
      value: 1,
    },
    {
      label: '想要孩子',
      value: 2,
    },
    {
      label: '丁克',
      value: 3,
    },
  ],
  follow_method: [
    {
      label: '到店',
      value: 'face',
    },
    {
      label: '电话',
      value: 'tel',
    },
    {
      label: '微信',
      value: 'wechat',
    },
    {
      label: '短信',
      value: 'sms',
    },
    {
      label: '外出',
      value: 'outside',
    },
    {
      label: '其他',
      value: 'other',
    },
  ],
  appeal_reason: [
    {
      value: 0,
      label: '空号',
    },
    {
      value: 1,
      label: '错号（非本人）',
    },
    {
      value: 2,
      label: '无法接通（已拨5次）',
    },
    {
      value: 3,
      label: '明确表明不是单身',
    },
    {
      value: 4,
      label: '不需要服务',
    },
  ],
  check_status: [
    {
      value: 0,
      color: '#faad14',
      status: 'warning',
      label: '待审核',
    },
    {
      value: 1,
      color: '#52c41a',
      status: 'success',
      label: '已通过',
    },
    {
      value: 2,
      color: '#ff4d4f',
      status: 'error',
      label: '已拒绝',
    },
  ],
  purge_status: [
    {
      value: 0,
      color: '#e5b134',
      status: 'warning',
      label: '等待清洗',
    },
    {
      value: 1,
      color: '#52c41a',
      status: 'success',
      label: '清洗通过',
    },
    {
      value: 2,
      color: '#4d8bff',
      status: 'error',
      label: '模糊意向',
    },
    {
      value: 100,
      color: '#ff4d4f',
      status: 'error',
      label: '无效资源',
    },
  ],
};
