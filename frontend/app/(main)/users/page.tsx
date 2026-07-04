import { serverApiRequest } from "@/lib/api/server";
import type {
  Role,
  UserWithRoles,
  UsersListResponse,
  RolesListResponse,
} from "@/types/user";
import UsersPageClient from "@/components/users/UsersPageClient";

export default async function UsersPage() {
  let users: UserWithRoles[] = [];
  let roles: Role[] = [];

  try {
    const [usersRes, rolesRes] = await Promise.all([
      serverApiRequest<UsersListResponse>("/admin/users"),
      serverApiRequest<RolesListResponse>("/admin/roles"),
    ]);

    const usersData = await usersRes.json();
    const rolesData = await rolesRes.json();

    users = usersData.users || [];
    roles = rolesData.roles || [];
  } catch (error) {
    console.error("Error fetching users data:", error);
  }

  return <UsersPageClient initialUsers={users} initialRoles={roles} />;
}
