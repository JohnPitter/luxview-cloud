import { api } from './client';

export type AlertChannel = 'email' | 'webhook' | 'discord';

export interface Alert {
  id: string;
  appId: string;
  metric: string;
  condition: string;
  threshold: number;
  channel: AlertChannel;
  channelConfig: Record<string, string>;
  enabled: boolean;
  lastTriggered: string | null;
}

export interface CreateAlertPayload {
  metric: string;
  condition: string;
  threshold: number;
  channel: AlertChannel;
  channelConfig: Record<string, string>;
}

export const alertsApi = {
  async list(appId: string): Promise<Alert[]> {
    const { data } = await api.get<Alert[]>(`/apps/${appId}/alerts`);
    return data;
  },

  async create(appId: string, payload: CreateAlertPayload): Promise<Alert> {
    const { data } = await api.post<Alert>(`/apps/${appId}/alerts`, payload);
    return data;
  },

  async update(appId: string, alertId: string, payload: Partial<Alert>): Promise<Alert> {
    const { data } = await api.patch<Alert>(`/apps/${appId}/alerts/${alertId}`, payload);
    return data;
  },

  async delete(appId: string, alertId: string): Promise<void> {
    await api.delete(`/apps/${appId}/alerts/${alertId}`);
  },
};
