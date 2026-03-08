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

export interface AnalysisResult {
  suggestions: Suggestion[];
  dockerfile: string;
  port: number;
  stack: string;
  envHints: EnvHint[];
  diagnosis?: string;
}

export interface AISettings {
  anthropicApiKey: string;
  claudeClientId: string;
  claudeClientSecret: string;
  aiEnabled: boolean;
  aiModel: string;
}

export const analyzeApi = {
  async analyze(appId: string): Promise<AnalysisResult> {
    const { data } = await api.post<AnalysisResult>(`/apps/${appId}/analyze`);
    return data;
  },
  async analyzeFailure(appId: string): Promise<AnalysisResult> {
    const { data } = await api.post<AnalysisResult>(`/apps/${appId}/analyze-failure`);
    return data;
  },
  async saveDockerfile(appId: string, content: string): Promise<void> {
    await api.put(`/apps/${appId}/dockerfile`, { content });
  },
  async deleteDockerfile(appId: string): Promise<void> {
    await api.delete(`/apps/${appId}/dockerfile`);
  },
};

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
  async testConnection(apiKey?: string, model?: string): Promise<AITestResult> {
    const { data } = await api.post<AITestResult>('/admin/settings/ai/test', { apiKey, model });
    return data;
  },
};
