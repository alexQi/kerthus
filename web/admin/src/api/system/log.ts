import { defHttp } from '/@/utils/http/axios';

enum Api {
  Query = '/system/log/query',
}

export function getAuditLogList(params?: any) {
  return defHttp.get<any>({ url: Api.Query, params }, { errorMessageMode: 'none' });
}
