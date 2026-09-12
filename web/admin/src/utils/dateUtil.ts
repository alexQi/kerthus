/**
 * Independent time operation tool to facilitate subsequent switch to dayjs
 */
import dayjs from 'dayjs';

const DATE_TIME_FORMAT = 'YYYY-MM-DD HH:mm:ss';
const DATE_FORMAT = 'YYYY-MM-DD';

export function formatToDateTime(date?: dayjs.ConfigType, format = DATE_TIME_FORMAT): string {
  return dayjs(date).format(format);
}

export function formatToDate(date?: dayjs.ConfigType, format = DATE_FORMAT): string {
  return dayjs(date).format(format);
}

export function formatTimeRange(start, end) {
  const mo_start = dayjs.unix(start);
  const mo_end = dayjs.unix(end);

  let return_start = mo_start.format('YY/MM/DD HH:mm');
  let return_end = mo_end.format('YY/MM/DD HH:mm');

  const mo_now_y = dayjs().year();

  const mo_start_y = mo_start.year();
  const mo_start_m = mo_start.month() + 1;
  const mo_start_d = mo_start.date();

  const mo_end_y = mo_end.year();
  const mo_end_m = mo_end.month() + 1;
  const mo_end_d = mo_end.date();

  if (mo_now_y === mo_start_y) {
    return_start = mo_start.format('MM/DD HH:mm');
  }
  if (mo_start_y === mo_end_y) {
    return_end = mo_end.format('MM/DD HH:mm');
    if (mo_start_m === mo_end_m && mo_start_d === mo_end_d) {
      return_end = mo_end.format('HH:mm');
    }
  }

  return return_start + ' - ' + return_end;
}

export function getDaysBetweenDates(startDate, endDate?: number) {
  const start = dayjs(startDate);
  let end = dayjs();
  if (endDate) {
    end = dayjs(endDate);
  }
  return end.diff(start, 'day');
}

export function getAge(birthday) {
  if (birthday) {
    return dayjs().diff(birthday, 'year') + '岁';
  }
  return '-- 岁';
}

export function checkTimeAfter(start_time) {
  return dayjs(start_time).isAfter(dayjs());
}

export function getTimeToUnix(date) {
  return dayjs(date).unix();
}

export const dateUtil = dayjs;
