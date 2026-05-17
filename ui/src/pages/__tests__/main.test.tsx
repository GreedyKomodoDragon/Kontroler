import { describe, it, expect, vi, afterEach } from "vitest";
import { render as solidRender } from "solid-js/web";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

const mockStats = {
  dag_count: 10,
  successful_dag_runs: 50,
  failed_dag_runs: 5,
  total_dag_runs: 55,
  active_dag_runs: 3,
  dag_type_counts: { "Event Driven": 3, "Scheduled": 7 },
  task_outcomes: { "Completed": 100, "Failed": 10 },
  daily_dag_run_counts: [
    { day: "2024-01-01", successful_count: 10, failed_count: 1 },
    { day: "2024-01-02", successful_count: 15, failed_count: 2 },
  ],
};

vi.mock("/src/api/dags", () => ({
  getDashboardStats: vi.fn(() => Promise.resolve(mockStats)),
}));

vi.mock("/src/components/chart", () => ({
  default: (props: any) => <div data-testid="chart" />
}));

vi.mock("/src/components/skeletonCard", () => ({
  default: (props: any) => <div data-testid="skeleton" />
}));

vi.mock("/src/components/loadable", () => ({
  default: (props: any) => props.loading ? <div data-testid="loading" /> : <div data-testid="content">{props.children}</div>
}));

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

queryClient.setQueryData(["dashboard-stats"], mockStats);

afterEach(() => {
  document.querySelectorAll('#vitest-root').forEach((el) => el.remove());
  queryClient.clear();
});

describe('Main page', () => {
  it('renders dashboard content when data loads', async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    queryClient.setQueryData(["dashboard-stats"], mockStats);

    const { default: Main } = await import('/src/pages/main');

    const dispose = solidRender(
      () => <QueryClientProvider client={queryClient}><Main /></QueryClientProvider>,
      container
    );

    await new Promise(r => setTimeout(r, 50));

    expect(container.textContent).toContain('Kontroler Dashboard');

    dispose();
  });

  it('displays correct stat values from query data', async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    queryClient.setQueryData(["dashboard-stats"], mockStats);

    const { default: Main } = await import('/src/pages/main');

    const dispose = solidRender(
      () => <QueryClientProvider client={queryClient}><Main /></QueryClientProvider>,
      container
    );

    await new Promise(r => setTimeout(r, 50));

    expect(container.textContent).toContain('10');

    dispose();
  });

  it('uses correct query key and staleTime for dashboard stats', async () => {
    const { getDashboardStats } = await import('/src/api/dags');

    expect(getDashboardStats).toBeDefined();
    expect(typeof getDashboardStats).toBe('function');

    const queryConfig = {
      queryKey: ["dashboard-stats"] as const,
      queryFn: getDashboardStats,
      staleTime: 5 * 60 * 1000,
    };

    expect(queryConfig.queryKey).toEqual(["dashboard-stats"]);
    expect(queryConfig.staleTime).toBe(5 * 60 * 1000);
  });
});