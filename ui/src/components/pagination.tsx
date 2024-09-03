import {
  createPagination,
  PaginationProps,
} from "@solid-primitives/pagination";
import { Accessor, createEffect, For, Setter } from "solid-js";

type PaginationComponentProps = {
  setPage: Setter<number>;
  maxPage: Accessor<number>;
};

export default function PaginationComponent(props: PaginationComponentProps) {
  const [paginationProps, page] = createPagination({ pages: props.maxPage() });

  createEffect(() => {
    props.setPage(page());
  });

  return (
    <div class="box-border flex flex-col items-center justify-center space-y-2  p-8 text-white ">
      <nav class="flex space-x-2 mt-4">
        <For each={paginationProps()}>
          {(props) => (
            <button
              {...props}
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              {props.children}
            </button>
          )}
        </For>
      </nav>
      <p class="text-sm text-gray-400">
        Current page: {page()} / {props.maxPage()}
      </p>
    </div>
  );
}
