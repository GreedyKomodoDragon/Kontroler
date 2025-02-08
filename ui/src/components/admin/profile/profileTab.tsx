import { useAuth } from "../../../providers/authProvider";

// Define valid role types
type Role = "admin" | "editor" | "viewer";

export default function ProfileTab() {
  const auth = useAuth();

  const roleDescriptions: Record<Role, string[]> = {
    admin: [
      "Admins have full access.",
      "Can manage users.",
      "Can edit content.",
      "Can configure settings.",
    ],
    editor: [
      "Editors can modify content.",
      "Do not have access to user management.",
      "Cannot change system settings.",
    ],
    viewer: [
      "Viewers can only read content.",
      "Cannot make any modifications.",
    ],
  };

  return (
    <div class="rounded-lg border bg-white text-gray-900 shadow-sm p-8">
      <div class="space-y-6">
        {/* User Info Section */}
        <div class="pb-6 border-b border-gray-200">
          <h3 class="text-3xl font-bold text-gray-900 mb-2">
            {auth.username()}
          </h3>
          <div class="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-blue-100 text-blue-800">
            {auth.role()}
          </div>
        </div>

        {/* Permissions Section */}
        <div>
          <h4 class="text-xl font-semibold text-gray-900 mb-4">
            Your Permissions
          </h4>
          <ul class="space-y-3">
            {auth.role() && roleDescriptions[auth.role() as Role].map((desc: string, index: number) => (
              <li
                class="flex items-center space-x-3 text-gray-700 hover:bg-gray-50 p-2 rounded-md transition-colors"
              >
                {/* green tick */}
                <svg class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <span>{desc}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  );
}