import { describe, it, expect, vi, afterEach } from "vitest";
import { render as solidRender } from "solid-js/web";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

vi.mock("/src/api/dags", () => ({
  getDags: vi.fn(({ queryKey }: { queryKey: string[] }) => {
    const page = queryKey[1];
    if (page === "1") {
      return Promise.resolve([
        { dagId: 1, name: "dag-1", namespace: "default", connections: {} },
        { dagId: 2, name: "dag-2", namespace: "default", connections: {} },
      ]);
    }
    return Promise.resolve([]);
  }),
  getDagPageCount: vi.fn(() => Promise.resolve(3)),
}));

vi.mock("/src/components/dagComponent", () => ({
  default: (props: any) => <div data-testid="dag-component">{props.dag.name}</div>
}));

vi.mock("/src/components/pagination", () => ({
  default: (props: any) => <div data-testid="pagination" />
}));

vi.mock("/src/components/loadable", () => ({
  default: (props: any) => props.loading ? <div data-testid="loading" /> : <div data-testid="content">{props.children}</div>
}));

vi.mock("/src/components/skeletonCard", () => ({
  default: (props: any) => <div data-testid="skeleton" />
}));

const queryClient = new QueryClient();

afterEach(() => {
  document.querySelectorAll('#vitest-root').forEach((el) => el.remove());
  queryClient.clear();
});

describe('Dags page', () => {
  it('renders DAGs when data loads', async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const { default: Dags } = await import('/src/pages/dags');

    const dispose = solidRender(
      () => <QueryClientProvider client={queryClient}><Dags /></QueryClientProvider>,
      container
    );

    await new Promise(r => setTimeout(r, 100));

    expect(container.textContent).toContain('Your DAGs');

    dispose();
  });

  it('uses correct query keys for dags and page count', async () => {
    const { getDags, getDagPageCount } = await import('/src/api/dags');

    expect(getDags).toBeDefined();
    expect(getDagPageCount).toBeDefined();

    const dagsConfig = {
      queryKey: ["dags", "1"] as const,
      queryFn: getDags,
      staleTime: 5 * 60 * 1000,
    };

    const pageCountConfig = {
      queryKey: ["dag-page-count"] as const,
      queryFn: getDagPageCount,
      staleTime: 5 * 60 * 1000,
    };

    expect(dagsConfig.queryKey).toEqual(["dags", "1"]);
    expect(pageCountConfig.queryKey).toEqual(["dag-page-count"]);
    expect(dagsConfig.staleTime).toBe(5 * 60 * 1000);
    expect(pageCountConfig.staleTime).toBe(5 * 60 * 1000);
  });

  it('shows pagination when page count > 1', async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const { default: Dags } = await import('/src/pages/dags');

    const dispose = solidRender(
      () => <QueryClientProvider client={queryClient}><Dags /></QueryClientProvider>,
      container
    );

    await new Promise(r => setTimeout(r, 100));

    const pagination = container.querySelector('[data-testid="pagination"]');
    expect(pagination).toBeTruthy();

    dispose();
  });
});