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
    </nav>
  );
};

export default Sidebar;
