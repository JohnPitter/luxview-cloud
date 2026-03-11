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
  recommendedService: 'postgres' | 'redis' | 'mongodb' | 'rabbitmq' | 'storage';
  reason: string;
  manualSteps: string[];
}

export interface AnalysisResult {
  suggestions: Suggestion[];
  dockerfile: string;
  port: number;
  stack: string;
  envHints: EnvHint[];
  diagnosis?: string;
  serviceRecommendations?: ServiceRecommendation[];
  requiresAi?: boolean;
}

export interface AISettings {
  apiKey: string;
  aiEnabled: boolean;
  aiModel: string;
}

export interface ApplyAnalysisRequest {
  dockerfile: string;
  envVars: Record<string, string>;
  services: string[];
}

export interface ApplyAnalysisResponse {
  message: string;
  provisionedEnvs?: Record<string, string>;
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
  async applyAnalysis(appId: string, req: ApplyAnalysisRequest): Promise<ApplyAnalysisResponse> {
    const { data } = await api.post<ApplyAnalysisResponse>(`/apps/${appId}/apply-analysis`, req);
    return data;
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
  async testConnection(params: {
    apiKey?: string;
    model?: string;
  }): Promise<AITestResult> {
    const { data } = await api.post<AITestResult>('/admin/settings/ai/test', params);
    return data;
  },
};
