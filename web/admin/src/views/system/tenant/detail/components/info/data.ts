import { DescItem } from '/@/components/Description';
import { formatToDateTime } from '/@/utils/dateUtil';
import { h } from 'vue';
import { Image } from 'ant-design-vue';
import { getImageSrc } from '/@/utils/file/resource';
import { GetDistricts } from '/@/api/common/config';

export const infoSchema: DescItem[] = [
  {
    field: 'logo',
    label: 'LOGO',
    render: (val) => {
      return h(Image, {
        src: getImageSrc(val) || '/resource/img/logo.png',
        fallback: '/resource/img/logo.png',
        height: '100px',
      });
    },
  },
  {
    field: 'name',
    label: '企业名称',
    render: displayText,
  },
  {
    field: 'expiration_time',
    label: '有效期',
    render: (val) => {
      if (val == null || val === '') return '—';
      return Number(val) > 0 ? formatToDateTime(Number(val) * 1000) : '永久有效';
    },
  },
  {
    field: 'contact_person',
    label: '联系人',
    render: displayText,
  },
  {
    field: 'contact_phone',
    label: '联系方式',
    render: displayText,
  },
  {
    field: 'contact_email',
    label: '联系邮箱',
    render: displayText,
  },
  {
    field: 'address_display',
    label: '所在地区',
    render: displayText,
  },
  {
    field: 'address_detail',
    label: '详细地址',
    render: displayText,
  },
  {
    field: 'credit_code',
    label: '统一社会信用代码',
    render: displayText,
  },
  {
    field: 'desc',
    label: '企业简介',
    render: displayText,
  },
];

function displayText(value: unknown): string {
  return value == null || String(value).trim() === '' ? '—' : String(value);
}

function addressParts(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String).filter(Boolean);
  if (typeof value === 'number') return value > 0 ? [String(value)] : [];
  if (typeof value !== 'string' || !value.trim()) return [];
  try {
    const parsed = JSON.parse(value);
    if (Array.isArray(parsed)) return addressParts(parsed);
  } catch {
    // Earlier records stored the region path as a delimited string.
  }
  return value.split(/[-,，/|;\s]+/).filter(Boolean);
}

export async function resolveAddress(data: Record<string, any>): Promise<string> {
  const labels = addressParts(data.address);
  const storedCodes = addressParts(data.address_code);
  const codes = storedCodes.length ? storedCodes : labels.filter((part) => /^\d+$/.test(part));
  if (!codes.length) return labels.join(' / ') || '—';

  const names: string[] = [];
  let parentId = 0;
  for (let index = 0; index < codes.length; index++) {
    const code = codes[index];
    const fallback = labels[index] || code;
    try {
      const districts = await GetDistricts({ parent_id: parentId });
      const district = districts.find((item) => String(item.id) === code);
      names.push(district?.name || fallback);
    } catch {
      names.push(fallback);
    }
    parentId = Number(code);
    if (!Number.isFinite(parentId)) {
      names.push(...codes.slice(index + 1));
      break;
    }
  }
  return names.join(' / ') || '—';
}
