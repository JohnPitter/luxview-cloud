import { api } from './client';

export interface Plan {
  id: string;
  name: string;
  description: string;
  price: number;
  currency: string;
  billingCycle: string;
  maxApps: number;
  maxCpuPerApp: number;
  maxMemoryPerApp: string;
  maxDiskPerApp: string;
  maxServicesPerApp: number;
  autoDeployEnabled: boolean;
  customDomainEnabled: boolean;
  priorityBuilds: boolean;
  highlighted: boolean;
  sortOrder: number;
  features: string[];
  isActive: boolean;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

export type CreatePlanPayload = Omit<Plan, 'id' | 'isActive' | 'isDefault' | 'createdAt' | 'updatedAt'>;
export type UpdatePlanPayload = Partial<CreatePlanPayload & { isActive: boolean; isDefault: boolean }>;

export const plansApi = {
  async listActive(): Promise<Plan[]> {
    const { data } = await api.get<Plan[]>('/plans');
    return data;
  },

  async listAll(): Promise<Plan[]> {
    const { data } = await api.get<Plan[]>('/admin/plans');
    return data;
  },

  async create(payload: CreatePlanPayload): Promise<Plan> {
    const { data } = await api.post<Plan>('/admin/plans', payload);
    return data;
  },

  async update(id: string, payload: UpdatePlanPayload): Promise<Plan> {
    const { data } = await api.patch<Plan>(`/admin/plans/${id}`, payload);
    return data;
  },

  async delete(id: string): Promise<void> {
    await api.delete(`/admin/plans/${id}`);
  },

  async setDefault(id: string): Promise<void> {
    await api.patch(`/admin/plans/${id}/default`);
  },

  async assignUserPlan(userId: string, planId: string): Promise<void> {
    await api.patch(`/admin/users/${userId}/plan`, { planId });
  },
};
