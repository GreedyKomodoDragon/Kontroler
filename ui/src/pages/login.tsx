import { useNavigate } from "@solidjs/router";
import { useAuth } from "../providers/authProvider";
import { createSignal } from "solid-js";

export default function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [error, setError] = createSignal<string | null>(null);

  const handleLogin = async (e: Event) => {
    e.preventDefault(); // Prevent default form submission behavior (page reload)
    try {
      const successful = await login(username(), password());
      if (successful) {
        navigate("/");
        return;
      }

      setError("Login failed. Please check your username and password.");
    } catch (err) {
      setError("Login failed. Please check your username and password.");
    }
  };

  return (
    <main class="w-full h-screen flex flex-col items-center justify-center px-4">
      <div class="max-w-sm w-full text-gray-600 space-y-5">
        <img
          src="/logo.svg"
          width="1500"
          style={{
            margin: "0px 0px -100px 0px",
          }}
        />
        <h1 class="text-4xl text-white text-center">Welcome to Kontroler</h1>
        <form onSubmit={handleLogin}>
          <div>
            <label class="font-medium my-2"> Username </label>
            <input
              type="text"
              required
              class="w-full mt-2 px-3 py-2 text-gray-500 bg-transparent outline-none border focus:border-indigo-600 shadow-sm rounded-lg"
              onChange={(e) => setUsername(e.currentTarget.value)}
            />
          </div>
          <div>
            <label class="font-medium my-2"> Password </label>
            <input
              type="password"
              required
              class="w-full mt-2 px-3 py-2 text-gray-500 bg-transparent outline-none border focus:border-indigo-600 shadow-sm rounded-lg"
              onChange={(e) => setPassword(e.currentTarget.value)}
            />
          </div>
          <div class="flex items-center justify-between text-sm my-2">
            <a
              href="javascript:void(0)"
              class="text-center text-indigo-600 hover:text-indigo-500"
            >
              Forgot password?
            </a>
          </div>
          <button
            type="submit"
            class="w-full px-4 py-2 text-white font-medium bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-600 rounded-lg duration-150"
          >
            Sign in
          </button>
        </form>
        {error() && <h2 class="text-red-700" >{error()}</h2>}
      </div>
    </main>
  );
}
