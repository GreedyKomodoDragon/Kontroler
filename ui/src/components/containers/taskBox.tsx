import { createSignal } from "solid-js";
import { TaskDetails } from "../../types/dag";
import JsonToYamlViewer from "../code/JsonToYamlViewer";
import ShellScriptViewer from "../code/shellScriptViewer";

type TaskBoxProps = {
  taskDetails: TaskDetails;
};

export default function TaskBox(props: TaskBoxProps) {
  const [open, setOpen] = createSignal<boolean>(false);

  const onClick = () => {
    setOpen(!open());
  };

  return (
    <div class="mt-8 p-6 bg-gray-700 rounded-lg shadow-inner">
      <div
        onClick={onClick}
        class="flex items-center justify-between cursor-pointer"
      >
        <h4 class="text-xl font-semibold text-gray-200">
          Name: {props.taskDetails.name}
        </h4>
        <svg
          class={`w-5 h-5 transition-transform ${
            open() ? "rotate-180" : "rotate-0"
          }`}
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width={2}
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </div>

      {open() && (
        <div class="space-y-3 mt-2">
          {props.taskDetails.command && (
            <p>
              <strong class="font-medium text-gray-300">Command:</strong>{" "}
              <span class="text-gray-400">
                {props.taskDetails.command!.join(" ")}
              </span>
            </p>
          )}
          {props.taskDetails.args && (
            <p>
              <strong class="font-medium text-gray-300">Args:</strong>{" "}
              <span class="text-gray-400">
                {props.taskDetails.args!.join(" ")}
              </span>
            </p>
          )}
          {props.taskDetails.script && (
            <div>
              <p class="my-2">
                <strong class="font-medium text-gray-300">Script:</strong>
              </p>
              <ShellScriptViewer script={props.taskDetails.script!} />
            </div>
          )}
          <p>
            <strong class="font-medium text-gray-300">Image:</strong>{" "}
            {props.taskDetails.image}
          </p>
          <p>
            <strong class="font-medium text-gray-300">Parameters:</strong>
          </p>
          <ul class="ml-4 list-disc text-gray-400">
            {props.taskDetails.parameters &&
              props.taskDetails.parameters.map((param) => (
                <li>
                  {param.name} - Default
                  {param.isSecret && " Secret"}:{" "}
                  {param.defaultValue ? param.defaultValue : "N/A"}
                </li>
              ))}
          </ul>
          <p>
            <strong class="font-medium text-gray-300">BackOff Limit:</strong>{" "}
            {props.taskDetails.backOffLimit}
          </p>
          <p>
            <strong class="font-medium text-gray-300">Conditional:</strong>{" "}
            {props.taskDetails.isConditional ? "Yes" : "No"}
          </p>
          <p>
            <strong class="font-medium text-gray-300">Retry Codes:</strong>{" "}
            {props.taskDetails.retryCodes}
          </p>
          {props.taskDetails.podTemplate && (
            <div>
              <p class="mb-2">
                <strong class="font-medium text-gray-300">Pod Template:</strong>
              </p>
              <JsonToYamlViewer json={props.taskDetails.podTemplate!} />
            </div>
          )}
        </div>
      )}
    </div>
  );
}
