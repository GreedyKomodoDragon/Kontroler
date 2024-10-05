export default function CredentialsTab() {
    return (
      <div class="rounded-lg border bg-white text-gray-900 shadow-sm">
        <div class="flex flex-col space-y-1.5 p-6">
          <h3 class="text-2xl font-semibold leading-none tracking-tight">
            Change Password
          </h3>
          <p class="text-sm text-gray-500">Update your password here. Please make sure it's secure.</p>
        </div>
        <div class="p-6 pt-0">
          <form class="space-y-4">
            <div class="space-y-2">
              <label for="current-password" class="text-sm font-medium leading-none">Current Password</label>
              <div class="relative">
                <input
                  type="password"
                  id="current-password"
                  class="flex h-10 w-full rounded-md border bg-gray-100 px-3 py-2 text-sm ring-offset-background placeholder-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                  required
                />
                <button
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 inline-flex items-center justify-center rounded-md text-sm font-medium h-10 w-10 text-gray-500"
                >
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
                    class="h-4 w-4"
                  >
                    <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"></path>
                    <circle cx="12" cy="12" r="3"></circle>
                  </svg>
                  <span class="sr-only">Show password</span>
                </button>
              </div>
            </div>
  
            <div class="space-y-2">
              <label for="new-password" class="text-sm font-medium leading-none">New Password</label>
              <input
                type="password"
                id="new-password"
                class="flex h-10 w-full rounded-md border bg-gray-100 px-3 py-2 text-sm ring-offset-background placeholder-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                required
              />
            </div>
  
            <div class="space-y-2">
              <label for="confirm-password" class="text-sm font-medium leading-none">Confirm New Password</label>
              <input
                type="password"
                id="confirm-password"
                class="flex h-10 w-full rounded-md border bg-gray-100 px-3 py-2 text-sm ring-offset-background placeholder-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                required
              />
            </div>
          </form>
        </div>
        <div class="flex items-center p-6 pt-0">
          <button
            type="submit"
            class="inline-flex items-center justify-center rounded-md text-sm font-medium bg-blue-500 text-white h-10 px-4 py-2 hover:bg-blue-600 transition-colors"
          >
            Change Password
          </button>
        </div>
      </div>
    );
  }
  