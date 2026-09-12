import { defHttp } from '/@/utils/http/axios';
import {
  GetAuthInfoModel,
  GetUserInfoModel,
  LoginParams,
  LoginResultModel,
} from './model/userModel';

import { ErrorMessageMode } from '/#/axios';

enum Api {
  Login = '/system/user/login',
  Logout = '/system/user/logout',
  GetUserInfo = '/system/user/profile',
  GetUserAuth = '/system/user/auth',
}

/**
 * @description: user login api
 */
export function loginApi(params: LoginParams, mode: ErrorMessageMode = 'modal') {
  return defHttp.post<LoginResultModel>(
    { url: Api.Login, params },
    { errorMessageMode: mode, withToken: false },
  );
}

/**
 * @description: getUserInfo
 */
export function getUserInfo() {
  return defHttp.get<GetUserInfoModel>({ url: Api.GetUserInfo }, { errorMessageMode: 'none' });
}

/**
 * @description: getAuthInfo
 */
export function getAuthInfo() {
  return defHttp.get<GetAuthInfoModel>({ url: Api.GetUserAuth }, { errorMessageMode: 'none' });
}

export function doLogout(accessToken: string) {
  return defHttp.get(
    { url: Api.Logout, headers: { 'access-token': accessToken } },
    { withToken: false, errorMessageMode: 'none' },
  );
}
