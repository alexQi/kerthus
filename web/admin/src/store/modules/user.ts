import type { TokenInfo, UserInfo } from '/#/store';
import type { ErrorMessageMode } from '/#/axios';
import { defineStore } from 'pinia';
import { store } from '/@/store';
import { PageEnum } from '/@/enums/pageEnum';
import { SAAS_CONF_KEY, TOKEN_KEY, USER_INFO_KEY } from '/@/enums/cacheEnum';
import { getAuthCache, setAuthCache } from '/@/utils/auth';
import { LoginParams } from '/@/api/common/model/userModel';
import { doLogout, getUserInfo, loginApi } from '/@/api/common/user';
import { useI18n } from '/@/hooks/web/useI18n';
import { useMessage } from '/@/hooks/web/useMessage';
import { router, resetRouter } from '/@/router';
import { useMultipleTabStore } from '/@/store/modules/multipleTab';
import { usePermissionStore } from '/@/store/modules/permission';
import { h } from 'vue';

interface UserState {
  userInfo: Nullable<UserInfo>;
  tokenInfo: Nullable<TokenInfo>;
  sessionTimeout?: boolean;
  lastUpdateTime: number;
  saasConf?: Nullable<any>;
}

export const useUserStore = defineStore({
  id: 'app-user',
  state: (): UserState => ({
    // user info
    userInfo: null,
    // token
    tokenInfo: null,
    // Whether the login expired
    sessionTimeout: true,
    // Last fetch time
    lastUpdateTime: 0,
    // saasConf
    saasConf: null,
  }),
  getters: {
    getUserInfo(state): UserInfo {
      return state.userInfo || getAuthCache<UserInfo>(USER_INFO_KEY) || {};
    },
    getTokenInfo(state): TokenInfo {
      return state.tokenInfo || getAuthCache<TokenInfo>(TOKEN_KEY) || {};
    },
    getSessionTimeout(state): boolean {
      return !!state.sessionTimeout;
    },
    getLastUpdateTime(state): number {
      return state.lastUpdateTime;
    },
    getSaasConf(state): any {
      return state.saasConf || getAuthCache<any>(SAAS_CONF_KEY) || {};
    },
  },
  actions: {
    setTokenInfo(info: TokenInfo | null) {
      this.tokenInfo = info; // for null or undefined value
      setAuthCache(TOKEN_KEY, info);
    },
    setUserInfo(info: UserInfo | null) {
      this.userInfo = info;
      this.lastUpdateTime = new Date().getTime();
      setAuthCache(USER_INFO_KEY, info);
    },
    setSessionTimeout(flag: boolean) {
      this.sessionTimeout = flag;
    },
    setSaasConf(saasConf: any) {
      this.saasConf = saasConf;
      setAuthCache(SAAS_CONF_KEY, saasConf);
    },
    resetState() {
      this.userInfo = null;
      this.tokenInfo = null;
      this.sessionTimeout = false;
    },
    /**
     * @description: login
     */
    async login(
      params: LoginParams & {
        goHome?: boolean;
        mode?: ErrorMessageMode;
      },
    ): Promise<any> {
      try {
        const { goHome = true, mode, ...loginParams } = params;
        const tokenInfo = await loginApi(loginParams, mode);
        this.setTokenInfo(tokenInfo);
        this.setSaasConf({
          tenantId: tokenInfo.tenant_id,
          unitId: tokenInfo.unit_id,
          sectionId: tokenInfo.section_id,
          appId: tokenInfo.app_id,
          appCode: tokenInfo.app_code,
          home: '',
        });
        await this.afterLoginAction(goHome);
      } catch (error) {
        return Promise.reject(error);
      }
      return Promise.resolve();
    },
    async afterLoginAction(goHome?: boolean) {
      await this.getUserInfoAction();
      await this.getAuthInfoAction(goHome);
    },
    async getAuthInfoAction(goHome?: boolean) {
      const sessionTimeout = this.sessionTimeout;
      if (sessionTimeout) {
        this.setSessionTimeout(false);
      }
      await usePermissionStore().getUserAuthAction(goHome);
    },
    async getUserInfoAction() {
      const userInfo = await getUserInfo();
      this.setUserInfo(userInfo);

      const { notification } = useMessage();
      const { t } = useI18n();
      notification.success({
        message: t('sys.login.loginSuccessTitle'),
        description: `${t('sys.login.loginSuccessDesc')}: ${userInfo.name}`,
        duration: 3,
      });
    },

    /**
     *
     * @param clearRemote
     * @param goLogin
     */
    async logout(clearRemote = true, _goLogin = false) {
      const accessToken = this.getTokenInfo.access_token;
      this.setSessionTimeout(true);
      this.setTokenInfo(null);
      this.setUserInfo(null);
      this.lastUpdateTime = 0;
      this.setSaasConf(null);
      usePermissionStore().resetState();
      useMultipleTabStore().resetState();
      resetRouter();

      // Clear locally before any network wait. A delayed logout response must
      // not erase a subsequent login, and repeated logout calls have no token.
      const navigation = router.replace(PageEnum.BASE_LOGIN);
      const remote = clearRemote && accessToken
        ? doLogout(accessToken).catch(() => undefined)
        : Promise.resolve();
      await Promise.all([navigation, remote]);
    },

    /**
     * @description: Confirm before logging out
     */
    confirmLoginOut() {
      const { createConfirm } = useMessage();
      const { t } = useI18n();
      createConfirm({
        iconType: 'warning',
        title: () => h('span', t('sys.app.logoutTip')),
        content: () => h('span', t('sys.app.logoutMessage')),
        onOk: async () => {
          await this.logout(true);
        },
      });
    },
    getHomePage() {
      return this.getSaasConf.home || (this.getTokenInfo.access_token ? '/profile/setting' : PageEnum.BASE_LOGIN);
    },
  },
});

// Need to be used outside the setup
export function useUserStoreWithOut() {
  return useUserStore(store);
}
