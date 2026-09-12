<template>
  <Upload
    :accept="getStringAccept"
    :multiple="multiple"
    :file-list="fileList"
    :max-count="maxNumber"
    :custom-request="customRequest"
    :before-upload="beforeUpload"
    :on-remove="handleDelete"
    class="upload-comp"
    list-type="picture"
  >
    <template #itemRender="{ file, actions }">
      <div :style="'width:' + width + ';height:' + height" class="upload-item">
        <Image
          :src="getImageSrc(file.url)"
          :width="width"
          :height="height"
          :style="{ objectFit }"
        />
        <div class="upload-item-delete">
          <CloseSquareFilled @click="actions.remove" />
        </div>
      </div>
    </template>
    <div
      v-if="fileList.length < maxNumber"
      class="upload-btn"
      :style="'width:' + width + ';height:' + height"
    >
      <PlusOutlined />
      <div class="mt-1">上传</div>
    </div>
  </Upload>
</template>
<script lang="ts">
  import { defineComponent, onMounted, CSSProperties, PropType, ref, toRefs, watch } from 'vue';
  import { Image, Upload, UploadProps } from 'ant-design-vue';
  import { CloseSquareFilled, PlusOutlined } from '@ant-design/icons-vue';
  import { UploadResultStatus } from '/@/components/Upload/src/typing';
  import { useI18n } from '/@/hooks/web/useI18n';
  import { useMessage } from '/@/hooks/web/useMessage';
  import { useGlobSetting } from '/@/hooks/setting';
  import { isArray } from '/@/utils/is';
  import { getImageSrc } from '/@/utils/file/resource';
  import { basicProps } from './props';
  import { useUploadType } from './useUpload';

  export default defineComponent({
    components: { Upload, PlusOutlined, Image, CloseSquareFilled },
    props: {
      ...basicProps,
      value: {
        type: Array as PropType<any[]>,
        default: () => [],
      },
      width: {
        type: String as PropType<string>,
        default: '10rem',
      },
      height: {
        type: String as PropType<string>,
        default: '10rem',
      },
      objectFit: {
        type: String as PropType<CSSProperties['objectFit']>,
        default: 'cover',
      },
    },
    emits: ['change', 'register', 'delete', 'update:value'],
    setup(props, { emit }) {
      //   是否正在上传
      const fileList = ref<NonNullable<UploadProps['fileList']>>([]);
      const { accept, helpText, maxNumber, maxSize } = toRefs(props);

      const { t } = useI18n();
      const { uploadUrl = '' } = useGlobSetting();

      onMounted(() => {
        fileList.value = isArray(props.value)
          ? props.value.map((item, index) => {
              return {
                uid: index.toString(),
                name: item,
                url: item,
                status: 'done',
              };
            })
          : [];
      });

      watch(
        () => props.value,
        (value = []) => {
          fileList.value = isArray(value)
            ? value.map((item, index) => {
                return {
                  uid: index.toString(),
                  name: item,
                  status: 'done',
                  url: item,
                };
              })
            : [];
        },
      );

      const { getStringAccept } = useUploadType({
        acceptRef: accept,
        helpTextRef: helpText,
        maxNumberRef: maxNumber,
        maxSizeRef: maxSize,
      });

      const { createMessage } = useMessage();

      // 上传前校验
      function beforeUpload(file: File) {
        const { maxSize } = props;
        // 设置最大值，则判断
        if (maxSize && file.size / 1024 / 1024 >= maxSize) {
          createMessage.error(t('component.upload.maxSizeMultiple', [maxSize]));
          return false;
        }
        if (fileList.value) {
          for (const tempFile of fileList.value) {
            if (tempFile.name === file.name) {
              createMessage.error('当前文件已上传');
              return false;
            }
          }
        }
        return true;
      }

      const customRequest = async (e) => {
        try {
          const { data } = await props.api?.(
            {
              data: {
                ...(props.uploadParams || {}),
              },
              file: e.file,
              name: props.name,
            },
            function onUploadProgress(progressEvent: ProgressEvent) {
              e.onProgress({
                percent: ((progressEvent.loaded / progressEvent.total) * 100) | 0,
              });
            },
          );
          e.file.status = UploadResultStatus.DONE;
          e.file.url = data.data;
          if (fileList.value) {
            fileList.value.push({
              uid: e.file.uid,
              name: e.file.name,
              status: e.file.status,
              url: e.file.url,
            });
          }
          e.onSuccess(data.data, e);
          postMessage();
        } catch (err) {
          e.onError({ event: err });
        }
      };

      function handleDelete(file) {
        if (fileList.value) {
          fileList.value = fileList.value.filter((item) => {
            return item.url !== file.url;
          });
          postMessage();
        }
      }

      function postMessage() {
        if (fileList.value) {
          const tempFiles = fileList.value.map((item) => item.url);
          console.log(tempFiles);
          emit('update:value', tempFiles);
          emit('change', tempFiles);
        }
      }

      return {
        beforeUpload,
        handleDelete,
        customRequest,
        uploadUrl,
        getStringAccept,
        fileList,
        getImageSrc,
      };
    },
  });
</script>
<style lang="less">
  .upload-comp {
    display: flex;
    justify-content: flex-end;
    flex-flow: row-reverse;

    .ant-upload-list-picture {
      display: flex;

      .ant-upload-list-picture-container {
        margin-right: 10px;
        margin-bottom: 10px;

        .upload-item {
          position: relative;
          display: flex;
          justify-content: center;
          align-items: center;
          background: #f5f7fa;
          height: 100%;

          img {
            width: 100%;
            height: 100%;
          }

          .upload-item-delete {
            position: absolute;
            top: -5px;
            right: -5px;
            width: 20px;
            height: 20px;
            display: flex;
            justify-content: center;
            align-items: center;
            transition: opacity 1s;

            span {
              color: #ff7b7b;
              font-size: 20px;
              cursor: pointer;
            }
          }
        }
      }
    }

    .upload-btn {
      font-size: 14px;
      color: #d9d9d9;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      border-radius: 5px;
      overflow: hidden;
      border: 1px dashed #d9d9d9;
    }

    .upload-btn:hover {
      cursor: pointer;
      border-color: #1677ff;
    }
  }
</style>
