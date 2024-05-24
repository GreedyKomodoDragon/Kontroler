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
        href="/cronjobs"
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
            <circle cx="12" cy="12" r="10"></circle>
            <polyline points="12 6 12 12 16 14"></polyline>
          </svg>
          <span class="text-sm font-medium">CronJobs</span>
        </div>
      </a>
      <a
        class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-gray-800 transition-colors"
        href="/runs"
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
          <span class="text-sm font-medium">Runs</span>
        </div>
      </a>
      <a
        class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-gray-800 transition-colors"
        href="/crds"
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
            <path d="m21.12 6.4-6.05-4.06a2 2 0 0 0-2.17-.05L2.95 8.41a2 2 0 0 0-.95 1.7v5.82a2 2 0 0 0 .88 1.66l6.05 4.07a2 2 0 0 0 2.17.05l9.95-6.12a2 2 0 0 0 .95-1.7V8.06a2 2 0 0 0-.88-1.66Z"></path>
            <path d="M10 22v-8L2.25 9.15"></path>
            <path d="m10 14 11.77-6.87"></path>
          </svg>
          <span class="text-sm font-medium">CRDs</span>
        </div>
      </a>
    </nav>
  );
};

export default Sidebar;
