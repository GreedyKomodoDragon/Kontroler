import { describe, it, expect, vi, afterEach } from 'vitest';
import { render as solidRender } from 'solid-js/web';

// Mock prismjs highlightAllUnder
vi.mock('prismjs', () => {
  const m = { highlightAllUnder: vi.fn() };
  return m;
});

import LogHighlighter from '/src/components/code/logViewer';
import { highlightAllUnder } from 'prismjs';

afterEach(() => {
  document.querySelectorAll('#vitest-root').forEach(el => el.remove());
  (highlightAllUnder as any).mockReset?.();
});

describe('LogHighlighter', () => {
  it('assigns correct classes and renders numbered lines', async () => {
    const logs = [
      'this is info',
      'WARN something happened',
      'ERROR fatal',
    ].join('\n');

    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => <LogHighlighter logs={logs} />, container);

    // allow effects to run
    await new Promise((r) => setTimeout(r, 0));

    const codes = container.querySelectorAll('code');
    expect(codes.length).toBe(3);

    expect(codes[0].className).toContain('language-info');
    expect(codes[0].textContent).toContain('0: this is info');

    expect(codes[1].className).toContain('language-warning');
    expect(codes[1].textContent).toContain('1: WARN something happened');

    expect(codes[2].className).toContain('language-error');
    expect(codes[2].textContent).toContain('2: ERROR fatal');

    dispose();
  });

  it('pads line numbers based on total lines', async () => {
    const lines = Array.from({ length: 12 }).map((_, i) => `line ${i}`);
    const logs = lines.join('\n');

    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => <LogHighlighter logs={logs} />, container);
    await new Promise((r) => setTimeout(r, 0));

    const codes = container.querySelectorAll('code');
    // last index is 11; maxDigits = String(11).length = 2, so indices should be padded to width 2
    expect(codes[0].textContent?.startsWith(' 0:') || codes[0].textContent?.startsWith('0:')).toBeTruthy();
    expect(codes[10].textContent).toMatch(/^10:/);
    expect(codes[11].textContent).toMatch(/^11:/);

    dispose();
  });

  it('calls prism highlightAllUnder', async () => {
    const logs = 'a';
    const container = document.createElement('div');
    container.id = 'vitest-root';
    document.body.appendChild(container);

    const dispose = solidRender(() => <LogHighlighter logs={logs} />, container);
    await new Promise((r) => setTimeout(r, 0));

    expect((highlightAllUnder as any)).toHaveBeenCalled();

    dispose();
  });
});
