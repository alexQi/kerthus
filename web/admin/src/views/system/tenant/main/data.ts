import { h } from 'vue';
import dayjs from 'dayjs';
import { Switch } from 'ant-design-vue';
import { BasicColumn, FormSchema } from '/@/components/Table';
import { useMessage } from '/@/hooks/web/useMessage';
import { uploadApi } from '/@/api/app/upload';
import { setStatus } from '/@/api/tenant/tenant';
import { GetDistricts } from '/@/api/common/config';

export const columns: BasicColumn[] = [
  {
    title: 'ID',
    dataIndex: 'id',
    key: 'id',
    width: 100,
  },
  {
    title: '企业名称',
    dataIndex: 'name',
    key: 'name',
    align: 'left',
    width: 300,
  },
  {
    title: '注册类型',
    dataIndex: 'register_type', // create register
    key: 'register_type',
    width: 150,
  },
  {
    title: '状态',
    dataIndex: 'status',
    key: 'status',
    width: 150,
    customRender: ({ record }) => {
      if (!Reflect.has(record, 'pendingStatus')) {
        record.pendingStatus = false;
      }
      return h(Switch, {
        checked: record.status === 1,
        checkedChildren: '已启用',
        unCheckedChildren: '已禁用',
        loading: record.pendingStatus,
        onChange(checked: boolean) {
          record.pendingStatus = true;
          const newStatus = checked ? 1 : 0;
          const { createMessage } = useMessage();
          setStatus(record.id, newStatus)
            .then(() => {
              record.status = newStatus;
              createMessage.success(`修改租户状态成功`);
            })
            .catch((e) => {
              createMessage.error(e.message ? e.message : '修改租户状态失败');
            })
            .finally(() => {
              record.pendingStatus = false;
            });
        },
      });
    },
  },
  {
    title: '审核状态',
    dataIndex: 'verify_status',
    key: 'verify_status',
  },
  {
    title: '有效期',
    dataIndex: 'expiration_time',
    key: 'expiration_time',
  },
  {
    title: '申请时间',
    dataIndex: 'created_at',
    key: 'created_at',
  },
];

export const searchFormSchema: FormSchema[] = [
  {
    label: '企业名称',
    field: 'name',
    component: 'Input',
    colProps: { span: 6 },
  },
  {
    label: '创建时间',
    field: 'range_time',
    component: 'RangePicker',
    colProps: { span: 6 },
    componentProps: {
      valueFormat: 'YYYY-MM-DD',
    },
  },
];

export const formSchema: FormSchema[] = [
  {
    field: 'divider-basic',
    component: 'Divider',
    label: '基础信息',
    colProps: {
      span: 24,
    },
  },
  {
    field: 'id',
    label: 'Id',
    required: false,
    component: 'Input',
    show: false,
    colProps: {
      span: 24,
    },
  },
  // 上传
  {
    field: 'logo',
    component: 'DefaultUpload',
    label: '上传LOGO',
    colProps: {
      span: 12,
    },
    componentProps: {
      api: uploadApi,
      multiple: false,
      maxNumber: 1,
      accept: ['image/*'],
      uploadParams: {
        directory: 'tenant',
        param: 'logo',
      },
    },
  },
  {
    field: 'name',
    label: '企业名称',
    required: true,
    component: 'Input',
    colProps: {
      span: 24,
    },
  },
  {
    field: 'expiration_time',
    label: '有效期',
    required: false,
    component: 'DatePicker',
    componentProps: {
      style: 'width:100%',
      showTime: true,
      valueFormat: 'X',
      disabledDate: (current) => {
        return current < dayjs().add(-1, 'd');
      },
    },
    colProps: {
      span: 12,
    },
  },
  {
    field: 'divider-basic',
    component: 'Divider',
    label: '联系人信息',
    required: true,
    colProps: {
      span: 24,
    },
  },
  {
    field: 'contact_person',
    label: '联系人',
    required: true,
    component: 'Input',
    colProps: {
      span: 12,
    },
  },
  {
    field: 'contact_phone',
    label: '联系方式',
    required: true,
    component: 'Input',
    colProps: {
      span: 12,
    },
  },
  {
    field: 'contact_email',
    label: '联系邮箱',
    required: true,
    component: 'Input',
    colProps: {
      span: 12,
    },
  },
  {
    field: 'divider-basic',
    component: 'Divider',
    label: '地区信息',
    colProps: {
      span: 24,
    },
  },
  {
    field: 'address_code',
    component: 'ApiCascader',
    label: '地区',
    required: true,
    colProps: { span: 12 },
    componentProps: ({ formModel }) => {
      return {
        style: 'width:100%',
        api: GetDistricts,
        placeholder: '请选择地区',
        labelField: 'name',
        valueField: 'id',
        asyncFetchParamKey: 'parent_id',
        displayRenderArray: formModel.address ? formModel.address : [],
        isLeaf: (record) => {
          return record.has_children === false;
        },
        onDefaultChange: (_, selectedOptions) => {
          formModel.address = JSON.stringify(selectedOptions.map((o) => o.label));
          formModel.area_code = selectedOptions[selectedOptions.length - 1]?.id || 0;
        },
      };
    },
  },
  {
    field: 'area_code',
    component: 'Input',
    label: '区域代码',
    required: false,
    show: false,
  },
  {
    field: 'address',
    component: 'Select',
    label: '所在地区',
    required: false,
    show: false,
  },
  {
    field: 'address_detail',
    label: '详细地址',
    required: true,
    component: 'Input',
  },
  {
    field: 'divider-basic',
    component: 'Divider',
    label: '其他信息',
    colProps: {
      span: 24,
    },
  },
  {
    field: 'credit_code',
    label: '统一社会信用代码',
    required: false,
    component: 'Input',
    colProps: {
      span: 12,
    },
  },
  {
    field: 'desc',
    label: '企业简介',
    required: false,
    component: 'InputTextArea',
    componentProps: {
      resize: 'none',
    },
    colProps: {
      span: 24,
    },
  },
];
