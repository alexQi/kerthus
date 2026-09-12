import { defHttp } from '/@/utils/http/axios';

enum Api {
  IndexList = '/system/tenant/query',
  Save = '/system/tenant/save',
  Info = '/system/tenant/info',
  Delete = '/system/tenant/delete',
  setStatus = '/system/tenant/setStatus',
  verify = '/system/tenant/approve',
}

export function getList(params) {
  return defHttp.get<any>({ url: Api.IndexList, params }, { errorMessageMode: 'none' });
}

export function getInfo(params) {
  return defHttp.get<any>({ url: Api.Info, params }, { errorMessageMode: 'none' });
}

export function saveData(params) {
  return defHttp.post<any>({ url: Api.Save, params }, { errorMessageMode: 'message' });
}

export function setStatus(id: number, status: any) {
  return defHttp.get<any>(
    { url: Api.setStatus, params: { id, status } },
    { errorMessageMode: 'none' },
  );
}

export function deleteItem(params) {
  return defHttp.get<any>({ url: Api.Delete, params }, { errorMessageMode: 'message' });
}

export function setVerify(params) {
  return defHttp.post<any>({ url: Api.verify, params }, { errorMessageMode: 'message' });
}
