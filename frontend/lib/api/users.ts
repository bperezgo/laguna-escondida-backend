import { apiRequest } from "./config";
import type {
  Role,
  UserWithRoles,
  CreateUserRequest,
  UpdateUserRequest,
  UsersListResponse,
  RolesListResponse,
} from "@/types/user";

export const usersApi = {
  /**
   * Get all users with their roles
   */
  async getAll(): Promise<UserWithRoles[]> {
    const response = await apiRequest<UsersListResponse>("/admin/users");
    return response.users || [];
  },

  /**
   * Get a single user with roles by ID
   */
  async getById(id: string): Promise<UserWithRoles> {
    return apiRequest<UserWithRoles>(`/admin/users/${id}`);
  },

  /**
   * Create a new user (admin defines the password)
   */
  async create(data: CreateUserRequest): Promise<UserWithRoles> {
    return apiRequest<UserWithRoles>("/admin/users", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  /**
   * Update a user's name, roles and/or active state
   */
  async update(id: string, data: UpdateUserRequest): Promise<UserWithRoles> {
    return apiRequest<UserWithRoles>(`/admin/users/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  /**
   * Reset a user's password to an admin-defined value
   */
  async resetPassword(id: string, password: string): Promise<void> {
    await apiRequest(`/admin/users/${id}/reset-password`, {
      method: "POST",
      body: JSON.stringify({ password }),
    });
  },

  /**
   * Soft-delete a user
   */
  async remove(id: string): Promise<void> {
    await apiRequest(`/admin/users/${id}`, {
      method: "DELETE",
    });
  },
};

export const rolesApi = {
  /**
   * Get all assignable roles
   */
  async getAll(): Promise<Role[]> {
    const response = await apiRequest<RolesListResponse>("/admin/roles");
    return response.roles || [];
  },
};
