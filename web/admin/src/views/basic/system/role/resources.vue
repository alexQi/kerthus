<template>
  <div class="overflow-hidden">
    <CollapseContainer
      v-for="(item, index) in appResources"
      :key="index"
      class="app-resources mb-4"
    >
      <template #title>
        <div class="flex" style="align-items: center">
          <Checkbox
            :checked="roleState[index].checkAll"
            :indeterminate="roleState[index].indeterminate"
            @change="onCheckAllChange"
            :disabled="!Number(roleId) || loading"
            :value="item.id"
          />
          <span class="ml-1 mr-12">{{ item.name }}</span>
        </div>
      </template>
      <BasicTree
        toolbar
        search
        checkable
        checkStrictly
        :allowCheckStrictlyChange="false"
        treeWrapperClassName="h-[calc(100%-35px)] overflow-auto mt-2"
        :disabled="!Number(roleId) || loading"
        :clickRowToExpand="false"
        :loading="loading"
        :treeData="item['resources']"
        :value="roleState[index].checkedList"
        :beforeRightClick="getRightMenus"
        @change="(val) => handleCheck(val, null, item.id)"
      >
        <template #title="record">
          <Tag :color="resourceTypeMap[record.type].color">
            {{ resourceTypeMap[record.type].text }}
          </Tag>
          {{ record.title }}
          <span
            class="ml-4"
            :style="
              'color:' +
              (roleState[index].scope.hasOwnProperty(record.id)
                ? scopeOptions[roleState[index].scope[record.id].toString()].color
                : scopeOptions[5].color)
            "
          >
            [{{
              roleState[index].scope.hasOwnProperty(record.id)
                ? scopeOptions[roleState[index].scope[record.id]].label
                : scopeOptions[5].label
            }}]
          </span>
        </template>
      </BasicTree>
    </CollapseContainer>
  </div>
</template>
<script lang="ts">
  import { defineComponent, PropType, ref, watch } from 'vue';
  import { Checkbox, Tag } from 'ant-design-vue';
  import { CollapseContainer } from '/@/components/Container';
  import { BasicTree, ContextMenuItem } from '/@/components/Tree';
  import { getTenantResources } from '/@/api/application/application';
  import { queryRoleResources } from '/@/api/tenant/role';
  import { scopeOptions } from '/@/constants/employ';
  import { useUserStore } from '/@/store/modules/user';
  import { getChildrenIds } from '/@/utils/helper/treeHelper';

  export default defineComponent({
    name: 'RoleResource',
    components: {
      Tag,
      Checkbox,
      BasicTree,
      CollapseContainer,
    },
    props: {
      tenantId: {
        type: [Number, String] as PropType<number | string>,
        default: 0,
      },
      roleId: {
        type: [Number, String] as PropType<number | string>,
        default: 0,
      },
    },
    emits: ['select', 'loading'],
    setup(props, { emit }) {
      const resourceTypeMap = {
        menu: {
          color: 'processing',
          text: '菜单',
        },
        view: {
          color: 'warning',
          text: '视图',
        },
        action: {
          color: 'error',
          text: '操作',
        },
        field: {
          color: 'success',
          text: '字段',
        },
      };
      const loading = ref<boolean>(false);
      const roleState = ref<any>({});
      const appResources = ref<any>({});

      function getRightMenus(node: any): ContextMenuItem[] {
        const rightMenus: any[] = [];
        for (const optionKey in scopeOptions) {
          rightMenus.push({
            label: scopeOptions[optionKey].label,
            handler: () => {
              roleState.value[node.app_id].scope[node.id] = scopeOptions[optionKey].value;
              const childIds = getChildrenIds(node.dataRef.children);
              for (const childId of childIds) {
                if (roleState.value[node.app_id].scope.hasOwnProperty(childId)) {
                  roleState.value[node.app_id].scope[childId] = scopeOptions[optionKey].value;
                }
              }
              emit('select', roleState.value);
            },
          });
        }
        return rightMenus;
      }

      const userStore = useUserStore();
      let requestVersion = 0;

      function updateSelection(appId, keys) {
        const state = roleState.value[appId];
        if (!state || loading.value) return;
        const selected = [...new Set(keys)];
        if (
          selected.length === state.checkedList.length &&
          selected.every((id) => state.checkedList.includes(id))
        )
          return;
        state.checkedList = selected;
        state.indeterminate =
          !!state.checkedList.length &&
          state.checkedList.length < appResources.value[appId].ids.length;
        state.checkAll = state.checkedList.length === appResources.value[appId].ids.length;
        for (const id of state.checkedList) {
          // New grants default to personal data; existing scopes survive selection changes.
          if (!Object.prototype.hasOwnProperty.call(state.scope, id)) state.scope[id] = 5;
        }
        emit('select', roleState.value);
      }

      function handleCheck(keys, _event, appId) {
        updateSelection(appId, Array.isArray(keys) ? keys : keys.checked);
      }

      const onCheckAllChange = (event: any) => {
        const appId = event.target.value;
        updateSelection(appId, event.target.checked ? appResources.value[appId].ids : []);
      };

      async function loadResources() {
        const version = ++requestVersion;
        loading.value = true;
        emit('loading', true);
        emit('select', {});
        appResources.value = {};
        roleState.value = {};
        try {
          const params = { tenant_id: props.tenantId || userStore.getSaasConf.tenant_id };
          const [resources, grants] = await Promise.all([
            getTenantResources(params),
            props.roleId ? queryRoleResources({ ...params, role_id: props.roleId }) : null,
          ]);
          if (version !== requestVersion) return;
          const states = {};
          for (const appId in resources) {
            const ids = resources[appId].ids;
            const checkedList = (grants?.resource_ids?.[appId] || []).filter((id) =>
              ids.includes(id),
            );
            states[appId] = {
              checkedList,
              scope: { ...grants?.resource_map?.[appId] },
              indeterminate: !!checkedList.length && checkedList.length < ids.length,
              checkAll: !!ids.length && checkedList.length === ids.length,
            };
          }
          roleState.value = states;
          appResources.value = resources;
          emit('select', states);
          emit('loading', false);
        } finally {
          if (version === requestVersion) loading.value = false;
        }
      }

      watch(() => [props.roleId, props.tenantId, userStore.getSaasConf.tenant_id], loadResources, {
        immediate: true,
      });

      return {
        scopeOptions,
        roleState,
        resourceTypeMap,
        loading,
        appResources,
        getRightMenus,
        onCheckAllChange,
        handleCheck,
      };
    },
  });
</script>
<style lang="less" scoped>
  .app-resources {
    border: 1px solid rgb(217, 217, 217);
  }
</style>
