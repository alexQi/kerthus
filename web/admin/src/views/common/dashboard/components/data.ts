interface NavItem {
  title: string;
  icon: string;
  color: string;
}

interface DynamicInfoItem {
  avatar: string;
  name: string;
  date: string;
  desc: string;
}

export const navItems: NavItem[] = [
  {
    title: '首页',
    icon: 'ion:home-outline',
    color: '#1fdaca',
  },
  {
    title: '仪表盘',
    icon: 'ion:grid-outline',
    color: '#bf0c2c',
  },
  {
    title: '组件',
    icon: 'ion:layers-outline',
    color: '#e18525',
  },
  {
    title: '系统管理',
    icon: 'ion:settings-outline',
    color: '#3fb27f',
  },
  {
    title: '权限管理',
    icon: 'ion:key-outline',
    color: '#4daf1bc9',
  },
  {
    title: '图表',
    icon: 'ion:bar-chart-outline',
    color: '#00d8ff',
  },
];

export const dynamicInfoItems: DynamicInfoItem[] = [
  {
    avatar: 'dynamic-avatar-1|svg',
    name: '研发No.1',
    date: '2023-04-01',
    desc: ` 创建了应用 <a>研发运营平台</a>,用于研发人员管理该saas系统`,
  },
  {
    avatar: 'dynamic-avatar-3|svg',
    name: '研发No.1',
    date: '2023-04-10',
    desc: ` 创建了应用 <a>基础平台</a>，用于租户基本信息设计，类似组织架构，员工管理，角色权限`,
  },
  {
    avatar: 'dynamic-avatar-3|svg',
    name: '研发No.1',
    date: '2023-04-18',
    desc: ` 创建了应用 <a>CRM客户管理系统</a>，专注服务与婚恋行业的CRM`,
  },
];
