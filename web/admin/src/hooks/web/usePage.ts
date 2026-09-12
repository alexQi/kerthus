import type { RouteLocationRaw, Router } from 'vue-router';
import { useRouter } from 'vue-router';

import { PageEnum } from '/@/enums/pageEnum';
import { isExternalDestination, openExternalUrl } from '/@/utils/externalUrl';
import { unref } from 'vue';
import { REDIRECT_NAME } from '/@/router/constant';
import { useUserStore } from '/@/store/modules/user';

export type PathAsPageEnum<T> = T extends { path: string } ? T & { path: PageEnum } : T;
export type RouteLocationRawEx = PathAsPageEnum<RouteLocationRaw>;

function handleError(e: Error) {
  console.error(e);
}

/**
 * page switch
 */
export function useGo(_router?: Router) {
  const { push, replace } = _router || useRouter();
  const userStore = useUserStore();
  function go(opt?: RouteLocationRawEx, isReplace = false) {
    const target = opt || userStore.getHomePage();
    if (!target) return;
    const path = typeof target === 'string' ? target : 'path' in target ? target.path : '';
    if (path && isExternalDestination(path)) {
      if (!openExternalUrl(path)) handleError(new Error('不支持的外部地址'));
      return;
    }
    isReplace ? replace(target).catch(handleError) : push(target).catch(handleError);
  }
  return go;
}

/**
 * @description: redo current page
 */
export const useRedo = (_router?: Router) => {
  const { replace, currentRoute } = _router || useRouter();
  const { query, params = {}, name, fullPath } = unref(currentRoute.value);
  function redo(): Promise<boolean> {
    return new Promise((resolve) => {
      if (name === REDIRECT_NAME) {
        resolve(false);
        return;
      }
      if (name && Object.keys(params).length > 0) {
        params['_redirect_type'] = 'name';
        params['path'] = String(name);
      } else {
        params['_redirect_type'] = 'path';
        params['path'] = fullPath;
      }
      replace({ name: REDIRECT_NAME, params, query }).then(() => resolve(true));
    });
  }
  return redo;
};
