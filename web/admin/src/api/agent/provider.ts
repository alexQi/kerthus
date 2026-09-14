import { defHttp } from '/@/utils/http/axios';

enum Api {
  Models = '/api/agent/provider/models',
  Test = '/api/agent/provider/test',
}

export interface AgentProviderModelRequest {
  name?: string;
  code?: string;
  provider?: string;
  endpoint?: string;
  api_key?: string;
  model?: string;
  models?: unknown[];
}

export function syncProviderModels(params: AgentProviderModelRequest) {
  return defHttp.post<any>({ url: Api.Models, params }, { errorMessageMode: 'none' });
}

export function testProviderModel(params: AgentProviderModelRequest) {
  return defHttp.post<any>({ url: Api.Test, params }, { errorMessageMode: 'none' });
}
