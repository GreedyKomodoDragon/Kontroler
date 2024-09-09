import { Component, createSignal } from "solid-js";
import Identicon from "./navbar/icon";

const Header: Component = () => {
  const [state, setState] = createSignal<boolean>(false);

  const navigation = [
    { title: "Log out", path: "/logout" },
  ];

  return (
    <div class="flex items-center h-16 px-4 border-b border-gray-800">
      <a class="flex items-center gap-2" href="/" rel="ugc">
        <img src="/src/assets/logo.svg" width={100} height={100} />
        <span class="font-semibold text-lg">Kontroler</span>
      </a>
      <div class="flex items-center ml-auto gap-4">
        <div class="flex items-center space-x-4">
          <button
            onClick={() => {
              setState(!state());
            }}
          >
            <Identicon value="administrator" size={50} />
          </button>
        </div>
        <ul
          class={`bg-white top-14 right-6 mt-2 absolute border rounded-md text-sm w-36 shadow-md space-y-0 ${
            state() ? "" : "hidden"
          }`}
        >
          {navigation.map((item, idx) => (
            <li>
              <a
                class="block text-gray-600 hover:bg-gray-50 p-2.5"
                href={item.path}
              >
                {item.title}
              </a>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
};

export default Header;
