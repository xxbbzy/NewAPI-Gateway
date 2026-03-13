import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import ProvidersTable from './ProvidersTable';
import { API, showError } from '../helpers';

jest.mock('react-router-dom', () => ({
  ...jest.requireActual('react-router-dom'),
  useNavigate: () => jest.fn(),
}));

jest.mock('../helpers', () => ({
  API: {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn(),
  },
  normalizePaginatedData: jest.requireActual('../helpers/utils').normalizePaginatedData,
  showError: jest.fn(),
  showSuccess: jest.fn(),
  timestamp2string: (value) => `ts:${value}`,
}));

const flushPromises = async () => {
  await act(async () => {
    await Promise.resolve();
  });
};

const setInputValue = (input, value) => {
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
  setter.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
  input.dispatchEvent(new Event('change', { bubbles: true }));
};

global.IS_REACT_ACT_ENVIRONMENT = true;

describe('ProvidersTable', () => {
  let container;
  let root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);

    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [],
              p: 0,
              page_size: 10,
              total: 0,
              total_pages: 0,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/summary') {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              total_providers: 2,
              balance_total_usd: 12.5,
              balance_account_count: 2,
              balance_fresh_count: 1,
              balance_stale_count: 1,
              balance_never_updated_count: 0,
              unreachable_providers: 1,
              proxy_enabled_providers: 1,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: 1,
                status: 'success',
                success_count: 2,
                failure_count: 0,
                ended_at: 1730000000,
              },
            ],
          },
        });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: 1,
                provider_name: 'Provider-A',
                status: 'success',
                message: 'ok',
                quota_awarded: 500000,
                checked_at: 1730000000,
              },
            ],
          },
        });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({
          data: {
            success: true,
            data: [],
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    document.body.removeChild(container);
    jest.clearAllMocks();
  });

  it('renders checkin overview data', async () => {
    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    expect(container.textContent).toContain('签到任务概览');
    expect(container.textContent).toContain('签到未签到渠道');
    expect(container.textContent).toContain('Provider-A');
    expect(container.textContent).toContain('奖励额度：$1.00');
    expect(container.textContent).toContain('今日所有已启用签到渠道均已签到');
    expect(container.textContent).toContain('余额汇总');
    expect(container.textContent).toContain('$12.50');
  });

  it('renders site health independently from checkin enabled state in provider rows', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [
                {
                  id: 1,
                  name: 'Provider-A',
                  base_url: 'https://a.example.com',
                  created_at: 1730000000,
                  status: 1,
                  checkin_enabled: true,
                  weight: 10,
                  priority: 0,
                  health_status: 'unreachable',
                  health_blocked: true,
                },
              ],
              p: 0,
              page_size: 10,
              total: 1,
              total_pages: 1,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/summary') {
        return Promise.resolve({ data: { success: true, data: {} } });
      }
      if (url === '/api/provider/checkin/summary?limit=1' || url === '/api/provider/checkin/messages?limit=20' || url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    expect(container.textContent).toContain('Provider-A');
    expect(container.textContent).toContain('不可用');
    expect(container.textContent).toContain('已启用');
  });

  it('triggers unchecked-only checkin run from overview action', async () => {
    API.post.mockResolvedValue({
      data: { success: true, message: '' },
    });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const runButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('签到未签到渠道'));
    expect(runButton).not.toBeNull();

    await act(async () => {
      runButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.post).toHaveBeenCalledWith('/api/provider/checkin/run');
  });

  it('keeps add-provider modal open when overlay is clicked', async () => {
    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const addButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('添加供应商'));
    expect(addButton).not.toBeNull();

    await act(async () => {
      addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    const overlay = container.querySelector('div[style*="position: fixed"]');
    expect(overlay).not.toBeNull();

    await act(async () => {
      overlay.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(container.textContent).toContain('添加供应商');
  });

  it('normalizes empty upstream user id to 0 on create provider submit', async () => {
    API.post.mockResolvedValue({ data: { success: true, message: '' } });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const addButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('添加供应商'));
    expect(addButton).not.toBeNull();

    await act(async () => {
      addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    const numberInputs = container.querySelectorAll('input[type="number"]');
    expect(numberInputs.length).toBeGreaterThan(0);
    expect(numberInputs[0].value).toBe('');

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '保存');
    expect(saveButton).not.toBeNull();

    await act(async () => {
      saveButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.post).toHaveBeenCalledWith('/api/provider/', expect.objectContaining({ user_id: 0 }));
  });

  it('resets provider list to first page when search keyword changes', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=0&page_size=10')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [{ id: 1, name: 'Provider-A', base_url: 'https://a.example.com', created_at: 1730000000, status: 1, checkin_enabled: false, weight: 10, priority: 0 }],
              p: 0,
              page_size: 10,
              total: 11,
              total_pages: 2,
              has_more: true,
            },
          },
        });
      }
      if (url.startsWith('/api/provider/?p=1&page_size=10')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [{ id: 2, name: 'Provider-B', base_url: 'https://b.example.com', created_at: 1730000000, status: 1, checkin_enabled: false, weight: 10, priority: 0 }],
              p: 1,
              page_size: 10,
              total: 11,
              total_pages: 2,
              has_more: false,
            },
          },
        });
      }
      if (url.includes('keyword=beta')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [{ id: 2, name: 'Provider-B', base_url: 'https://b.example.com', created_at: 1730000000, status: 1, checkin_enabled: false, weight: 10, priority: 0 }],
              p: 0,
              page_size: 10,
              total: 1,
              total_pages: 1,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/summary') {
        return Promise.resolve({ data: { success: true, data: {} } });
      }
      if (url === '/api/provider/checkin/summary?limit=1' || url === '/api/provider/checkin/messages?limit=20' || url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const nextPageButton = container.querySelector('button[aria-label="第 2 页"]');
    expect(nextPageButton).not.toBeNull();
    await act(async () => {
      nextPageButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    const searchInput = container.querySelector('input[placeholder="搜索名称、地址、备注、用户 ID"]');
    expect(searchInput).not.toBeNull();
    await act(async () => {
      setInputValue(searchInput, 'beta');
    });
    await flushPromises();
    await flushPromises();

    expect(API.get).toHaveBeenCalledWith(expect.stringContaining('/api/provider/?p=0&page_size=10&keyword=beta'));
  });

  it('submits provider proxy settings from the modal form', async () => {
    API.post.mockResolvedValue({ data: { success: true, message: '' } });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const addButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('添加供应商'));
    await act(async () => {
      addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    const proxyToggle = container.querySelector('input#proxy_enabled');
    expect(proxyToggle).not.toBeNull();
    await act(async () => {
      proxyToggle.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    const proxyInput = Array.from(container.querySelectorAll('input')).find((input) => input.placeholder === 'http://user:pass@proxy.example.com:7890');
    expect(proxyInput).not.toBeNull();
    await act(async () => {
      setInputValue(proxyInput, 'http://proxy.example.com:8080');
    });

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '保存');
    await act(async () => {
      saveButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.post).toHaveBeenCalledWith('/api/provider/', expect.objectContaining({
      proxy_enabled: true,
      proxy_url: 'http://proxy.example.com:8080',
    }));
  });

  it('renders already-signed message as success', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [],
              p: 0,
              page_size: 10,
              total: 0,
              total_pages: 0,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: 1,
                status: 'success',
                success_count: 1,
                failure_count: 0,
                ended_at: 1730000000,
              },
            ],
          },
        });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: 1,
                provider_name: 'Provider-A',
                status: 'success',
                message: '今日已签到',
                quota_awarded: 0,
                checked_at: 1730000000,
              },
            ],
          },
        });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({
          data: {
            success: true,
            data: [],
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    expect(container.textContent).toContain('今日已签到');
    expect(container.textContent).toContain('Provider-A成功今日已签到');
    expect(container.textContent).toContain('奖励额度：$0.00');
    expect(container.textContent).not.toContain('Provider-A失败今日已签到');
  });

  it('shows error when overview API fails', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [],
              p: 0,
              page_size: 10,
              total: 0,
              total_pages: 0,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({
          data: { success: false, message: 'summary error' },
        });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({
          data: { success: true, data: [] },
        });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({
          data: { success: true, data: [] },
        });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    expect(showError).toHaveBeenCalledWith('summary error');
  });

  it('renders upstream-disabled checkin message with auto-disable hint', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [],
              p: 0,
              page_size: 10,
              total: 0,
              total_pages: 0,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({
          data: {
            success: true,
            data: [],
          },
        });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: 99,
                provider_name: 'Provider-X',
                status: 'failed',
                message: 'checkin failed: 签到功能未启用',
                auto_disabled: true,
                quota_awarded: 0,
                checked_at: 1730000000,
              },
            ],
          },
        });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({
          data: {
            success: true,
            data: [],
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    expect(container.textContent).toContain('已自动关闭签到');
    expect(container.textContent).toContain('签到功能上游未启用，已自动关闭该供应商签到');
  });

  it('does not show auto-disable badge when message has keyword but auto_disabled is false', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [],
              p: 0,
              page_size: 10,
              total: 0,
              total_pages: 0,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({
          data: {
            success: true,
            data: [],
          },
        });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: 100,
                provider_name: 'Provider-Y',
                status: 'failed',
                message: 'checkin failed: 签到功能未启用',
                auto_disabled: false,
                quota_awarded: 0,
                checked_at: 1730000001,
              },
            ],
          },
        });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({
          data: {
            success: true,
            data: [],
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    expect(container.textContent).toContain('Provider-Y');
    expect(container.textContent).not.toContain('已自动关闭签到');
  });

  it('enables checkin from provider list with one click', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [
                {
                  id: 1,
                  name: 'Provider-A',
                  base_url: 'https://example.com',
                  created_at: 1730000000,
                  status: 1,
                  checkin_enabled: false,
                  weight: 10,
                  priority: 0,
                },
              ],
              p: 0,
              page_size: 10,
              total: 1,
              total_pages: 1,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });
    API.put.mockResolvedValue({ data: { success: true, message: '' } });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const enableButton = container.querySelector('button[title="一键开启签到"]');
    expect(enableButton).not.toBeNull();

    await act(async () => {
      enableButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.put).toHaveBeenCalledWith('/api/provider/', { id: 1, checkin_enabled: true });
  });

  it('rolls back one-click enable state when API fails', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [
                {
                  id: 2,
                  name: 'Provider-B',
                  base_url: 'https://example.com',
                  created_at: 1730000000,
                  status: 1,
                  checkin_enabled: false,
                  weight: 10,
                  priority: 0,
                },
              ],
              p: 0,
              page_size: 10,
              total: 1,
              total_pages: 1,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });
    API.put.mockResolvedValue({ data: { success: false, message: 'enable failed' } });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const enableButton = container.querySelector('button[title="一键开启签到"]');
    expect(enableButton).not.toBeNull();

    await act(async () => {
      enableButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(showError).toHaveBeenCalledWith('enable failed');
    expect(container.textContent).toContain('未启用');
  });

  it('disables checkin from provider list with one click', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [
                {
                  id: 3,
                  name: 'Provider-C',
                  base_url: 'https://example.com',
                  created_at: 1730000000,
                  status: 1,
                  checkin_enabled: true,
                  weight: 10,
                  priority: 0,
                },
              ],
              p: 0,
              page_size: 10,
              total: 1,
              total_pages: 1,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });
    API.put.mockResolvedValue({ data: { success: true, message: '' } });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const disableButton = container.querySelector('button[title="一键取消签到"]');
    expect(disableButton).not.toBeNull();

    await act(async () => {
      disableButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.put).toHaveBeenCalledWith('/api/provider/', { id: 3, checkin_enabled: false });
  });

  it('rolls back one-click disable state when API fails', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: {
            success: true,
            message: '',
            data: {
              items: [
                {
                  id: 4,
                  name: 'Provider-D',
                  base_url: 'https://example.com',
                  created_at: 1730000000,
                  status: 1,
                  checkin_enabled: true,
                  weight: 10,
                  priority: 0,
                },
              ],
              p: 0,
              page_size: 10,
              total: 1,
              total_pages: 1,
              has_more: false,
            },
          },
        });
      }
      if (url === '/api/provider/checkin/summary?limit=1') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/checkin/messages?limit=20') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/checkin/uncheckin') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });
    API.put.mockResolvedValue({ data: { success: false, message: 'disable failed' } });

    await act(async () => {
      root.render(<ProvidersTable />);
    });
    await flushPromises();
    await flushPromises();

    const disableButton = container.querySelector('button[title="一键取消签到"]');
    expect(disableButton).not.toBeNull();

    await act(async () => {
      disableButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(showError).toHaveBeenCalledWith('disable failed');
    expect(container.textContent).toContain('已启用');
  });
});
