import { getSaasConf, getToken } from '/@/utils/auth';

/**
 * Headers shared by the Axios interceptor and the few browser streaming
 * requests that cannot use Axios (for example Agent SSE).
 */
export function getAuthHeaders(): Record<string, string> {
  const token: any = getToken() || {};
  const saasConf: any = getSaasConf() || {};
  const headers: Record<string, string> = {};
  const set = (name: string, value: unknown) => {
    if (value !== undefined && value !== null && value !== '') headers[name] = String(value);
  };
  set('access-token', token.access_token);
  set('tenant-id', saasConf.tenantId || saasConf.tenant_id || token.tenant_id);
  set('unit-id', saasConf.unitId || saasConf.unit_id || token.unit_id);
  set('section-id', saasConf.sectionId || saasConf.section_id || token.section_id);
  set('app-id', saasConf.appId || saasConf.app_id || token.app_id);
  return headers;
}
