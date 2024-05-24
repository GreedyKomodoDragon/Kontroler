import { Component } from "solid-js";

const Header: Component = () => {
  return (
    <div class="flex items-center h-16 px-4 border-b border-gray-800">
      <a class="flex items-center gap-2" href="/" rel="ugc">
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
          class="h-6 w-6"
        >
          <path d="M22 7.7c0-.6-.4-1.2-.8-1.5l-6.3-3.9a1.72 1.72 0 0 0-1.7 0l-10.3 6c-.5.2-.9.8-.9 1.4v6.6c0 .5.4 1.2.8 1.5l6.3 3.9a1.72 1.72 0 0 0 1.7 0l10.3-6c.5-.3.9-1 .9-1.5Z"></path>
          <path d="M10 21.9V14L2.1 9.1"></path>
          <path d="m10 14 11.9-6.9"></path>
          <path d="M14 19.8v-8.1"></path>
          <path d="M18 17.5V9.4"></path>
        </svg>
        <span class="font-semibold text-lg">KubeConductor</span>
      </a>
      <div class="flex items-center ml-auto gap-4">
        <button
          class="inline-flex items-center justify-center whitespace-nowrap text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 hover:bg-accent hover:text-accent-foreground h-10 w-10 rounded-full"
          type="button"
          id="radix-:rg:"
          aria-haspopup="menu"
          aria-expanded="false"
          data-state="closed"
        >
          <img
            src="/placeholder.svg"
            class="rounded-full border"
            alt="Avatar"
            style="aspect-ratio: 32 / 32; object-fit: cover;"
            width="32"
            height="32"
          />
          <span class="sr-only">Toggle user menu</span>
        </button>
      </div>
    </div>
  );
};

export default Header;
