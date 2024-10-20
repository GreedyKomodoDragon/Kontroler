import { highlightElement } from "prismjs";
import { createEffect } from "solid-js";
import yaml from "js-yaml";
import "prismjs/themes/prism.css";
import "prismjs/components/prism-yaml";

type JsonToYamlViewerProps = {
  json: string;
};

function JSONToYaml(json: string) {
  try {
    return yaml.dump(JSON.parse(json));
  } catch (e) {
    return "Invalid JSON";
  }
}

export default function JsonToYamlViewer(props: JsonToYamlViewerProps) {
  let codeElement: HTMLPreElement | undefined;

  createEffect(() => {
    if (codeElement) {
      codeElement.textContent = JSONToYaml(props.json);
      highlightElement(codeElement);
    }
  });

  return (
    <pre class="p-4 rounded-md overflow-auto">
      <code ref={codeElement} class="language-yaml">
        {JSONToYaml(props.json)}
      </code>
    </pre>
  );
}
