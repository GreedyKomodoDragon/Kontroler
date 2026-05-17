import { describe, it, expect, afterEach } from "vitest";
import { render as solidRender } from "solid-js/web";
import SkeletonCard from "../skeletonCard";

afterEach(() => {
  document.querySelectorAll('#vitest-root').forEach((el) => el.remove());
});

describe('SkeletonCard component', () => {
  it('renders default title and body placeholders', () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => <SkeletonCard />, container);

    // title placeholders have class bg-gray-600
    const titles = container.querySelectorAll('.bg-gray-600');
    expect(titles.length).toBeGreaterThanOrEqual(1);

    // body placeholders have class bg-gray-700
    const bodies = container.querySelectorAll('.bg-gray-700');
    expect(bodies.length).toBeGreaterThanOrEqual(1);

    dispose();
  });

  it('renders the specified number of title and body lines', () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => <SkeletonCard titleLines={2} bodyLines={3} />, container);

    const titles = container.querySelectorAll('.bg-gray-600');
    expect(titles.length).toBe(2);

    const bodies = container.querySelectorAll('.bg-gray-700');
    expect(bodies.length).toBe(3);

    dispose();
  });

  it('applies provided height class to body placeholders', () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => <SkeletonCard bodyLines={2} height={'h-24'} />, container);

    const bodies = Array.from(container.querySelectorAll('.bg-gray-700'));
    expect(bodies.length).toBe(2);
    bodies.forEach((el) => {
      // each body placeholder should include the height class
      expect(el.className).toContain('h-24');
    });

    dispose();
  });
});
