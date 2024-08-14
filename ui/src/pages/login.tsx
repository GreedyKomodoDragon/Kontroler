export default function Login() {
  return (
    <main class="w-full h-screen flex flex-col items-center justify-center px-4">
      <div class="max-w-sm w-full text-gray-600 space-y-5">
        <div class="text-center pb-8">
          <img src="" width="150" class="mx-auto" />
        </div>
        <form class="space-y-5">
          <div>
            <label class="font-medium"> Email </label>
            <input
              type="email"
              required
              class="w-full mt-2 px-3 py-2 text-gray-500 bg-transparent outline-none border focus:border-indigo-600 shadow-sm rounded-lg"
            />
          </div>
          <div>
            <label class="font-medium"> Password </label>
            <input
              type="password"
              required
              class="w-full mt-2 px-3 py-2 text-gray-500 bg-transparent outline-none border focus:border-indigo-600 shadow-sm rounded-lg"
            />
          </div>
          <div class="flex items-center justify-between text-sm">
            <a
              href="javascript:void(0)"
              class="text-center text-indigo-600 hover:text-indigo-500"
            >
              Forgot password?
            </a>
          </div>
          <button class="w-full px-4 py-2 text-white font-medium bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-600 rounded-lg duration-150">
            Sign in
          </button>
        </form>
        <p class="text-center">
          Don't have an account?
          <a
            href="javascript:void(0)"
            class="font-medium text-indigo-600 hover:text-indigo-500 ml-2"
          >
            Sign up
          </a>
        </p>
      </div>
    </main>
  );
}