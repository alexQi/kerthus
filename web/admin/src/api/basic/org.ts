import { defHttp } from '/@/utils/http/axios';

enum Api {
  queryOrgTree = '/system/org/query',
  deleteOrgItem = '/system/org/delete',
  saveItem = '/system/org/save',
  getInfo = '/system/org/info',
}

export function queryOrgTree(params?: any) {
  return defHttp.get<any>({ url: Api.queryOrgTree, params }, { errorMessageMode: 'none' });
}

export function deleteItem(params) {
  return defHttp.get<any>({ url: Api.deleteOrgItem, params });
}

export function saveItem(params) {
  return defHttp.post<any>({ url: Api.saveItem, params }, { errorMessageMode: 'message' });
}

export function getInfo(params?: any) {
  return defHttp.get<any>({ url: Api.getInfo, params }, { errorMessageMode: 'none' });
}
