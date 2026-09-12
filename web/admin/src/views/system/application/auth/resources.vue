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
            :checked="appState[index].checkAll"
            :indeterminate="appState[index].indeterminate"
            @change="onCheckAllChange"
            :disabled="
              !Number(tenantId) ||
              loading ||
              (isThirdPartyApp(item) && !!appState[index].entitlementId)
            "
            :value="item.id"
          />
          <span class="ml-1 mr-12">{{ item.name }}</span>
          <Switch
            style="margin-right: 1rem"
            checkedChildren="有限时间"
            unCheckedChildren="永不过期"
            :disabled="!Number(tenantId) || loading"
            :checked="appState[index].hasTTL"
            @change="(e) => onPermanentChange(e, item.id)"
          />
          <DatePicker
            class="ml-12"
            size="small"
            valueFormat="X"
            v-if="appState[index].hasTTL"
            :disabled="!Number(tenantId) || loading"
            :value="appState[index].ttl ? String(appState[index].ttl) : undefined"
            @change="(e) => onTtlChange(e, item.id)"
          />
        </div>
      </template>
      <div v-if="isThirdPartyApp(item)" class="px-4 pb-3 text-secondary"
        >第三方应用只需开通，不分配站内资源。已开通应用如需取消授权，请到租户应用授权列表操作。</div
      >
      <BasicTree
        v-else
        toolbar
        search
        checkable
        checkStrictly
        :allowCheckStrictlyChange="false"
        treeWrapperClassName="h-[calc(100%-35px)] overflow-auto mt-2"
        :disabled="!Number(tenantId) || loading"
        :clickRowToExpand="false"
        :loading="loading"
        :treeData="item['resources']"
        :value="appState[index].checkedList"
        @change="(val) => handleCheck(val, null, item.id)"
      >
        <template #title="record">
          <Tag :color="resourceTypeMap[record.type].color">
            {{ resourceTypeMap[record.type].text }}
          </Tag>
          {{ record.title }}
        </template>
      </BasicTree>
    </CollapseContainer>
  </div>
</template>
<script lang="ts">
  import { defineComponent, PropType, ref, watch } from 'vue';
  import { isThirdPartyApp } from '/@/utils/externalUrl';
  import dayjs from 'dayjs';
  import { Checkbox, DatePicker, Switch, Tag } from 'ant-design-vue';
  import { CollapseContainer } from '/@/components/Container';
  import { BasicTree } from '/@/components/Tree';
  import { getGlobalResource } from '/@/api/application/application';

  import { getList as getAuthorizations } from '/@/api/application/authorize';

  export default defineComponent({
    name: 'ResourceTree',
    components: {
      Tag,
      Checkbox,
      Switch,
      BasicTree,
      CollapseContainer,
      DatePicker,
    },
    props: {
      tenantId: {
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
      const appState = ref<any>({});
      const appResources = ref<any>({});

      let requestVersion = 0;

      function updateSelection(appId, keys) {
        const state = appState.value[appId];
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
        emit('select', appState.value);
      }

      function handleCheck(keys, _event, appId) {
        updateSelection(appId, Array.isArray(keys) ? keys : keys.checked);
      }

      const onCheckAllChange = (event: any) => {
        const appId = event.target.value;
        if (isThirdPartyApp(appResources.value[appId])) {
          const state = appState.value[appId];
          if (!state || loading.value || state.entitlementId) return;
          state.checkAll = !!event.target.checked;
          state.selectedApp = !!event.target.checked;
          emit('select', appState.value);
          return;
        }
        updateSelection(appId, event.target.checked ? appResources.value[appId].ids : []);
      };

      const onPermanentChange = (enabled: any, appId) => {
        appState.value[appId].hasTTL = !!enabled;
        emit('select', appState.value);
      };

      const onTtlChange = (value: any, appId) => {
        appState.value[appId].ttl = value ? Number(value) : 0;
        emit('select', appState.value);
      };

      function activeResources(nodes) {
        return nodes.flatMap((node) => {
          const children = activeResources(node.children || []);
          return node.status === 1 ? [{ ...node, children }] : children;
        });
      }

      function resourceIds(nodes): number[] {
        return nodes.flatMap((node) => [node.id, ...resourceIds(node.children || [])]);
      }

      async function loadAuthorizations(tenantId, version) {
        const items: Recordable[] = [];
        if (!tenantId) return { items };
        const seen = new Set<number>();
        for (let page = 1; ; page++) {
          const result = await getAuthorizations({ tenant_id: tenantId, page, pageSize: 200 });
          if (version !== requestVersion) return { items: [] };
          const batch: Recordable[] = result?.items || [];
          if (!batch.length) break;
          for (const item of batch) {
            if (seen.has(Number(item.id))) throw new Error('应用授权列表分页异常，请刷新重试');
            seen.add(Number(item.id));
            items.push(item);
          }
          if (items.length >= Number(result.total || 0)) break;
        }
        return { items };
      }

      async function loadResources() {
        const version = ++requestVersion;
        loading.value = true;
        emit('loading', true);
        emit('select', {});
        appResources.value = {};
        appState.value = {};
        try {
          const [resources, authorizations] = await Promise.all([
            getGlobalResource(),
            loadAuthorizations(props.tenantId, version),
          ]);
          if (version !== requestVersion) return;
          const states = {};
          const available = {};
          for (const appId in resources) {
            const app = resources[appId];
            if (app.code === 'system' || app.status !== 1) continue;
            app.resources = activeResources(app.resources || []);
            app.ids = resourceIds(app.resources);
            const thirdParty = isThirdPartyApp(app);
            if (!app.ids.length && !thirdParty) continue;
            const grant = authorizations?.items?.find(
              (item) => Number(item.app_id) === Number(appId),
            );
            const checkedList = (grant?.resource_ids || []).filter((id) => app.ids.includes(id));
            available[appId] = app;
            states[appId] = {
              checkedList,
              thirdParty,
              selectedApp: !!grant,
              entitlementId: grant?.id,
              hasTTL: Number(grant?.expiration_time) > 0,
              ttl: Number(grant?.expiration_time) || dayjs().add(1, 'year').unix(),
              indeterminate: !!checkedList.length && checkedList.length < app.ids.length,
              checkAll: thirdParty
                ? !!grant
                : !!app.ids.length && checkedList.length === app.ids.length,
            };
          }
          appState.value = states;
          appResources.value = available;
          emit('select', states);
          emit('loading', false);
        } finally {
          if (version === requestVersion) loading.value = false;
        }
      }

      watch(() => props.tenantId, loadResources, { immediate: true });

      return {
        appState,
        isThirdPartyApp,
        reload: loadResources,
        resourceTypeMap,
        loading,
        appResources,
        onCheckAllChange,
        onPermanentChange,
        onTtlChange,
        handleCheck,
        dayjs,
      };
    },
  });
</script>
<style lang="less" scoped>
  .app-resources {
    border: 1px solid rgb(217, 217, 217);
  }
</style>
