import { describe, it, expect, vi, afterEach } from "vitest";
import { render as solidRender } from "solid-js/web";
import { fireEvent } from "@testing-library/dom";
import PaginationComponent from "../pagination";

afterEach(() => {
  document.querySelectorAll('#vitest-root').forEach((el) => el.remove());
});

describe('PaginationComponent', () => {
  it('renders pagination buttons and current page info', () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const setPage = vi.fn();
    const maxPage = () => 3;

    const dispose = solidRender(
      () => <PaginationComponent setPage={setPage} maxPage={maxPage} />,
      container
    );

    expect(container.textContent).toContain('Current page:');
    expect(container.textContent).toContain('/ 3');

    const buttons = container.querySelectorAll('button');
    expect(buttons.length).toBeGreaterThan(0);

    dispose();
  });

  it('calls setPage when clicking on a page button', async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const setPage = vi.fn();
    const maxPage = () => 3;

    const dispose = solidRender(
      () => <PaginationComponent setPage={setPage} maxPage={maxPage} />,
      container
    );

    const buttons = container.querySelectorAll('button');
    const pageButton = Array.from(buttons).find(b => b.textContent?.trim() === '2');
    expect(pageButton).toBeTruthy();

    if (pageButton) {
      await fireEvent.click(pageButton);
      expect(setPage).toHaveBeenCalledWith(2);
    }

    dispose();
  });

  it('renders with given maxPage', () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const setPage = vi.fn();
    const maxPage = () => 5;

    const dispose = solidRender(
      () => <PaginationComponent setPage={setPage} maxPage={maxPage} />,
      container
    );

    expect(container.textContent).toContain('/ 5');

    dispose();
  });
});