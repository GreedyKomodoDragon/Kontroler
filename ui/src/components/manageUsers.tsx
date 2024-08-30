import { createSignal } from "solid-js";
import { User } from "../types/admin";
import { getUsers } from "../api/admin";
import { A } from "@solidjs/router";

export default function ManageUsers() {
  const [users, setUsers] = createSignal<User[]>([]);

  getUsers(0)
    .then((data) => {
      setUsers(data);
    })
    .catch(() => {
      console.log("failed to get users");
    });

  return (
    <div class="mx-auto px-4">
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
      <ul class="mt-12 divide-y">
        {users().map((item, idx) => (
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
            <button
              class="text-gray-700 text-sm border rounded-lg px-3 py-2 duration-150 bg-white"
              disabled
            >
              Manage
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
