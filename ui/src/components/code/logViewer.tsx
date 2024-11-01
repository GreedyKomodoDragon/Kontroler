import { highlightAllUnder } from "prismjs";
import { createEffect } from "solid-js";
import "prismjs/themes/prism.css";

type LogHighlighterProps = {
  logs: string;
};

export default function LogHighlighter(props: LogHighlighterProps) {
  let codeElement: HTMLPreElement | undefined;

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

  // Split the logs into lines and get the maximum number of lines
  const logLines = props.logs.split("\n");
  const maxDigits = String(logLines.length - 1).length; // Determine max digits for padding

  return (
    <pre ref={codeElement} class="p-4 rounded-md overflow-auto">
      {logLines.map((log, i) => (
        <>
          <code class={getLogClass(log)}>
            {String(i).padStart(maxDigits, " ")}: {log}
          </code>
          <br />
        </>
      ))}
    </pre>
  );
}
