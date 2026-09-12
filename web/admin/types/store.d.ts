import { ErrorTypeEnum } from '/@/enums/exceptionEnum';
import { MenuModeEnum, MenuTypeEnum } from '/@/enums/menuEnum';
import { Menu } from '/@/router/types';

// Lock screen information
export interface LockInfo {
  // Password required
  pwd?: string | undefined;
  // Is it locked?
  isLock?: boolean;
}

// Error-log information
export interface ErrorLogInfo {
  // Type of error
  type: ErrorTypeEnum;
  // Error file
  file: string;
  // Error name
  name?: string;
  // Error message
  message: string;
  // Error stack
  stack?: string;
  // Error detail
  detail: string;
  // Error url
  url: string;
  // Error time
  time?: string;
}

export interface UserInfo {
  id: number;
  phone: string;
  email: string;
  name: string;
  avatar: string;
  sex: number;
}

export interface TokenInfo {
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

export interface AuthInfo {
  roles: string[];
  permissions: string[];
  menu: Menu;
}

export interface BeforeMiniState {
  menuCollapsed?: boolean;
  menuSplit?: boolean;
  menuMode?: MenuModeEnum;
  menuType?: MenuTypeEnum;
}
