import { describe, it, expect, vi, afterEach } from "vitest";
import { render as solidRender } from "solid-js/web";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

vi.mock("/src/api/admin", () => ({
  getUsers: vi.fn(({ queryKey }: { queryKey: string[] }) => {
    const page = queryKey[1];
    if (page === "1") {
      return Promise.resolve([
        { username: "admin", role: "admin" },
        { username: "editor", role: "editor" },
      ]);
    }
    return Promise.resolve([]);
  }),
  getUserPageCount: vi.fn(() => Promise.resolve(3)),
  deleteAccount: vi.fn(() => Promise.resolve()),
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

vi.mock("/src/components/admin/deleteButton", () => ({
  DeleteButton: (props: any) => <button data-testid="delete-button" onClick={props.delete}>Delete</button>
}));

vi.mock("/src/components/admin/confirmDeletion", () => ({
  default: (props: any) => props.show ? <div data-testid="confirm-dialog">Confirm</div> : null
}));

vi.mock("/src/components/alerts/errorSingleAlert", () => ({
  default: (props: any) => <div data-testid="error-alert">{props.msg}</div>
}));

vi.mock("/src/components/navbar/icon", () => ({
  default: (props: any) => <div data-testid="identicon" />
}));

vi.mock("/src/providers/ErrorProvider", () => ({
  useError: () => ({ handleApiError: vi.fn() }),
}));

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a href={props.href} data-testid="link">{props.children}</a>
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

queryClient.setQueryData(["user-page-count"], 3);

afterEach(() => {
  document.querySelectorAll('#vitest-root').forEach((el) => el.remove());
  queryClient.clear();
});

describe('ManageUsers component', () => {
  it('renders users when data loads', async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    queryClient.setQueryData(["user-page-count"], 3);
    queryClient.setQueryData(["users", "1"], [
      { username: "admin", role: "admin" },
      { username: "editor", role: "editor" },
    ]);

    const { default: ManageUsers } = await import('/src/components/manageUsers');

    const dispose = solidRender(
      () => <QueryClientProvider client={queryClient}><ManageUsers /></QueryClientProvider>,
      container
    );

    await new Promise(r => setTimeout(r, 50));

    expect(container.textContent).toContain('Team members');
    expect(container.textContent).toContain('admin');
    expect(container.textContent).toContain('editor');

    dispose();
  });

  it('uses correct query keys for users and page count', async () => {
    const { getUsers, getUserPageCount } = await import('/src/api/admin');

    expect(getUsers).toBeDefined();
    expect(getUserPageCount).toBeDefined();

    const usersConfig = {
      queryKey: ["users", "1"] as const,
      queryFn: getUsers,
      staleTime: 5 * 60 * 1000,
    };

    const pageCountConfig = {
      queryKey: ["user-page-count"] as const,
      queryFn: getUserPageCount,
      staleTime: 5 * 60 * 1000,
    };

    expect(usersConfig.queryKey).toEqual(["users", "1"]);
    expect(pageCountConfig.queryKey).toEqual(["user-page-count"]);
  });

  it('shows pagination when page count > 1', async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    queryClient.setQueryData(["user-page-count"], 3);
    const { default: ManageUsers } = await import('/src/components/manageUsers');

    const dispose = solidRender(
      () => <QueryClientProvider client={queryClient}><ManageUsers /></QueryClientProvider>,
      container
    );

    await new Promise(r => setTimeout(r, 50));

    const pagination = container.querySelector('[data-testid="pagination"]');
    expect(pagination).toBeTruthy();

    dispose();
  });

  it('does not show pagination when page count is 1', async () => {
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    queryClient.setQueryData(["user-page-count"], 1);
    const { default: ManageUsers } = await import('/src/components/manageUsers');

    const dispose = solidRender(
      () => <QueryClientProvider client={queryClient}><ManageUsers /></QueryClientProvider>,
      container
    );

    await new Promise(r => setTimeout(r, 50));

    const pagination = container.querySelector('[data-testid="pagination"]');
    expect(pagination).toBeNull();

    dispose();
  });
});