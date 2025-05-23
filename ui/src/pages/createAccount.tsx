import { createSignal } from "solid-js";
import { createAccount } from "../api/admin";
import { Role, roleDescriptions } from "../types/admin";
import { useError } from "../providers/ErrorProvider";

export default function CreateAccountPage() {
  const { handleApiError } = useError();

  const [errorMsgs, setErrorMsgs] = createSignal<string[]>([]);
  const [successMsg, setSuccessMsg] = createSignal<string>("");

  const [username, setUsername] = createSignal<string>("");
  const [password, setPassword] = createSignal<string>("");
  const [passwordConfirm, setPasswordConfirm] = createSignal<string>("");
  const [role, setRole] = createSignal<string>("");

  const onSubmit = () => {
    setErrorMsgs([]);

    let errors: string[] = [];
    if (username() === "") {
      errors.push("Username cannot be empty");
    }

    if (password() === "") {
      errors.push("Password cannot be empty");
    }

    if (password() !== passwordConfirm()) {
      errors.push("Passwords must match");
    }

    if (["admin", "editor", "viewer"].indexOf(role()) === -1) {
      errors.push("Role must be either Admin, Editor, or Viewer");
    }

    if (errors.length !== 0) {
      setErrorMsgs(errors);
      return;
    }

    createAccount(username(), password(), role())
      .then(() => {
        setSuccessMsg("Account has been successfully created!");
      })
      .catch((error) => handleApiError(error));
  };

  return (
    <>
      <main class="w-full flex flex-col items-center justify-center px-4">
        <h2 class="text-4xl font-semibold mb-4">Create a new Account</h2>
        <div class="max-w-sm w-full space-y-5">
          <div>
            <label class="block text-lg font-medium my-4">Username</label>
            <input
              type="text"
              class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-700 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              required
              onChange={(event) => {
                setUsername(event.currentTarget.value);
              }}
            />
          </div>
          <div>
            <label class="block text-lg font-medium my-4">Password</label>
            <input
              type="password"
              class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-700 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              required
              onChange={(event) => {
                setPassword(event.currentTarget.value);
              }}
            />
          </div>
          <div>
            <label class="block text-lg font-medium my-4">Confirm Password</label>
            <input
              type="password"
              class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-700 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              required
              onChange={(event) => {
                setPasswordConfirm(event.currentTarget.value);
              }}
            />
          </div>
          <div>
            <label class="block text-lg font-medium my-4">Role</label>
            <select
              class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-700 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              required
              onChange={(event) => {
                setRole(event.currentTarget.value.toLowerCase());
              }}
            >
              <option value="">Select a role</option>
              <option value="admin">Admin</option>
              <option value="editor">Editor</option>
              <option value="viewer">Viewer</option>
            </select>
            {role() && <h4 class="mt-2">Permission:</h4>}
            {role() &&
              roleDescriptions[role() as Role]?.map((desc: string, index: number) => (
                <li class="flex items-center space-x-3 text-white p-2 rounded-md transition-colors">
                  <svg
                    class="w-5 h-5 text-green-500"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                  <span>{desc}</span>
                </li>
              ))}
          </div>

          <div class="flex justify-center">
            <button
              type="submit"
              class="mt-2 px-6 py-3 bg-indigo-600 text-white rounded-md shadow hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
              onclick={onSubmit}
            >
              Create Account
            </button>
          </div>
        </div>
      </main>
      {errorMsgs().length !== 0 && (
        <div class="mt-4 mx-auto max-w-sm">
          <div class="bg-red-50 border border-red-500 text-red-600 p-4 rounded-md">
            <h3 class="font-semibold">Errors:</h3>
            <ul class="list-disc list-inside">
              {errorMsgs().map((msg) => (
                <li>{msg}</li>
              ))}
            </ul>
          </div>
        </div>
      )}
      {successMsg() && (
        <div class="mt-6 mx-auto max-w-sm">
          <div class="bg-green-50 border border-green-500 text-green-600 p-4 rounded-md">
            <h3 class="font-semibold">Success:</h3>
            <p>{successMsg()}</p>
          </div>
        </div>
      )}
    </>
  );
}
