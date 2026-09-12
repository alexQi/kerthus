<template>
  <div class="p-4 m-4 bg-white">
    <Description @register="register" :column="1" />
  </div>
</template>
<script lang="ts">
  import { defineComponent, onMounted } from 'vue';
  import { Description, useDescription } from '/@/components/Description';
  import { infoSchema, resolveAddress } from './data';
  import { getInfo } from '/@/api/tenant/tenant';

  export default defineComponent({
    name: 'ExtendInfo',
    components: {
      Description,
    },
    props: {
      fetchParams: {
        type: Object as PropType<any>,
        default: () => {
          return {};
        },
      },
    },
    setup(props) {
      const [register, { setDescProps }] = useDescription({
        schema: infoSchema,
        bordered: true,
        labelStyle: {
          width: '150px',
          textAlign: 'right',
        },
      });

      const fetchData = async () => {
        const data = await getInfo({ id: props.fetchParams.id });
        if (data) {
          setDescProps({ data: { ...data, address_display: await resolveAddress(data) } });
        }
      };

      onMounted(() => {
        fetchData();
      });

      return {
        register,
      };
    },
  });
</script>
