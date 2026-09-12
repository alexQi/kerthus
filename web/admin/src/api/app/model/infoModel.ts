/**
 * @description: Info interface parameters
 */

export interface RoutesParams {
  path?: string;
  server?: string;
}

export interface LocationParams {
  longitude: string;
  latitude: string;
}

export interface RoutesItem {
  method: string[];
  uri: string;
  action: string;
}

/**
 * @description: Login interface return value
 */
export interface RoutesModel {
  T: {
    T: RoutesItem;
  };
}

/**
 * @description: Get user information return value
 */
export interface LocationModel {
  location: string;
  province: string;
  city: string;
  district: string;
  township: string;
  detail: string;
  latitude: string;
  longitude: string;
}
