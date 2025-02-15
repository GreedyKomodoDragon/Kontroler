import { highlightAllUnder } from "prismjs";
import { createEffect, createMemo } from "solid-js";
import "prismjs/themes/prism.css";

type LogHighlighterProps = {
  logs: string;
};

export default function LogHighlighter(props: LogHighlighterProps) {
  let codeElement: HTMLPreElement | undefined;

  // Create memoized values for log processing
  const logLines = createMemo(() => props.logs.split("\n"));
  const maxDigits = createMemo(() => String(logLines().length - 1).length);


  createEffect(() => {
    if (codeElement) {
      highlightAllUnder(codeElement);
    }
  });

  const getLogClass = (log: string) => {
    if (log.includes("ERROR")) return "language-error";
    if (log.includes("WARN")) return "language-warning";
    return "language-info";
  };

  return (
    <pre ref={codeElement} class="p-4 rounded-md overflow-auto">
      {logLines().map((log, i) => (
        <>
          <code class={getLogClass(log)}>
            {String(i).padStart(maxDigits(), " ")}: {log}
          </code>
          <br />
        </>
      ))}
    </pre>
  );
}
