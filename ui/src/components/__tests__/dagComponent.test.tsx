import { describe, it, expect, vi, afterEach } from 'vitest';
import { render as solidRender } from 'solid-js/web';
import { fireEvent } from '@testing-library/dom';

// mocks for modules the component depends on
vi.mock('/src/api/dags', () => ({
  getTaskDetails: vi.fn((id: number) => Promise.resolve({
    id,
    name: 'task-name',
    command: ['echo','hello'],
    args: ['a','b'],
    image: 'alpine',
    parameters: [{ name: 'p1', isSecret: false, defaultValue: 'v' }],
    backOffLimit: 3,
    isConditional: false,
    retryCodes: [1,2],
  })),
  deleteDag: vi.fn(() => Promise.resolve()),
  suspendDag: vi.fn(() => Promise.resolve()),
}));

vi.mock('/src/components/dagViz', () => ({
  default: (props: any) => {
    return (
      <div>
        <button data-testid="select-task" onClick={() => props.setSelectedTask(1)}>Select</button>
      </div>
    );
  }
}));

vi.mock('/src/components/deleteTaskButton', () => ({
  DeleteTaskButton: (props: any) => (
    <button data-testid="delete-button" onClick={() => props.delete(props.taskIndex)}>
      Delete
    </button>
  )
}));

vi.mock('/src/components/code/shellScriptViewer', () => ({
  default: (props: any) => <div data-testid="shell">{props.script}</div>
}));

vi.mock('/src/components/code/JsonToYamlViewer', () => ({
  default: (props: any) => <div data-testid="yaml">YAML</div>
}));

afterEach(() => {
  document.querySelectorAll('#vitest-root').forEach(el => el.remove());
});

describe('DagComponent', () => {
  it('renders basic dag info and toggles diagram and shows task details', async () => {
    const { default: DagComponent } = await import('/src/components/dagComponent');

    const dag = {
      name: 'my-dag',
      dagId: 42,
      schedule: '* * * * *',
      namespace: 'default',
      isSuspended: false,
      connections: {},
    };

    const onDelete = vi.fn();

    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const { ErrorProvider } = await import('/src/providers/ErrorProvider');
    const dispose = solidRender(() => <ErrorProvider>{<DagComponent dag={dag as any} onDelete={onDelete} />}</ErrorProvider>, container);

    // Basic info visible
    expect(container.textContent).toContain('my-dag');
    expect(container.textContent).toContain('ID:');

    // Click show diagram
    const btn = Array.from(container.querySelectorAll('button')).find(b => /Show Diagram|Hide Diagram/.test(b.textContent || '')) as HTMLButtonElement;
    expect(btn).toBeTruthy();
    if (btn) await fireEvent.click(btn);

    // After opening, the DagViz mock renders a select button
    const selectBtn = container.querySelector('[data-testid="select-task"]') as HTMLButtonElement;
    expect(selectBtn).toBeTruthy();

    // Click to select task -> should trigger getTaskDetails and render task details
    if (selectBtn) await fireEvent.click(selectBtn);

    // Wait for microtask resolution
    await new Promise(r => setTimeout(r, 0));

    expect(container.textContent).toContain('Task Details');
    expect(container.textContent).toContain('task-name');
    expect(container.textContent).toContain('alpine');

    dispose();
  });

  it('calls deleteDag and onDelete when delete button clicked', async () => {
    const api = await import('/src/api/dags');
    const { default: DagComponent } = await import('/src/components/dagComponent');

    const dag = {
      name: 'my-dag',
      dagId: 99,
      namespace: 'default',
      connections: {},
    };

    const onDelete = vi.fn();
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const { ErrorProvider } = await import('/src/providers/ErrorProvider');
    const dispose = solidRender(() => <ErrorProvider>{<DagComponent dag={dag as any} onDelete={onDelete} />}</ErrorProvider>, container);

    const del = container.querySelector('[data-testid="delete-button"]') as HTMLButtonElement;
    expect(del).toBeTruthy();
    if (del) await fireEvent.click(del);

    // deleteDag mock should have been called
    expect((api as any).deleteDag).toHaveBeenCalledWith('default', 'my-dag');

    // onDelete should be called by the component after delete resolves
    await new Promise(r => setTimeout(r, 0));
    expect(onDelete).toHaveBeenCalled();

    dispose();
  });
});
