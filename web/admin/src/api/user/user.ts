import { defHttp } from '/@/utils/http/axios';

enum Api {
  IndexList = '/system/user/query',
  setStatus = '/system/user/setStatus',
  modifyInfo = '/system/user/modifyInfo',
  modifyPassword = '/system/user/modifyPassword',
  resetPassword = '/system/user/resetPassword',
}

export function getList(params?: any) {
  return defHttp.get<any>({ url: Api.IndexList, params }, { errorMessageMode: 'none' });
}

export function setStatus(id: number, status: any) {
  return defHttp.get<any>(
    { url: Api.setStatus, params: { id, status } },
    { errorMessageMode: 'none' },
  );
}

export function modifyPassword(params) {
  return defHttp.post<any>({ url: Api.modifyPassword, params });
}

export function modifyInfo(params) {
  return defHttp.post<any>({ url: Api.modifyInfo, params });
}

export function resetPassword(params) {
  return defHttp.post<any>({ url: Api.resetPassword, params }, { errorMessageMode: 'message' });
}
