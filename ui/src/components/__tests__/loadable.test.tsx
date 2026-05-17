import { describe, it, expect, vi, afterEach } from "vitest";
import { render as solidRender } from "solid-js/web";
import { fireEvent } from "@testing-library/dom";
import Loadable from "../loadable";

afterEach(() => {
  // remove any containers from previous renders
  document.querySelectorAll('#vitest-root').forEach((el) => el.remove());
});

describe("Loadable component", () => {
  it("renders skeleton when loading and skeleton provided", async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => (
      <Loadable loading={true} skeleton={<div data-testid="skeleton" />}> 
        <div>child</div>
      </Loadable>
    ), container);

    expect(container.querySelector('[data-testid="skeleton"]')).toBeTruthy();
    expect(container.textContent).not.toContain('child');
    dispose();
  });

  it("renders spinner when loading and no skeleton provided", async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => (
      <Loadable loading={true}>
        <div>child</div>
      </Loadable>
    ), container);

    // Spinner is an svg element
    const svg = container.querySelector("svg");
    expect(svg).toBeTruthy();
    expect(container.textContent).not.toContain('child');
    dispose();
  });

  it("renders error and calls onRetry when retry clicked", async () => {
    const onRetry = vi.fn();
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => (
      <Loadable loading={false} error={"network"} onRetry={onRetry}>
        <div>child</div>
      </Loadable>
    ), container);

    expect(container.textContent).toContain("network");
    const btn = Array.from(container.querySelectorAll('button')).find(b => b.textContent === 'Retry');
    expect(btn).toBeTruthy();
    if (btn) await fireEvent.click(btn);
    expect(onRetry).toHaveBeenCalled();
    // children should not be shown
    expect(container.textContent).not.toContain('child');
    dispose();
  });

  it("renders children when not loading and no error", () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => (
      <Loadable loading={false} error={undefined}>
        <div>child</div>
      </Loadable>
    ), container);

    expect(container.textContent).toContain('child');
    dispose();
  });
});
