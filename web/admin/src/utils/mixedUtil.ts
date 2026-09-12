/**
 * Independent time operation tool to facilitate subsequent switch to dayjs
 */

type Option = {
  label: string;
  value: string | number;
  color?: string;
  linked?: string;
};

/**
 *
 * @param options
 * @param val
 * @param field
 */
export function getSelectOptionField(
  options: Option[],
  val: string | number,
  field = 'label',
): string {
  let fieldVal: any = '-';
  for (const key in options) {
    if (val === options[key].value) {
      fieldVal = options[key].hasOwnProperty(field) ? options[key][field] : '';
      break;
    }
  }
  return fieldVal;
}

/**
 *
 * @param options
 * @param val
 * @param field
 */
export function getColumnsField(options: any[], val: string | number, field = 'title'): string {
  let fieldVal: any = '';
  for (const key in options) {
    if (val === options[key].key) {
      fieldVal = options[key].hasOwnProperty(field) ? options[key][field] : '';
      break;
    }
  }
  return fieldVal;
}
