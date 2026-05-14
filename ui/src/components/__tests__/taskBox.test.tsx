import { describe, it, expect, afterEach } from 'vitest';
import { render as solidRender } from 'solid-js/web';
import { fireEvent } from '@testing-library/dom';

vi.mock('/src/components/code/shellScriptViewer', () => ({
  default: (props: any) => <div data-testid="shell">{props.script}</div>
}));

vi.mock('/src/components/code/JsonToYamlViewer', () => ({
  default: (props: any) => <div data-testid="yaml">YAML</div>
}));

import TaskBox from '/src/components/containers/taskBox';

afterEach(() => {
  document.querySelectorAll('#vitest-root').forEach(el => el.remove());
});

describe('TaskBox', () => {
  it('toggles details on click and shows content', async () => {
    const taskDetails = {
      name: 'task-1',
      command: ['echo','hi'],
      args: ['a','b'],
      script: 'echo script',
      image: 'alpine',
      parameters: ['p1','p2'],
      backOffLimit: 2,
      isConditional: true,
      retryCodes: [1],
      podTemplate: { key: 'value' },
    };

    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => <TaskBox taskDetails={taskDetails as any} />, container);

    // Initially collapsed
    expect(container.textContent).toContain('Name: task-1');
    expect(container.textContent).not.toContain('Command:');

    // Click header to open
    const h4 = Array.from(container.querySelectorAll('h4')).find(el => el.textContent?.includes('Name: task-1')) as HTMLHeadingElement;
    expect(h4).toBeTruthy();
    const header = h4?.parentElement as HTMLElement;
    expect(header).toBeTruthy();
    if (header) await fireEvent.click(header);

    // Now details should be visible
    // Wait a tick for reactive update
    await new Promise(r => setTimeout(r, 0));
    expect(container.textContent).toContain('Command:');
    expect(container.querySelector('[data-testid="shell"]')).toBeTruthy();
    expect(container.querySelector('[data-testid="yaml"]')).toBeTruthy();

    dispose();
  });
});
