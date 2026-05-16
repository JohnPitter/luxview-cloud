import { api } from './client';

export type ActionStatus = 'queued' | 'running' | 'success' | 'failed' | 'cancelled' | 'skipped';

export interface ActionRun {
  id: string;
  appId: string;
  workflow: string;
  workflowPath: string;
  trigger: string;
  branch: string;
  commitSha: string;
  status: ActionStatus;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
}

export interface ActionJob {
  id: string;
  runId: string;
  name: string;
  image: string;
  status: ActionStatus;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
}

export interface ActionStep {
  id: string;
  jobId: string;
  name: string;
  kind: string;
  command: string;
  uses: string;
  inputs?: Record<string, string>;
  status: ActionStatus;
  exitCode: number;
  log: string;
  position: number;
  startedAt?: string;
  finishedAt?: string;
}

export interface ActionRunDetail {
  run: ActionRun;
  jobs: ActionJob[];
  steps: ActionStep[];
}

export interface ActionArtifact {
  id: string;
  runId: string;
  name: string;
  path: string;
  sizeBytes: number;
  createdAt: string;
}

export interface ActionSecret {
  id: string;
  appId: string;
  key: string;
  createdAt: string;
  updatedAt: string;
}

export const actionsApi = {
  async listRuns(appId: string, limit = 20, offset = 0): Promise<{ runs: ActionRun[]; total: number }> {
    const { data } = await api.get<{ runs: ActionRun[]; total: number }>(
      `/apps/${appId}/actions/runs`,
      { params: { limit, offset } },
    );
    return { runs: data.runs ?? [], total: data.total ?? 0 };
  },

  async triggerRun(appId: string, workflowPath?: string): Promise<ActionRun> {
    const { data } = await api.post<ActionRun>(`/apps/${appId}/actions/runs`, {
      workflowPath: workflowPath ?? '',
      trigger: 'manual',
    });
    return data;
  },

  async getRun(runId: string): Promise<ActionRunDetail> {
    const { data } = await api.get<ActionRunDetail>(`/actions/runs/${runId}`);
    return data;
  },

  async listArtifacts(runId: string): Promise<ActionArtifact[]> {
    const { data } = await api.get<{ artifacts: ActionArtifact[] }>(`/actions/runs/${runId}/artifacts`);
    return data.artifacts ?? [];
  },

  async listSecrets(appId: string): Promise<ActionSecret[]> {
    const { data } = await api.get<{ secrets: ActionSecret[] }>(`/apps/${appId}/actions/secrets`);
    return data.secrets ?? [];
  },

  async upsertSecret(appId: string, key: string, value: string): Promise<ActionSecret> {
    const { data } = await api.put<ActionSecret>(`/apps/${appId}/actions/secrets/${key}`, { value });
    return data;
  },

  async deleteSecret(appId: string, key: string): Promise<void> {
    await api.delete(`/apps/${appId}/actions/secrets/${key}`);
  },
};
