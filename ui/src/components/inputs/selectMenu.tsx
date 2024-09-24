import { DropdownMenu } from "@kobalte/core/dropdown-menu";
import { For } from "solid-js";

type SelectMenuProps = {
  selectedValue: string;
  setValue: (value: string) => void;
  search: (term: string) => void;
  items: string[];
};

export default function SelectMenu(props: SelectMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenu.Trigger>
        <button class="flex items-center justify-between w-60 px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md shadow hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-opacity-50 transition duration-200">
          {props.selectedValue === ""
            ? "Select a Dag"
            : `Selected: ${props.selectedValue}`}
          <svg
            class="w-4 h-4 ml-2"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              fill-rule="evenodd"
              d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
              clip-rule="evenodd"
            />
          </svg>
        </button>
      </DropdownMenu.Trigger>

      <DropdownMenu.Content class="mt-2 bg-white rounded-lg shadow-lg w-60 border border-gray-200">
        <div class="px-3 py-2 border-gray-100">
          <input
            type="text"
            placeholder="Search..."
            class="w-full px-3 py-2 text-sm text-gray-700 placeholder-gray-400 bg-gray-50 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-blue-400"
            oninput={(e) => {
              props.search(e.currentTarget.value);
            }}
          />
        </div>
        <div class="max-h-60 overflow-y-auto">
          <For each={props.items}>
            {(item) => (
              <DropdownMenu.Item
                class="flex items-center px-4 py-2 text-sm rounded-b-lg text-gray-700 cursor-pointer hover:bg-blue-50 hover:text-blue-700 focus:bg-blue-100 focus:outline-none"
                onSelect={() => props.setValue(item)}
              >
                {item}
              </DropdownMenu.Item>
            )}
          </For>
        </div>
      </DropdownMenu.Content>
    </DropdownMenu>
  );
}
