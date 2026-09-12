import { MockMethod } from 'vite-plugin-mock';
import { resultSuccess } from '../utils';

const getList = {
  items: [
    {
      uid: '152435',
      avatar: 'user_header/152435/1688624052ibQmIq.jpg',
      invite_code: '6cf437',
      last_time: '1688624019',
      nickname: '冰冰',
      now_money: '0',
      now_point: '0',
      phone: '17765028241',
      sex: '2',
      user_status: '1',
    },
  ],
};
export default [
  {
    url: '/basic-api/activity/query',
    timeout: 1000,
    method: 'get',
    response: () => {
      return resultSuccess(getList);
    },
  },
] as MockMethod[];
