/**
 * @description: Login interface parameters
 */
import { Menu } from '/@/router/types';

export interface LoginParams {
  username: string;
  password: string;
  scene: string;
}

/**
 * @description: Login interface return value
 */
export interface LoginResultModel {
  user_id: number;
  access_token: string;
  expires_time: number;
  tenant_id: number;
  seat_id: number;
  app_id: number;
  unit_id: number;
  section_id: number;
  app_code: string;
}

/**
 * @description: Get user information return value
 */
export interface GetUserInfoModel {
  id: number;
  phone: string;
  email: string;
  name: string;
  avatar: string;
  sex: number;
}

/**
 * @description: Get user information return value
 */
export interface AuthRoute extends Menu {
  component?: string;
  type?: string;
  children?: AuthRoute[];
}

export interface GetAuthInfoModel {
  platform_admin: boolean;
  context?: { home?: string };
  menus: Menu[];
  routes: AuthRoute[];
  roles: string[];
  permissions: string[];
}
