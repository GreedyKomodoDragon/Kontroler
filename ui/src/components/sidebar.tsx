import { Component } from "solid-js";

const Sidebar: Component = () => {
  return (
    <nav class="flex flex-col items-stretch justify-start w-48 border-r border-gray-800 py-4 overflow-y-auto">
      <a
        class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-gray-800 transition-colors"
        href="/"
        rel="ugc"
      >
        <div class="flex items-center gap-3">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            class="h-5 w-5"
          >
            <path d="M5 12s2.545-5 7-5c4.454 0 7 5 7 5s-2.546 5-7 5c-4.455 0-7-5-7-5z"></path>
            <path d="M12 13a1 1 0 1 0 0-2 1 1 0 0 0 0 2z"></path>
            <path d="M21 17v2a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-2"></path>
            <path d="M21 7V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v2"></path>
          </svg>
          <span class="text-sm font-medium">Overview</span>
        </div>
      </a>
      <a
        class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-gray-800 transition-colors"
        href="/create"
        rel="ugc"
      >
        <div class="flex items-center gap-3">
          <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            xmlns="http://www.w3.org/2000/svg"
          >
            <g
              fill="none"
              fill-rule="evenodd"
              stroke="currentColor"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M10 4.5H5.5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V11" />
              <path d="M17.5 3.467a1.46 1.46 0 0 1-.017 2.05L10.5 12.5l-3 1 1-3 6.987-7.046a1.41 1.41 0 0 1 1.885-.104zm-2 2.033.953 1" />
            </g>
          </svg>
          <span class="text-sm font-medium">Create</span>
        </div>
      </a>
      <a
        class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-gray-800 transition-colors"
        href="/dags"
        rel="ugc"
      >
        <div class="flex items-center gap-3">
          <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="None"
            xmlns="http://www.w3.org/2000/svg"
          >
            <circle cx="18" cy="5" r="3" stroke="white" stroke-width="2" />
            <circle cx="18" cy="19" r="3" stroke="white" stroke-width="2" />
            <circle cx="6" cy="12" r="3" stroke="white" stroke-width="2" />
            <path
              d="m15.408 6.512-6.814 3.975m6.814 7.001-6.814-3.975"
              stroke="white"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>

          <span class="text-sm font-medium">DAGs</span>
        </div>
      </a>
      <a
        class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-gray-800 transition-colors"
        href="/dags/runs"
        rel="ugc"
      >
        <div class="flex items-center gap-3">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            class="h-5 w-5"
          >
            <polygon points="6 3 20 12 6 21 6 3"></polygon>
          </svg>
          <span class="text-sm font-medium">DAG Runs</span>
        </div>
      </a>
      <a
        class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-gray-800 transition-colors"
        href="/admin"
        rel="ugc"
      >
        <div class="flex items-center gap-3">
          <svg
            width="24"
            height="24"
            viewBox="0 0 15 15"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              clip-rule="evenodd"
              d="m5.944.5-.086.437-.329 1.598a5.5 5.5 0 0 0-1.434.823L2.487 2.82l-.432-.133-.224.385L.724 4.923.5 5.31l.328.287 1.244 1.058c-.045.277-.103.55-.103.841s.058.565.103.842L.828 9.395.5 9.682l.224.386 1.107 1.85.224.387.432-.135 1.608-.537c.431.338.908.622 1.434.823l.329 1.598.086.437h3.111l.087-.437.328-1.598a5.5 5.5 0 0 0 1.434-.823l1.608.537.432.135.225-.386 1.106-1.851.225-.386-.329-.287-1.244-1.058c.046-.277.103-.55.103-.842 0-.29-.057-.564-.103-.841l1.244-1.058.329-.287-.225-.386-1.106-1.85-.225-.386-.432.134-1.608.537a5.5 5.5 0 0 0-1.434-.823L9.142.937 9.055.5z"
              stroke="white"
              stroke-linecap="square"
              stroke-linejoin="round"
            />
            <path
              clip-rule="evenodd"
              d="M9.5 7.495a2 2 0 0 1-4 0 2 2 0 0 1 4 0Z"
              stroke="white"
              stroke-linecap="square"
              stroke-linejoin="round"
            />
          </svg>
          <span class="text-sm font-medium">Administration</span>
        </div>
      </a>
    </nav>
  );
};

export default Sidebar;
