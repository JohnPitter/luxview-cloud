import { api } from './client';

export interface Suggestion {
  type: 'error' | 'warning' | 'info';
  message: string;
}

export interface EnvHint {
  key: string;
  description: string;
  required: boolean;
}

export interface ServiceRecommendation {
  currentService: string;
  currentEvidence: string;
  recommendedService: 'postgres' | 'redis' | 'mongodb' | 'rabbitmq' | 's3';
  reason: string;
  manualSteps: string[];
  codeChanges?: CodeChange[];
}

export interface CodeChange {
  file: string;
  action: 'modify' | 'create' | 'delete';
  description: string;
  content: string;
}

export interface AnalysisResult {
  suggestions: Suggestion[];
  dockerfile: string;
  port: number;
  stack: string;
  envHints: EnvHint[];
  diagnosis?: string;
  serviceRecommendations?: ServiceRecommendation[];
}

export interface AISettings {
  apiKey: string;
  aiEnabled: boolean;
  aiModel: string;
}

export const analyzeApi = {
  async analyze(appId: string): Promise<AnalysisResult> {
    const { data } = await api.post<AnalysisResult>(`/apps/${appId}/analyze`, null, { timeout: 300000 });
    return data;
  },
  async analyzeFailure(appId: string): Promise<AnalysisResult> {
    const { data } = await api.post<AnalysisResult>(`/apps/${appId}/analyze-failure`, null, { timeout: 300000 });
    return data;
  },
  async saveDockerfile(appId: string, content: string): Promise<void> {
    await api.put(`/apps/${appId}/dockerfile`, { content });
  },
  async deleteDockerfile(appId: string): Promise<void> {
    await api.delete(`/apps/${appId}/dockerfile`);
  },
  async autoMigrate(appId: string, serviceType: string): Promise<AutoMigrateResult> {
    const { data } = await api.post<AutoMigrateResult>(`/apps/${appId}/auto-migrate`, { serviceType }, { timeout: 180000 });
    return data;
  },
};

export interface AutoMigrateResult {
  serviceId: string;
  prUrl?: string;
  message: string;
}

export interface AITestResult {
  success: boolean;
  model?: string;
  error?: string;
}

export const aiSettingsApi = {
  async get(): Promise<AISettings> {
    const { data } = await api.get<AISettings>('/admin/settings/ai');
    return data;
  },
  async update(settings: Partial<AISettings>): Promise<void> {
    await api.put('/admin/settings/ai', settings);
  },
  async testConnection(params: {
    apiKey?: string;
    model?: string;
  }): Promise<AITestResult> {
    const { data } = await api.post<AITestResult>('/admin/settings/ai/test', params);
    return data;
  },
};
