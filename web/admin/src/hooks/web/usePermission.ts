import { usePermissionStore } from '/@/store/modules/permission';
import { useMultipleTabStore } from '/@/store/modules/multipleTab';
import { resetRouter } from '/@/router';
import projectSetting from '/@/settings/projectSetting';
import { PermissionModeEnum } from '/@/enums/appEnum';
import { RoleEnum } from '/@/enums/roleEnum';
import { intersection } from 'lodash-es';
import { isArray } from '/@/utils/is';

// User permissions related operations
export function usePermission() {
  const permissionStore = usePermissionStore();
  const tabStore = useMultipleTabStore();

  /**
   * Reset and regain authority resource information
   * 重置和重新获得权限资源信息
   * @param goHome
   */
  async function resume(goHome?: boolean) {
    // Clearing tabs must not navigate while the new app authorization is loading.
    tabStore.resetState();
    resetRouter();
    permissionStore.resetState();
    await permissionStore.getUserAuthAction(goHome);
  }

  /**
   * Determine whether there is permission
   */
  function hasPermission(value?: RoleEnum | RoleEnum[] | string | string[], def = true): boolean {
    // Visible by default
    if (!value) {
      return def;
    }

    const permMode = projectSetting.permissionMode;

    if ([PermissionModeEnum.ROUTE_MAPPING, PermissionModeEnum.ROLE].includes(permMode)) {
      if (!isArray(value)) {
        return permissionStore.getRoles?.includes(value as RoleEnum);
      }
      return (intersection(value, permissionStore.getRoles) as RoleEnum[]).length > 0;
    }
    if (PermissionModeEnum.BACK === permMode) {
      if (permissionStore.getIsPlatformAdmin) {
        return true;
      }
      const allCodeList = permissionStore.getPermissions as string[];
      if (!isArray(value)) {
        return allCodeList.includes(value);
      }
      return (intersection(value, allCodeList) as string[]).length > 0;
    }
    return true;
  }

  /**
   * refresh menu data
   */
  async function refreshMenu(goHome?: boolean) {
    await resume(goHome);
  }

  return { hasPermission, refreshMenu };
}
