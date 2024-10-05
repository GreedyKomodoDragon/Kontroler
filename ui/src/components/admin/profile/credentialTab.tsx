import { createSignal } from "solid-js";
import ErrorSingleAlert from "../../alerts/errorSingleAlert";
import SuccessfulAlert from "../../successfulAlert";
import Spinner from "../../spinner";
import { updatePassword } from "../../../api/admin";

export default function CredentialsTab() {
  const [oldPassword, setOldPassword] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [confirmPassword, setConfirmPassword] = createSignal("");
  const [error, setError] = createSignal("");
  const [success, setSuccess] = createSignal("");

  const [loading, setLoading] = createSignal(false);

  const handleSubmit = (e: Event) => {
    e.preventDefault();

    setError("");
    setSuccess("");

    if (!oldPassword() || !password() || !confirmPassword()) {
      setError("All fields are required.");
      return;
    }
    if (password() !== confirmPassword()) {
      setError("New password and confirm password do not match.");
      return;
    }

    setLoading(true);
    updatePassword(oldPassword(), password())
      .then(() => {
        setSuccess("Password changed successfully!");
      })
      .catch(() => {
        setError("Unable to update password");
      })
      .finally(() => {
        setLoading(false);
      });
  };

  return (
    <div class="rounded-lg border bg-white text-gray-900 shadow-sm">
      <div class="flex flex-col space-y-1.5 p-6">
        <h3 class="text-2xl font-semibold leading-none tracking-tight">
          Change Password
        </h3>
        <p class="text-sm text-gray-500">
          Update your password here. Please make sure it's secure.
        </p>
      </div>
      <div class="p-6 pt-0">
        {!loading() ? (
          <form class="space-y-4" onSubmit={handleSubmit}>
            {error() && <ErrorSingleAlert msg={error()} />}
            {success() && <SuccessfulAlert msg={success()} />}
            <div class="space-y-2">
              <label
                for="current-password"
                class="text-sm font-medium leading-none"
              >
                Current Password
              </label>
              <div class="relative">
                <input
                  type="password"
                  id="current-password"
                  class="flex h-10 w-full rounded-md border bg-gray-100 px-3 py-2 text-sm ring-offset-background placeholder-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                  required
                  onInput={(e) => setOldPassword(e.currentTarget.value)}
                  value={oldPassword()}
                />
                {/* Add show/hide password logic here if needed */}
              </div>
            </div>

            <div class="space-y-2">
              <label
                for="new-password"
                class="text-sm font-medium leading-none"
              >
                New Password
              </label>
              <input
                type="password"
                id="new-password"
                class="flex h-10 w-full rounded-md border bg-gray-100 px-3 py-2 text-sm ring-offset-background placeholder-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                required
                onInput={(e) => setPassword(e.currentTarget.value)}
                value={password()}
              />
            </div>

            <div class="space-y-2">
              <label
                for="confirm-password"
                class="text-sm font-medium leading-none"
              >
                Confirm New Password
              </label>
              <input
                type="password"
                id="confirm-password"
                class="flex h-10 w-full rounded-md border bg-gray-100 px-3 py-2 text-sm ring-offset-background placeholder-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                required
                onInput={(e) => setConfirmPassword(e.currentTarget.value)}
                value={confirmPassword()}
              />
            </div>

            <div class="flex items-center pt-6">
              <button
                type="submit"
                class="inline-flex items-center justify-center rounded-md text-sm font-medium bg-blue-500 text-white h-10 px-4 py-2 hover:bg-blue-600 transition-colors"
              >
                Change Password
              </button>
            </div>
          </form>
        ) : (
          <Spinner width={100} height={100} />
        )}
      </div>
    </div>
  );
}
