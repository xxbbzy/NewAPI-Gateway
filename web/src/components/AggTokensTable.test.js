import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import AggTokensTable from './AggTokensTable';
import { API, copy, showError, showSuccess } from '../helpers';

jest.mock('../helpers', () => ({
  API: {
    get: jest.fn(),
    delete: jest.fn(),
    put: jest.fn(),
    post: jest.fn(),
  },
  copy: jest.fn(),
  normalizePaginatedData: jest.requireActual('../helpers/utils').normalizePaginatedData,
  showError: jest.fn(),
  showSuccess: jest.fn(),
}));

const flushPromises = async () => {
  await act(async () => {
    await Promise.resolve();
  });
};

global.IS_REACT_ACT_ENVIRONMENT = true;

const findCopyButton = (container) => {
  const tokenCode = Array.from(container.querySelectorAll('code')).find((node) =>
    node.textContent.includes('ag-abcdef')
  );
  expect(tokenCode).not.toBeNull();
  return tokenCode.parentElement.querySelector('button');
};

describe('AggTokensTable copy action', () => {
  let container;
  let root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);

    API.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          items: [
            {
              id: 1,
              name: 'Aggregated Token',
              key: 'abcdef123456',
              status: 1,
              expired_time: -1,
              model_limits_enabled: false,
              model_limits: '',
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
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    document.body.removeChild(container);
    jest.clearAllMocks();
  });

  it('shows success feedback only after a confirmed clipboard write', async () => {
    copy.mockResolvedValue(true);

    await act(async () => {
      root.render(<AggTokensTable />);
    });
    await flushPromises();
    await flushPromises();

    const copyButton = findCopyButton(container);
    expect(copyButton).not.toBeNull();

    await act(async () => {
      copyButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(copy).toHaveBeenCalledWith('ag-abcdef123456');
    expect(showSuccess).toHaveBeenCalledWith('已复制');
    expect(showError).not.toHaveBeenCalled();
  });

  it('shows explicit failure feedback when clipboard write fails', async () => {
    copy.mockResolvedValue(false);

    await act(async () => {
      root.render(<AggTokensTable />);
    });
    await flushPromises();
    await flushPromises();

    const copyButton = findCopyButton(container);
    expect(copyButton).not.toBeNull();

    await act(async () => {
      copyButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(copy).toHaveBeenCalledWith('ag-abcdef123456');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalledWith('复制失败：剪贴板不可用或权限被拒绝');
  });
});
