import { createSignal } from "solid-js";
import ErrorAlert from "../components/errorAlert";
import SuccessfulAlert from "../components/successfulAlert";
import { createAccount } from "../api/admin";

export default function CreateAccountPage() {
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
      errors.push("username cannot be empty");
    }

    if (password() === "") {
      errors.push("password cannot be empty");
    }

    if (password() !== passwordConfirm()) {
      errors.push("passwords must match");
    }

    if (["admin", "editor", "viewer"].indexOf(role()) === -1) {
      errors.push("role must be either Admin, Editor, or Viewer");
    }

    if (errors.length != 0) {
      setErrorMsgs(errors);
      return;
    }

    createAccount(username(), password(), role())
      .then(() => {
        setSuccessMsg("Account has been successfully created!");
      })
      .catch((e) => {
        setErrorMsgs([e.message]);
      });
  };

  return (
    <>
      <main class="w-full flex flex-col items-center justify-center px-4">
        <h2 class="text-4xl font-semibold mb-4">Create a new Account</h2>
        <div class="max-w-sm w-full  space-y-5">
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
            <label class="block text-lg font-medium my-4">
              Confirm Password
            </label>
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
            <label class="block text-lg font-medium my-4">
              Role
            </label>
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
        <div class="mt-2 mx-10">
          <ErrorAlert msgs={errorMsgs()} />
        </div>
      )}
      {successMsg() && (
        <div class="mt-6 mx-10">
          <SuccessfulAlert msg={successMsg()} />
        </div>
      )}
    </>
  );
}
