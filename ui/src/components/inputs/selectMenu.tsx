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
        <button class="px-4 py-2 text-white bg-blue-600 rounded-md shadow-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-400 focus:ring-opacity-50">
          {props.selectedValue === ""
            ? "Select Dag"
            : `Selected: ${props.selectedValue}`}
        </button>
      </DropdownMenu.Trigger>

      <DropdownMenu.Content class="mt-2 bg-white shadow-lg rounded-md w-48 border border-gray-200">
        <input
          type="text"
          placeholder="Search..."
          class="px-3 py-2 w-full border text-gray-700 border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-blue-400"
          oninput={(e) => {
            props.search(e.currentTarget.value);
          }}
        />
        <For each={props.items}>
          {(item, i) => (
            <DropdownMenu.Item
              class="px-4 py-2 hover:bg-blue-100 text-gray-700 cursor-pointer"
              onSelect={() => props.setValue(item)}
            >
              {item}
            </DropdownMenu.Item>
          )}
        </For>
      </DropdownMenu.Content>
    </DropdownMenu>
  );
}
