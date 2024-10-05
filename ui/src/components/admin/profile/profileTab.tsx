import { useAuth } from "../../../providers/authProvider";

export default function ProfileTab() {
    const auth = useAuth()

    return (
      <div class="rounded-lg border bg-white text-gray-900 shadow-sm">
        <div class="flex flex-col space-y-1.5 p-6">
          <h3 class="text-2xl font-semibold leading-none tracking-tight">
            Profile: {auth.username()}
          </h3>
        </div>
        <div class="p-6 pt-0">
          <p>Permissions: Coming soon!</p>
        </div>
      </div>
    );
  }
  