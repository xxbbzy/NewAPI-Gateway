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
  showError: jest.fn(),
  showSuccess: jest.fn(),
  timestamp2string: (value) => `ts:${value}`,
}));

const flushPromises = async () => {
  await act(async () => {
    await Promise.resolve();
  });
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
          data: { success: true, message: '', data: [] },
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
                quota_awarded: 100,
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
    expect(container.textContent).toContain('Provider-A');
    expect(container.textContent).toContain('今日所有已启用签到渠道均已签到');
  });

  it('renders already-signed message as success', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: { success: true, message: '', data: [] },
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
    expect(container.textContent).not.toContain('Provider-A失败今日已签到');
  });

  it('shows error when overview API fails', async () => {
    API.get.mockImplementation((url) => {
      if (url.startsWith('/api/provider/?p=')) {
        return Promise.resolve({
          data: { success: true, message: '', data: [] },
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
          data: { success: true, message: '', data: [] },
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
          data: { success: true, message: '', data: [] },
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
            data: [
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
            data: [
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
});
