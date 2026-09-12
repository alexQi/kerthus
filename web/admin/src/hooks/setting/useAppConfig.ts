import { useAppStore } from '/@/store/modules/app';

/**
 * Listening to page changes and dynamically changing site titles
 */
export async function useAppConfig() {
  const appStore = useAppStore();
  await appStore.initAppConfig();
}
