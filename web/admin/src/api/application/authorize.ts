// import { IndexParams } from './models/applicationModel';
import { defHttp } from '/@/utils/http/axios';

enum Api {
  IndexList = '/system/app/queryTeantAuthorizes',
  authorizeApp = '/system/app/authorizeApp',
  deauthorizeApp = '/system/app/deauthorizeApp',
}

export function getList(params: any) {
  return defHttp.get<any>({ url: Api.IndexList, params }, { errorMessageMode: 'none' });
}

export function authorizeApp(params) {
  return defHttp.post<any>({ url: Api.authorizeApp, params }, { errorMessageMode: 'message' });
}

export function deauthorizeApp(params) {
  return defHttp.post<any>({ url: Api.deauthorizeApp, params }, { errorMessageMode: 'message' });
}
