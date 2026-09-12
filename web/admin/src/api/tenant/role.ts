import { defHttp } from '/@/utils/http/axios';

enum Api {
  IndexList = '/system/role/query',
  Save = '/system/role/save',
  Delete = '/system/role/delete',
  setStatus = '/system/role/setStatus',
  queryRoleResources = '/system/role/queryRoleResources',
  authRoleResource = '/system/role/authRoleResource',
  relateEmployee = '/system/role/relateEmployee',
}

export function getList(params) {
  return defHttp.get<any>({ url: Api.IndexList, params }, { errorMessageMode: 'none' });
}

export function saveData(params) {
  return defHttp.post<any>({ url: Api.Save, params }, { errorMessageMode: 'message' });
}

export function deleteData(params: any) {
  return defHttp.get<any>({ url: Api.Delete, params }, { errorMessageMode: 'message' });
}

export function setStatus(id: number, status: any, tenant_id?: number) {
  return defHttp.get<any>(
    { url: Api.setStatus, params: { id, status, tenant_id } },
    { errorMessageMode: 'none' },
  );
}

export function queryRoleResources(params: any) {
  return defHttp.get<any>({ url: Api.queryRoleResources, params }, { errorMessageMode: 'message' });
}

export function authRoleResource(params) {
  return defHttp.post<any>({ url: Api.authRoleResource, params }, { errorMessageMode: 'message' });
}

export function relateEmployee(params) {
  return defHttp.post<any>({ url: Api.relateEmployee, params }, { errorMessageMode: 'message' });
}
