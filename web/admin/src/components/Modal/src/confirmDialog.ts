import type { ModalFuncProps } from 'ant-design-vue/lib/modal/Modal';
import { useI18n } from '/@/hooks/web/useI18n';
import { Button, Modal } from 'ant-design-vue';
import { defineComponent, getCurrentInstance, h, ref, render, shallowReactive } from 'vue';
import './confirmDialog.less';

export interface DialogHandle {
  destroy: () => void;
  update: (options: ModalFuncProps | ((current: ModalFuncProps) => ModalFuncProps)) => void;
}

const dialogs = new Set<DialogHandle>();
const contentNode = (value: any) => (typeof value === 'function' ? value() : value);

// Ant Design Vue 3's static dialogs mutate a functional component's plain props.
// Vue 3.4 only runs that component's update when its reactive effect is dirty.
// Render the public Modal with reactive state instead of relying on those internals.
export function createConfirmDialog(options: ModalFuncProps): DialogHandle {
  const { t } = useI18n();
  const state = shallowReactive<ModalFuncProps>({ okCancel: true, ...options });
  const visible = ref(true);
  const pending = ref(false);
  const container = document.createElement('div');
  document.body.appendChild(container);
  let destroyed = false;

  function cleanup() {
    if (destroyed) return;
    destroyed = true;
    dialogs.delete(handle);
    render(null, container);
    container.remove();
    state.afterClose?.();
  }

  function close() {
    if (!destroyed) visible.value = false;
  }

  async function run(action?: (...args: any[]) => any) {
    if (pending.value || destroyed) return;
    if (!action) {
      close();
      return;
    }
    try {
      const result = action.length ? action(close) : action();
      if (result && typeof result.then === 'function') {
        pending.value = true;
        await result;
        close();
      } else if (!action.length) {
        close();
      }
    } catch {
      // Rejected submissions leave the dialog open; callers report their API/form error.
    } finally {
      pending.value = false;
    }
  }

  const handle: DialogHandle = {
    destroy: cleanup,
    update(next) {
      if (destroyed) return;
      const update = typeof next === 'function' ? next({ ...state }) : next;
      Object.assign(state, update);
      if (update.visible !== undefined) visible.value = update.visible;
    },
  };
  const host = defineComponent({
    name: 'ControlledConfirmDialog',
    setup: () => () =>
      h(
        Modal,
        {
          ...state,
          class: ['kerthus-confirm-dialog', state.class],
          visible: visible.value,
          title: contentNode(state.title),
          okText: contentNode(state.okText) ?? t('common.okText'),
          cancelText: contentNode(state.cancelText) ?? t('common.cancelText'),
          width: state.width ?? 416,
          closable: state.closable ?? false,
          maskClosable: state.maskClosable ?? false,
          confirmLoading: pending.value,
          cancelButtonProps: {
            ...state.cancelButtonProps,
            disabled: pending.value || state.cancelButtonProps?.disabled,
          },
          onOk: () => run(state.onOk),
          onCancel: () => run(state.onCancel),
          afterClose: cleanup,
        } as any,
        {
          default: () =>
            h('div', { class: 'kerthus-confirm-dialog__body' }, [
              state.icon
                ? h('span', { class: 'kerthus-confirm-dialog__icon' }, [contentNode(state.icon)])
                : null,
              h('div', { class: 'kerthus-confirm-dialog__content' }, [contentNode(state.content)]),
            ]),
          ...(state.okCancel === false
            ? {
                footer: () =>
                  h(
                    Button,
                    {
                      ...state.okButtonProps,
                      type: state.okType || 'primary',
                      loading: pending.value,
                      onClick: () => run(state.onOk),
                    } as any,
                    () => contentNode(state.okText) ?? t('common.okText'),
                  ),
              }
            : {}),
        },
      ),
  });
  const vnode = h(host);
  vnode.appContext =
    options.appContext || options.parentContext || getCurrentInstance()?.appContext || null;
  dialogs.add(handle);
  render(vnode, container);
  return handle;
}

export function destroyAllDialogs() {
  for (const dialog of [...dialogs]) dialog.destroy();
}
