import { defHttp } from '/@/utils/http/axios';
import { DistrictModel } from '/@/api/common/model/configModel';

enum Api {
  Districts = '/system/config/districts',
}

export const GetDistricts = (params: any) => {
  return defHttp.get<DistrictModel[]>({ url: Api.Districts, params });
};
