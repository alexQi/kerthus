import { defHttp } from '/@/utils/http/axios';

enum Api {
  IndexList = '/system/app/query',
  Save = '/system/app/save',
  Delete = '/system/app/delete',
  setStatus = '/system/app/setStatus',
  saveResource = '/system/app/saveResource',
  removeResource = '/system/app/removeResource',
  getAppItems = '/system/app/getAppItems',
  getGlobalResource = '/system/app/getGlobalResource',
  getResource = '/system/app/getResource',
  getAppResources = '/system/app/getAppResources',
  queryResourceApis = '/system/app/queryResourceApis',
  getTenantResourceIds = '/system/app/getTenantResourceIds',
  getTenantResources = '/system/app/getTenantResources',
}

export function getList(params) {
  return defHttp.get<any>({ url: Api.IndexList, params }, { errorMessageMode: 'none' });
}

export function saveData(params) {
  return defHttp.post<any>({ url: Api.Save, params }, { errorMessageMode: 'message' });
}

export function deleteData(params) {
  return defHttp.get<any>({ url: Api.Delete, params }, { errorMessageMode: 'message' });
}

export function setStatus(id: number, status: any) {
  return defHttp.get<any>(
    { url: Api.setStatus, params: { id, status } },
    { errorMessageMode: 'none' },
  );
}

export function saveResource(params) {
  return defHttp.post<any>({ url: Api.saveResource, params }, { errorMessageMode: 'message' });
}

export function removeResource(params) {
  return defHttp.get<any>({ url: Api.removeResource, params });
}

export function getAppItems(params?: any) {
  return defHttp.get<any>({ url: Api.getAppItems, params }, { errorMessageMode: 'none' });
}

export function getGlobalResource() {
  return defHttp.get<any>({ url: Api.getGlobalResource }, { errorMessageMode: 'message' });
}

export function getAppResources(params?: any) {
  return defHttp.get<any>({ url: Api.getAppResources, params }, { errorMessageMode: 'none' });
}

export function getResource(params?: any) {
  return defHttp.get<any>({ url: Api.getResource, params }, { errorMessageMode: 'none' });
}

export function queryResourceApis(params?: any) {
  return defHttp.get<any>({ url: Api.queryResourceApis, params }, { errorMessageMode: 'none' });
}

export function getTenantResourceIds(params?: any) {
  return defHttp.get<any>({ url: Api.getTenantResourceIds, params }, { errorMessageMode: 'none' });
}

export function getTenantResources(params?: any) {
  return defHttp.get<any>({ url: Api.getTenantResources, params }, { errorMessageMode: 'message' });
}
