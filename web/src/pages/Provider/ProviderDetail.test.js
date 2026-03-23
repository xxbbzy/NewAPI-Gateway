import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import ProviderDetail from './ProviderDetail';
import { API, showError, showSuccess } from '../../helpers';

jest.mock('react-router-dom', () => ({
  ...jest.requireActual('react-router-dom'),
  useParams: () => ({ id: '1' }),
  useNavigate: () => jest.fn(),
}));

jest.mock('../../helpers', () => ({
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

describe('ProviderDetail token group form', () => {
  let container;
  let root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);

    API.get.mockImplementation((url) => {
      if (url === '/api/provider/1') {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              id: 1,
              name: 'Provider-A',
              base_url: 'https://example.com',
              status: 1,
              checkin_enabled: false,
              weight: 10,
              priority: 0,
              balance: '$0.00',
            },
          },
        });
      }
      if (url === '/api/provider/1/tokens') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/1/pricing') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: 1,
                model_name: 'gpt-4o',
                enable_groups: '["default","vip"]',
                quota_type: 0,
                model_ratio: 1,
                completion_ratio: 1,
                model_price: 0,
                supported_endpoint_types: '[]',
              },
            ],
            group_ratio: { default: 1, vip: 1.5 },
            token_group_options: [
              { group_name: 'default', ratio: 1 },
              { group_name: 'vip', ratio: 1.5 },
            ],
            default_group: 'vip',
            supported_endpoint: {},
          },
        });
      }
      if (url === '/api/provider/1/model-alias-mapping') {
        return Promise.resolve({ data: { success: true, data: {} } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    API.post.mockResolvedValue({ data: { success: true, message: '' } });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    document.body.removeChild(container);
    jest.clearAllMocks();
  });

  it('uses dropdown default group and blocks empty selection without closing on overlay click', async () => {
    await act(async () => {
      root.render(<ProviderDetail />);
    });
    await flushPromises();
    await flushPromises();

    const createButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('创建上游令牌'));
    expect(createButton).not.toBeNull();

    await act(async () => {
      createButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    const groupSelect = container.querySelector('select[name="group_name"]');
    expect(groupSelect).not.toBeNull();
    expect(groupSelect.value).toBe('vip');
    expect(groupSelect.textContent).toContain('default (x1)');
    expect(groupSelect.textContent).toContain('vip (x1.5)');

    await act(async () => {
      groupSelect.value = '';
      groupSelect.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await flushPromises();

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '保存');
    expect(saveButton).not.toBeNull();

    await act(async () => {
      saveButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(showError).toHaveBeenCalledWith('分组不能为空');
    expect(API.post).not.toHaveBeenCalledWith('/api/provider/1/tokens', expect.anything());

    const overlay = container.querySelector('div[style*="position: fixed"]');
    expect(overlay).not.toBeNull();

    await act(async () => {
      overlay.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(container.textContent).toContain('在上游创建令牌');
  });

  it('keeps site health driven by health_status while showing checkin result separately', async () => {
    API.get.mockImplementation((url) => {
      if (url === '/api/provider/1') {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              id: 1,
              name: 'Provider-A',
              base_url: 'https://example.com',
              status: 1,
              checkin_enabled: true,
              weight: 10,
              priority: 0,
              balance: '$0.00',
              balance_updated: 1730000001,
              health_status: 'healthy',
              health_success_at: 1730000000,
              last_checkin_at: 1730000002,
              last_checkin_status: 'failed',
            },
          },
        });
      }
      if (url === '/api/provider/1/tokens') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/provider/1/pricing') {
        return Promise.resolve({ data: { success: true, data: [], group_ratio: {}, token_group_options: [], default_group: '', supported_endpoint: {} } });
      }
      if (url === '/api/provider/1/model-alias-mapping') {
        return Promise.resolve({ data: { success: true, data: {} } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    await act(async () => {
      root.render(<ProviderDetail />);
    });
    await flushPromises();
    await flushPromises();

    expect(container.textContent).toContain('站点健康');
    expect(container.textContent).toContain('可访问');
    expect(container.textContent).toContain('最近成功：ts:1730000000');
    expect(container.textContent).toContain('签到');
    expect(container.textContent).toContain('最近结果：failed');
    expect(container.textContent).not.toContain('不可用');
  });

  it('shows unresolved create success messaging when key recovery is pending', async () => {
    API.post.mockResolvedValueOnce({
      data: {
        success: true,
        message: '',
        data: {
          upstream_created: true,
          created_token_identified: true,
          key_status: 'unresolved',
        },
      },
    });

    await act(async () => {
      root.render(<ProviderDetail />);
    });
    await flushPromises();
    await flushPromises();

    const createButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('创建上游令牌'));
    expect(createButton).not.toBeNull();

    await act(async () => {
      createButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '保存');
    expect(saveButton).not.toBeNull();

    await act(async () => {
      saveButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(showSuccess).toHaveBeenCalledWith('Token 已在上游创建，但明文密钥暂未恢复，请稍后同步后重试');
  });

  it('renders unresolved tokens as non-copyable with explicit recovery hints', async () => {
    API.get.mockImplementation((url) => {
      if (url === '/api/provider/1') {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              id: 1,
              name: 'Provider-A',
              base_url: 'https://example.com',
              status: 1,
              checkin_enabled: false,
              weight: 10,
              priority: 0,
              balance: '$0.00',
            },
          },
        });
      }
      if (url === '/api/provider/1/tokens') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: 1,
                name: 'ready-token',
                sk_key: 'sk-ready-key',
                key_status: 'ready',
                group_name: 'default',
                status: 1,
                unlimited_quota: true,
                remain_quota: 0,
                priority: 0,
                weight: 10,
              },
              {
                id: 2,
                name: 'pending-token',
                sk_key: '',
                key_status: 'unresolved',
                key_unresolved_reason: 'plaintext_not_recovered',
                group_name: 'vip',
                status: 1,
                unlimited_quota: false,
                remain_quota: 99,
                priority: 1,
                weight: 8,
              },
            ],
          },
        });
      }
      if (url === '/api/provider/1/pricing') {
        return Promise.resolve({
          data: {
            success: true,
            data: [],
            group_ratio: {},
            token_group_options: [],
            default_group: '',
            supported_endpoint: {},
          },
        });
      }
      if (url === '/api/provider/1/model-alias-mapping') {
        return Promise.resolve({ data: { success: true, data: {} } });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    await act(async () => {
      root.render(<ProviderDetail />);
    });
    await flushPromises();
    await flushPromises();

    expect(container.querySelector('#copy-btn-1')).not.toBeNull();
    expect(container.querySelector('#copy-btn-2')).toBeNull();
    expect(container.textContent).toContain('令牌已在上游创建，明文密钥暂不可复制');
    expect(container.textContent).toContain('原因：上游尚未返回可用明文密钥');
  });
});
