import { Component, createSignal } from "solid-js";

const Header: Component = () => {

  const [state, setState] = createSignal<boolean>(false);

  const navigation = [
    { title: "Dashboard", path: "javascript:void(0)" },
    { title: "Settings", path: "javascript:void(0)" },
    { title: "Log out", path: "javascript:void(0)" },
  ];

  return (
    <div class="flex items-center h-16 px-4 border-b border-gray-800">
      <a class="flex items-center gap-2" href="/" rel="ugc">
        <img src="/src/assets/logo.svg" width={100} height={100} />
        <span class="font-semibold text-lg">Kontroler</span>
      </a>
      <div class="flex items-center ml-auto gap-4">
        <div class="flex items-center space-x-4">
          <button class="w-10 h-10 outline-none rounded-full ring-offset-2 ring-gray-200 ring-2 "
          onClick={() => {
            setState(!state())
          }}>
            <img
              src="https://randomuser.me/api/portraits/men/46.jpg"
              class="w-full h-full rounded-full"
            />
          </button>
          <div class="lg:hidden">
            <span class="block">Micheal John</span>
            <span class="block text-sm text-gray-500">john@gmail.com</span>
          </div>
        </div>
        <ul
          class={`bg-white top-14 right-6 space-y-5 lg:absolute lg:border lg:rounded-md lg:text-sm lg:w-52 lg:shadow-md lg:space-y-0 lg:mt-0 ${
            state() ? "" : "lg:hidden"
          }`}
        >
          {navigation.map((item, idx) => (
            <li>
              <a
                class="block text-gray-600 lg:hover:bg-gray-50 lg:p-2.5"
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
