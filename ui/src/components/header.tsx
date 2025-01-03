import { Component, createSignal } from "solid-js";
import Identicon from "./navbar/icon";
import { useAuth } from "../providers/authProvider";

const Header: Component = () => {
  const [state, setState] = createSignal<boolean>(false);
  const auth = useAuth();

  const navigation = [
    { title: "Profile", path: "/account/profile" },
    { title: "Log out", path: "/logout" },
  ];

  return (
    <div class="flex items-center h-16 px-4 border-b border-gray-800 ">
      <a class="flex items-center gap-2" href="/" rel="ugc">
        <img src="/logo.svg" width={45} height={45} />
        <span class="font-semibold text-lg">Kontroler</span>
      </a>
      <div class="flex items-center ml-auto gap-4">
        <div class="flex items-center space-x-4">
          <button
            onClick={() => {
              setState(!state());
            }}
          >
            {auth.isAuthenticated() && auth.username() && (
              <Identicon value={auth.username()} size={50} />
            )}
          </button>
        </div>
        <ul
          class={`bg-gray-800 top-14 right-6 mt-2 absolute border border-gray-700 rounded-md text-sm w-36 shadow-lg space-y-0 ${
            state() ? "" : "hidden"
          }`}
        >
          {navigation.map((item) => (
            <li>
              <a
                class="block text-gray-300 hover:bg-gray-700 p-2.5 rounded-md transition-colors duration-200"
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
