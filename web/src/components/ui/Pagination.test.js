import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import Pagination from './Pagination';

global.IS_REACT_ACT_ENVIRONMENT = true;

describe('Pagination', () => {
  let container;
  let root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    document.body.removeChild(container);
    jest.clearAllMocks();
  });

  it('disables previous button on first page and triggers next page change', async () => {
    const onPageChange = jest.fn();
    await act(async () => {
      root.render(<Pagination activePage={1} totalPages={3} onPageChange={onPageChange} />);
    });

    const prevButton = container.querySelector('button[aria-label="上一页"]');
    const nextButton = container.querySelector('button[aria-label="下一页"]');
    expect(prevButton).not.toBeNull();
    expect(nextButton).not.toBeNull();
    expect(prevButton.disabled).toBe(true);
    expect(nextButton.disabled).toBe(false);

    await act(async () => {
      nextButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(onPageChange).toHaveBeenCalledWith(null, { activePage: 2 });
  });

  it('renders ellipsis for large page ranges', async () => {
    await act(async () => {
      root.render(<Pagination activePage={5} totalPages={12} onPageChange={jest.fn()} />);
    });

    expect(container.textContent).toContain('...');
    expect(container.querySelector('button[aria-label="第 5 页"]')).not.toBeNull();
  });
});
