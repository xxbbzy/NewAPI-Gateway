import { toast } from 'react-toastify';
import { toastConstants } from '../constants';

export function isAdmin() {
  let user = localStorage.getItem('user');
  if (!user) return false;
  user = JSON.parse(user);
  return user.role >= 10;
}

export function isRoot() {
  let user = localStorage.getItem('user');
  if (!user) return false;
  user = JSON.parse(user);
  return user.role >= 100;
}

export function getRoleName(role) {
  switch (Number(role)) {
    case 1:
      return '普通用户';
    case 10:
      return '管理员';
    case 100:
      return '超级管理员';
    default:
      return '未知身份';
  }
}

export function getSystemName() {
  let system_name = localStorage.getItem('system_name');
  if (!system_name) return 'NewAPI Gateway';
  return system_name;
}

export function getFooterHTML() {
  return localStorage.getItem('footer_html');
}

export async function copy(text) {
  let okay = true;
  try {
    await navigator.clipboard.writeText(text);
  } catch (e) {
    okay = false;
    console.error(e);
  }
  return okay;
}

export function isMobile() {
  return window.innerWidth <= 600;
}

let showErrorOptions = { autoClose: toastConstants.ERROR_TIMEOUT };
let showWarningOptions = { autoClose: toastConstants.WARNING_TIMEOUT };
let showSuccessOptions = { autoClose: toastConstants.SUCCESS_TIMEOUT };
let showInfoOptions = { autoClose: toastConstants.INFO_TIMEOUT };
let showNoticeOptions = { autoClose: false };

if (isMobile()) {
  showErrorOptions.position = 'top-center';
  // showErrorOptions.transition = 'flip';

  showSuccessOptions.position = 'top-center';
  // showSuccessOptions.transition = 'flip';

  showInfoOptions.position = 'top-center';
  // showInfoOptions.transition = 'flip';

  showNoticeOptions.position = 'top-center';
  // showNoticeOptions.transition = 'flip';
}

export function showError(error) {
  console.error(error);
  if (error.message) {
    if (error.name === 'AxiosError') {
      switch (error.response.status) {
        case 401:
          // toast.error('错误：未登录或登录已过期，请重新登录！', showErrorOptions);
          window.location.href = '/login?expired=true';
          break;
        case 429:
          toast.error('错误：请求次数过多，请稍后再试！', showErrorOptions);
          break;
        case 500:
          toast.error('错误：服务器内部错误，请联系管理员！', showErrorOptions);
          break;
        case 405:
          toast.info('本站仅作演示之用，无服务端！');
          break;
        default:
          toast.error('错误：' + error.message, showErrorOptions);
      }
      return;
    }
    toast.error('错误：' + error.message, showErrorOptions);
  } else {
    toast.error('错误：' + error, showErrorOptions);
  }
}

export function showWarning(message) {
  toast.warn(message, showWarningOptions);
}

export function showSuccess(message) {
  toast.success(message, showSuccessOptions);
}

export function showInfo(message) {
  toast.info(message, showInfoOptions);
}

export function showNotice(message) {
  toast.info(message, showNoticeOptions);
}

export function openPage(url) {
  window.open(url);
}

export function removeTrailingSlash(url) {
  if (url.endsWith('/')) {
    return url.slice(0, -1);
  } else {
    return url;
  }
}

export function timestamp2string(timestamp) {
  let date = new Date(timestamp * 1000);
  let year = date.getFullYear().toString();
  let month = (date.getMonth() + 1).toString();
  let day = date.getDate().toString();
  let hour = date.getHours().toString();
  let minute = date.getMinutes().toString();
  let second = date.getSeconds().toString();
  if (month.length === 1) {
    month = '0' + month;
  }
  if (day.length === 1) {
    day = '0' + day;
  }
  if (hour.length === 1) {
    hour = '0' + hour;
  }
  if (minute.length === 1) {
    minute = '0' + minute;
  }
  if (second.length === 1) {
    second = '0' + second;
  }
  return (
    year + '-' + month + '-' + day + ' ' + hour + ':' + minute + ':' + second
  );
}

export function normalizePaginatedData(data, fallback = {}) {
  const fallbackPage = Number.isFinite(Number(fallback.p)) ? Number(fallback.p) : 0;
  const fallbackPageSize = Number.isFinite(Number(fallback.page_size)) && Number(fallback.page_size) > 0
    ? Number(fallback.page_size)
    : 10;
  const clampNonNegativeInt = (value, defaultValue) => {
    const n = Number.parseInt(value, 10);
    if (!Number.isFinite(n) || Number.isNaN(n) || n < 0) {
      return defaultValue;
    }
    return n;
  };

  const payload = (data && typeof data === 'object' && !Array.isArray(data)) ? data : {};
  const items = Array.isArray(payload.items) ? payload.items : [];
  const p = clampNonNegativeInt(payload.p ?? payload.page, fallbackPage);
  const pageSize = clampNonNegativeInt(payload.page_size, fallbackPageSize) || fallbackPageSize;
  const total = clampNonNegativeInt(payload.total, items.length);

  let totalPages = clampNonNegativeInt(payload.total_pages, -1);
  if (totalPages < 0) {
    totalPages = pageSize > 0 ? Math.ceil(total / pageSize) : 0;
  }

  const hasMore = typeof payload.has_more === 'boolean'
    ? payload.has_more
    : (totalPages > 0 && p + 1 < totalPages);

  return {
    ...payload,
    items,
    p,
    page: p,
    page_size: pageSize,
    total,
    total_pages: totalPages,
    has_more: hasMore
  };
}
