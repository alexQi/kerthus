import type { ErrorMessageMode } from '/#/axios';
import { useUserStoreWithOut } from '/@/store/modules/user';
import { useI18n } from '/@/hooks/web/useI18n';
import { useMessage } from '/@/hooks/web/useMessage';

// Both HTTP 401 and the legacy { code: 401 } envelope use this path.
export function handleUnauthorized(
  config: any,
  msg: string,
  mode: ErrorMessageMode = 'message',
) {
  const { t } = useI18n();
  let content = msg || t('sys.api.errMsg401');
  if (config?.requestOptions?.withToken !== false) {
    const userStore = useUserStoreWithOut();
    const requestToken = config?.headers?.['access-token'];
    // Concurrent failures and late responses from an older session must not
    // clear a new login or display another timeout notification.
    if (!requestToken || requestToken !== userStore.getTokenInfo.access_token) return;
    content = t('sys.api.timeoutMessage');
    // Local state is cleared synchronously; an expired token needs no logout API call.
    void userStore.logout(false);
  }

  const { createMessage, createErrorModal } = useMessage();
  if (mode === 'modal') {
    createErrorModal({ title: t('sys.api.errorTip'), content });
  } else if (mode === 'message') {
    createMessage.error({ content, key: 'global_error_message_status_401' });
  }
}
