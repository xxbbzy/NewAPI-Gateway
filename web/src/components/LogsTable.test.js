import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import LogsTable from './LogsTable';
import { API } from '../helpers';

jest.mock('../helpers', () => ({
  API: {
    get: jest.fn(),
  },
  showError: jest.fn(),
  normalizePaginatedData: jest.requireActual('../helpers/utils').normalizePaginatedData,
}));

const flushPromises = async () => {
  await act(async () => {
    await Promise.resolve();
  });
};

global.IS_REACT_ACT_ENVIRONMENT = true;

describe('LogsTable pagination', () => {
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
          items: [],
          p: 0,
          page_size: 10,
          total: 0,
          total_pages: 0,
          has_more: false,
          providers: [],
          summary: {
            total: 0,
            success_count: 0,
            error_count: 0,
            input_tokens: 0,
            output_tokens: 0,
            cache_tokens: 0,
            total_cost: 0,
            avg_latency: 0
          }
        }
      }
    });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    document.body.removeChild(container);
    jest.clearAllMocks();
  });

  it('disables next button when has_more is false', async () => {
    API.get.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          items: [
            { id: 1, model_name: 'gpt-4o', provider_name: 'p1', status: 1, created_at: 1730000000 }
          ],
          p: 0,
          page_size: 10,
          total: 1,
          total_pages: 1,
          has_more: false,
          providers: ['p1'],
          summary: {
            total: 1,
            success_count: 1,
            error_count: 0,
            input_tokens: 1,
            output_tokens: 1,
            cache_tokens: 0,
            total_cost: 0,
            avg_latency: 1
          }
        }
      }
    });

    await act(async () => {
      root.render(<LogsTable selfOnly={false} />);
    });
    await flushPromises();
    await flushPromises();

    const prevButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('上一页'));
    const nextButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('下一页'));
    expect(prevButton).not.toBeNull();
    expect(nextButton).not.toBeNull();
    expect(prevButton.disabled).toBe(true);
    expect(nextButton.disabled).toBe(true);
  });

  it('resets to first page after filter changes', async () => {
    API.get.mockImplementation((url) => {
      if (url.includes('p=1')) {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              items: [{ id: 2, model_name: 'gpt-4.1', provider_name: 'p1', status: 1, created_at: 1730000100 }],
              p: 1,
              page_size: 10,
              total: 11,
              total_pages: 2,
              has_more: false,
              providers: ['p1']
            }
          }
        });
      }
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: [{ id: 1, model_name: 'gpt-4o', provider_name: 'p1', status: 1, created_at: 1730000000 }],
            p: 0,
            page_size: 10,
            total: 11,
            total_pages: 2,
            has_more: true,
            providers: ['p1']
          }
        }
      });
    });

    await act(async () => {
      root.render(<LogsTable selfOnly={false} />);
    });
    await flushPromises();
    await flushPromises();

    const nextButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('下一页'));
    expect(nextButton).not.toBeNull();

    await act(async () => {
      nextButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();
    await flushPromises();

    expect(API.get.mock.calls.some(([url]) => url.includes('p=1'))).toBe(true);

    const errorTabButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes('仅错误'));
    expect(errorTabButton).not.toBeNull();
    await act(async () => {
      errorTabButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();
    await flushPromises();

    const matchingCalls = API.get.mock.calls.map(([url]) => url).filter((url) => url.includes('view=error'));
    expect(matchingCalls.length).toBeGreaterThan(0);
    expect(matchingCalls[matchingCalls.length - 1]).toContain('p=0');
  });
});
