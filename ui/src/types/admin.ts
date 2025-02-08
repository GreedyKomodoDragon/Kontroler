export type User = {
  username: string;
  role: string;
};

export type Role = "admin" | "editor" | "viewer";

export const roleDescriptions: Record<Role, string[]> = {
  admin: [
    "Admins have full access",
    "Can create & delete users",
    "Plus all Editor permissions",
  ],
  editor: ["Editors can create Dags & DagRuns", "Can view all content"],
  viewer: ["Viewers can only read content", "Cannot make any modifications"],
};
