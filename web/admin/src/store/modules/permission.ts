import type { AppRouteRecordRaw, Menu } from '/@/router/types';
import { defineStore } from 'pinia';
import { store } from '/@/store';
import { ERROR_LOG_ROUTE, USER_SETTING_ROUTE } from '/@/router/routes/basic';
import { getAuthInfo } from '/@/api/common/user';
import type { AuthRoute } from '/@/api/common/model/userModel';
import { getAuthCache, setAuthCache } from '/@/utils/auth';
import { MENU_KEY, PERMISSIONS_KEY, ROLES_KEY, ROUTE_KEY } from '/@/enums/cacheEnum';
import { router } from '/@/router';
import { RouteRecordRaw } from 'vue-router';
import { flatMultiLevelRoutes, transformObjToRoute } from '/@/router/helper/routeHelper';
import { externalUrl } from '/@/utils/externalUrl';
import { useUserStore } from '/@/store/modules/user';

// Only concrete, visible views returned by authorization are valid landing pages.
function authorizedHome(routes: AuthRoute[], preferred?: string): string {
  const views: string[] = [];
  function visit(items: AuthRoute[], parentPath = '', parentHidden = false) {
    for (const route of items) {
      const path = route.path.startsWith('/')
        ? route.path
        : `${parentPath}/${route.path}`.replace(/\/+/g, '/');
      const hidden = parentHidden || !!route.hideMenu || !!route.meta?.hideMenu;
      const component = route.component?.toUpperCase();
      if (
        !hidden &&
        !route.disabled &&
        route.open_with !== 'outside' &&
        component &&
        (route.open_with !== 'inside' || !!externalUrl(route.component)) &&
        !['LAYOUT', 'PARENTLAYOUT', 'IFRAME'].includes(component) &&
        !['action', 'field'].includes(route.type || '') &&
        path.startsWith('/') &&
        !path.startsWith('//') &&
        !/[:*?#]/.test(path)
      )
        views.push(path);
      if (route.children) visit(route.children, path, hidden);
    }
  }
  visit(routes);
  return preferred && views.includes(preferred) ? preferred : views[0] || '/profile/setting';
}

interface PermissionState {
  platformAdmin: boolean;
  permissions: string[];
  roles: string[];
  menus: Menu[];
  routes: Menu[];
  isDynamicAddedRoute: boolean;
  lastBuildMenuTime: number;
}

export const usePermissionStore = defineStore({
  id: 'app-permission',
  state: (): PermissionState => ({
    platformAdmin: false,
    roles: [],
    permissions: [],
    menus: [],
    routes: [],
    isDynamicAddedRoute: false,
    lastBuildMenuTime: 0,
  }),
  getters: {
    getIsPlatformAdmin(state): boolean {
      return state.platformAdmin;
    },
    getPermissions(state): string[] {
      return state.permissions || getAuthCache<string[]>(PERMISSIONS_KEY) || [];
    },
    getRoles(state): string[] {
      return state.roles || getAuthCache<string[]>(ROLES_KEY) || [];
    },
    getMenus(state): Menu[] {
      return state.menus || getAuthCache<Menu[]>(MENU_KEY) || [];
    },
    getRoutes(state): Menu[] {
      return state.routes || getAuthCache<Menu[]>(ROUTE_KEY) || [];
    },
    getLastBuildMenuTime(state): number {
      return state.lastBuildMenuTime;
    },
    getIsDynamicAddedRoute(state): boolean {
      return state.isDynamicAddedRoute;
    },
  },
  actions: {
    setRoles(roles: string[]) {
      this.roles = roles; // for null or undefined value
      setAuthCache(ROLES_KEY, roles);
    },
    setPermissions(permissions: string[]) {
      this.permissions = permissions; // for null or undefined value
      setAuthCache(PERMISSIONS_KEY, permissions);
    },
    setMenus(menu: Menu[]) {
      this.menus = menu; // for null or undefined value
      setAuthCache(MENU_KEY, menu);
    },
    setRoutes(routes: Menu[]) {
      this.routes = routes; // for null or undefined value
      setAuthCache(ROUTE_KEY, routes);
    },
    setLastBuildMenuTime() {
      this.lastBuildMenuTime = new Date().getTime();
    },
    setDynamicAddedRoute(added: boolean) {
      this.isDynamicAddedRoute = added;
    },
    resetState(): void {
      this.platformAdmin = false;
      this.isDynamicAddedRoute = false;
      this.setRoles([]);
      this.setPermissions([]);
      this.setMenus([]);
      this.setRoutes([]);
      this.lastBuildMenuTime = 0;
    },

    async getUserAuthAction(goHome?: boolean) {
      const authInfo = await getAuthInfo();
      const { roles, permissions, menus, routes, context } = authInfo;
      this.platformAdmin = authInfo.platform_admin === true;
      const userStore = useUserStore();
      userStore.setSaasConf({
        ...userStore.getSaasConf,
        home: authorizedHome(routes, context?.home),
      });
      this.setRoles(roles);
      this.setPermissions(permissions);
      this.setMenus(menus);
      this.setRoutes(routes);
      this.setLastBuildMenuTime();
      if (!this.isDynamicAddedRoute) {
        const routes = await this.buildRoutesAction();
        routes.forEach((route) => {
          router.addRoute(route as unknown as RouteRecordRaw);
        });
        router.addRoute(ERROR_LOG_ROUTE as RouteRecordRaw);

        this.setDynamicAddedRoute(true);
      }
      const sessionTimeout = userStore.getSessionTimeout;
      if (sessionTimeout) {
        userStore.setSessionTimeout(false);
      }
      if (goHome) {
        await router.replace(userStore.getHomePage());
      }
    },

    // 构建路由
    async buildRoutesAction(): Promise<AppRouteRecordRaw[]> {
      let userRoutes: AppRouteRecordRaw[] = [];
      /**
       * @description 根据设置的首页path，修正routes中的affix标记（固定首页）
       * */
      const patchHomeAffix = (routes: AppRouteRecordRaw[]) => {
        if (!routes || routes.length === 0) return;
        const userStore = useUserStore();
        let homePath: string = userStore.getHomePage();

        function patcher(routes: AppRouteRecordRaw[], parentPath = '') {
          if (parentPath) parentPath = parentPath + '/';
          routes.forEach((route: AppRouteRecordRaw) => {
            const { path, children, redirect } = route;
            const currentPath = path.startsWith('/') ? path : parentPath + path;
            if (currentPath === homePath) {
              if (redirect) {
                homePath = route.redirect! as string;
              } else {
                route.meta = Object.assign({}, route.meta, { affix: true });
                throw new Error('end');
              }
            }
            children && children.length > 0 && patcher(children, currentPath);
          });
        }

        try {
          patcher(routes);
        } catch (e) {
          // 已处理完毕跳出循环
        }
        return;
      };

      const routeList: AppRouteRecordRaw[] = transformObjToRoute(this.getRoutes);
      // 动态引入组件
      userRoutes = flatMultiLevelRoutes(routeList);

      userRoutes.push(USER_SETTING_ROUTE);

      patchHomeAffix(userRoutes);
      return userRoutes;
    },
  },
});

// Need to be used outside the setup
// 需要在设置之外使用
export function usePermissionStoreWithOut() {
  return usePermissionStore(store);
}
