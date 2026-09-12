import { defHttp } from '/@/utils/http/axios';
import {
  RoutesModel,
  RoutesParams,
} from '/@/api/app/model/infoModel';

enum Api {
  Init = '/app/info/init',
  Servers = '/app/info/servers',
  Routes = '/app/info/routes',
}

/**
 * @description: Get user menu based on id
 */

export const AppInit = () => {
  return defHttp.get<any>({ url: Api.Init });
};

export const AppServers = () => {
  return defHttp.get<any>({ url: Api.Servers });
};

export const AppRoutes = (params: RoutesParams) => {
  return defHttp.get<RoutesModel>({ url: Api.Routes, params });
};
