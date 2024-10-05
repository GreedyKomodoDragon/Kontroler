import { createSignal } from "solid-js";
import CredentialsTab from "../components/admin/profile/credentialTab";
import ProfileTab from "../components/admin/profile/profileTab";
import Tabs from "../components/admin/profile/tabs";

export default function UserManagement() {
  const [activeTab, setActiveTab] = createSignal("profile");

  const tabs = [
    { id: "profile", label: "Profile" },
    { id: "credentials", label: "Credentials" },
  ];

  return (
    <div class="container mx-auto p-4">
      <h1 class="text-2xl font-bold mb-6">Profile Management</h1>
      <Tabs tabs={tabs} activeTab={activeTab} setActiveTab={setActiveTab} />

      {activeTab() === "profile" && (
        <div class="mt-4 p-4 transition-opacity duration-300">
          <ProfileTab />
        </div>
      )}

      {activeTab() === "credentials" && (
        <div class="mt-4 p-4 transition-opacity duration-300">
          <CredentialsTab />
        </div>
      )}
    </div>
  );
}
