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

  return (
    <pre ref={codeElement} class="p-4 rounded-md overflow-auto">
      {props.logs.split("\n").map((log) => (
        <>
          <code class={getLogClass(log)}>{log}</code>
          <br/>
        </>
      ))}
    </pre>
  );
}
