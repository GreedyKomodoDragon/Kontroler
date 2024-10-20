import { highlightElement } from "prismjs";
import { createEffect } from "solid-js";
import "prismjs/themes/prism.css";
import "prismjs/components/prism-bash";

type ShellScriptViewerProps = {
  script: string;
};

export default function ShellScriptViewer(props: ShellScriptViewerProps) {
  let codeElement: HTMLPreElement | undefined;

  createEffect(() => {
    if (codeElement) {
      codeElement.textContent = props.script;
      highlightElement(codeElement);
    }
  });

  return (
    <pre class="bg-gray- p-4 rounded-md overflow-auto">
      <code ref={codeElement} class="language-bash">{props.script}</code>
    </pre>
  );
}
