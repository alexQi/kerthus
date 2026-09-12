import { defHttp } from '/@/utils/http/axios';

enum Api {
  IndexList = '/system/position/query',
  Save = '/system/position/save',
  Delete = '/system/position/delete',
  setStatus = '/system/position/setStatus',
  getItems = '/system/position/getItems',
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

export function setStatus(id: number, status: any, tenant_id?: number) {
  return defHttp.get<any>(
    { url: Api.setStatus, params: { id, status, tenant_id } },
    { errorMessageMode: 'none' },
  );
}

export function getItems(params?: any) {
  return defHttp.get<any>({ url: Api.getItems, params }, { errorMessageMode: 'none' });
}
