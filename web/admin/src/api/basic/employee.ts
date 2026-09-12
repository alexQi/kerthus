import { defHttp } from '/@/utils/http/axios';

enum Api {
  IndexList = '/system/employee/query',
  Save = '/system/employee/save',
  Delete = '/system/employee/delete',
  Info = '/system/employee/info',
  getTenantApps = '/system/employee/getTenantApps',
  queryTenantApps = '/system/employee/queryTenantApps',
  hasApp = '/system/employee/hasApp',
}

export function getList(params?: any) {
  return defHttp.get<any>({ url: Api.IndexList, params }, { errorMessageMode: 'none' });
}

export function getInfo(params?: any) {
  return defHttp.get<any>({ url: Api.Info, params }, { errorMessageMode: 'none' });
}

export function getTenantApps(params?: any) {
  return defHttp.get<any>({ url: Api.getTenantApps, params }, { errorMessageMode: 'none' });
}

export function queryTenantApps(params?: any) {
  return defHttp.get<any>({ url: Api.queryTenantApps, params }, { errorMessageMode: 'none' });
}

export function hasApp(params?: any) {
  return defHttp.get<any>({ url: Api.hasApp, params }, { errorMessageMode: 'none' });
}

export function saveData(params: any) {
  return defHttp.post<any>({ url: Api.Save, params }, { errorMessageMode: 'message' });
}

export function setStatus(id: number, status: number, tenant_id?: number) {
  return defHttp.get({ url: '/system/employee/setStatus', params: { id, status, tenant_id } });
}
