import { createSignal, Show } from "solid-js";
import { deleteAccount, getUserPageCount, getUsers } from "../api/admin";
import { A } from "@solidjs/router";
import PaginationComponent from "./pagination";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import Spinner from "./spinner";
import { DeleteButton } from "./admin/deleteButton";
import ConfirmDeletion from "./admin/confirmDeletion";
import ErrorSingleAlert from "./alerts/errorSingleAlert";

export default function ManageUsers() {
  const queryClient = useQueryClient();

  const [maxPage, setMaxPage] = createSignal(-1);
  const [page, setPage] = createSignal(1);
  const [show, setShow] = createSignal(false);
  const [selectedName, setSelectedName] = createSignal("");

  const [errorMsg, setErrorMsg] = createSignal("");

  const users = createQuery(() => ({
    queryKey: ["users", page().toString()],
    queryFn: getUsers,
    staleTime: 5 * 60 * 1000, // 5 minutes
  }));

  getUserPageCount()
    .then((count) => {
      setMaxPage(count);
    })
    .catch((error) => console.error(error));

  return (
    <div class="mx-auto px-4">
      <ConfirmDeletion
        onConfirm={() => {
          setShow(false);
          deleteAccount(selectedName())
            .then(() => {
              queryClient.invalidateQueries({
                queryKey: ["users", page().toString()],
              });
            })
            .catch(() => {
              setErrorMsg("failed to delete user account");
              setTimeout(() => {
                setErrorMsg("");
              }, 5 * 1000);
            });
        }}
        onCancel={() => {
          setShow(false);
        }}
        show={show()}
      />
      <div class="items-start justify-between sm:flex">
        <div>
          <h4 class="text-xl font-semibold">Team members</h4>
          <p class="mt-2 text-white text-base sm:text-sm">
            Control who has access to Kontroler
          </p>
        </div>

        <A
          href="/admin/account/create"
          class="inline-flex items-center justify-center gap-1 py-2 px-3 mt-2 font-medium text-sm text-center text-white bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-700 rounded-lg sm:mt-0"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width={1.5}
            stroke="currentColor"
            class="w-6 h-6"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M12 6v12m6-6H6"
            />
          </svg>
          New member
        </A>
      </div>
      {errorMsg().length !== 0 && (
        <div class="my-4 w-full">
          <ErrorSingleAlert msg={errorMsg()} />
        </div>
      )}
      <Show when={users.isError}>
        <div>Error: {users.error && users.error.message}</div>
      </Show>
      <Show when={users.isLoading}>
        <Spinner />
      </Show>
      <Show when={users.isSuccess}>
        <ul class="mt-12 divide-y">
          {users.data &&
            users.data.map((item, idx) => (
              <li class="py-5 flex items-start justify-between">
                <div class="flex gap-3">
                  <img
                    src="https://randomuser.me/api/portraits/men/86.jpg"
                    class="flex-none w-12 h-12 rounded-full"
                  />
                  <div>
                    <div>
                      <span class="block text-lg font-semibold">
                        {item.username}
                      </span>
                      <span class="block text-sm ">{item.role}</span>
                    </div>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    class="text-gray-700 text-sm border rounded-lg px-3 py-2 duration-150 bg-white"
                    disabled
                  >
                    Manage
                  </button>
                  <DeleteButton
                    delete={() => {
                      setSelectedName(item.username);
                      setShow(true);
                    }}
                  />
                </div>
              </li>
            ))}
        </ul>
      </Show>
      <Show when={maxPage() > 1}>
        <PaginationComponent setPage={setPage} maxPage={maxPage} />
      </Show>
    </div>
  );
}
