/**
 *
 * @param imageSrc
 * @param staticUrl
 * @returns {*}
 */
import { useAppStore } from '/@/store/modules/app';

export function getImageSrc(imageSrc) {
  if (!imageSrc) return '';
  const reg = /^(https?:)?\/\//i;
  const base64Reg = /^(data:)/i;
  if (reg.test(imageSrc)) {
    return imageSrc;
  } else if (base64Reg.test(imageSrc)) {
    return imageSrc;
  } else {
    const appStore = useAppStore();
    return (appStore.getAppConfig.static_url || '').replace(/\/$/, '') + '/' + imageSrc.replace(/^\//, '');
  }
}
