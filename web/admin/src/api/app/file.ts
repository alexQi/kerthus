import type { AxiosResponse } from 'axios';
import type { UploadFileParams } from '/#/axios';
import { defHttp } from '/@/utils/http/axios';
import { downloadByData } from '/@/utils/file/download';
import { uploadApi } from './upload';

/** Private attachments return a storage key, never a public URL. */
export function uploadAttachmentApi(
  params: UploadFileParams,
  onUploadProgress: (event: ProgressEvent) => void = () => {},
) {
  return uploadApi(
    { ...params, data: { ...params.data, purpose: 'attachment' } },
    onUploadProgress,
  );
}

export interface FileLimits {
  image_max_bytes: number;
  attachment_max_bytes: number;
}

export function getFileLimits() {
  return defHttp.get<FileLimits>({ url: '/app/file/limits' });
}

async function attachmentResponse(key: string): Promise<AxiosResponse<Blob>> {
  if (!key || !key.trim()) throw new Error('缺少附件标识');
  // The standard interceptor supplies access-token, tenant-id and app-id headers.
  const response = await defHttp.get<AxiosResponse<Blob>>(
    { url: '/app/file/download', params: { key }, responseType: 'blob' },
    { isReturnNativeResponse: true, joinTime: false, errorMessageMode: 'message' },
  );
  // Preserve a JSON error envelope if a gateway returns one instead of attachment bytes.
  if (!response.headers['content-disposition'] && response.data.type.includes('json')) {
    const body = JSON.parse(await response.data.text());
    throw new Error(body.msg || '附件下载失败');
  }
  return response;
}

export async function getAttachmentBlob(key: string): Promise<Blob> {
  return (await attachmentResponse(key)).data;
}

export async function downloadAttachment(key: string, filename?: string): Promise<void> {
  const response = await attachmentResponse(key);
  const disposition = String(response.headers['content-disposition'] || '');
  let serverName =
    disposition.match(/filename="([^"]+)"/i)?.[1] ||
    disposition.match(/filename=([^;]+)/i)?.[1]?.trim() ||
    '';
  const encoded = disposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1];
  if (encoded) {
    try {
      serverName = decodeURIComponent(encoded);
    } catch {
      /* Use the plain filename. */
    }
  }
  const safeName = (filename || serverName || key.split('/').pop() || 'attachment').replace(
    /[\\/\u0000-\u001f\u007f]/g,
    '_',
  );
  downloadByData(response.data, safeName, response.data.type);
}
