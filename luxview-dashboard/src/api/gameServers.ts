import { api } from './client';
import type { GameServerConfig, GameVolume, CreateGameServerPayload, App, ExtraPort } from './apps';

export interface ConfigFieldOption {
  value: string;
  label: string;
}

export interface ConfigFieldDef {
  key: string;
  label: string;
  type: 'text' | 'password' | 'number' | 'select';
  options?: ConfigFieldOption[];
  placeholder?: string;
  section?: string;
}

export interface GameTemplate {
  id: string;
  displayName: string;
  description: string;
  protocol: string;
  defaultGamePort: number;
  defaultQueryPort: number;
  defaultExtraPorts?: ExtraPort[];
  defaultImage: string;
  configFields: ConfigFieldDef[];
  supportsQuery: boolean;
  defaultVolumes: GameVolume[];
}

export interface GameConfigResponse extends GameServerConfig {
  template?: GameTemplate;
  serverIp?: string;
}

export interface GameServerStatus {
  running: boolean;
  players: number;
  maxPlayers: number;
}

export interface PlayerInfo {
  name: string;
  score: number;
  duration: number; // seconds
}

export const gameServersApi = {
  async getTemplates(): Promise<GameTemplate[]> {
    const { data } = await api.get<GameTemplate[]>('/game-templates');
    return data;
  },

  async createGameServer(payload: CreateGameServerPayload): Promise<App> {
    const { data } = await api.post<App>('/apps', payload);
    return data;
  },

  async getConfig(appId: string): Promise<GameConfigResponse> {
    const { data } = await api.get<GameConfigResponse>(`/apps/${appId}/game-config`);
    return data;
  },

  async updateConfig(appId: string, fields: Record<string, string>): Promise<GameServerConfig> {
    // Wrap in configFields so the request interceptor preserves the user-defined keys
    // (configFields is in PRESERVE_NESTED_KEYS).
    const { data } = await api.put<GameServerConfig>(`/apps/${appId}/game-config`, { configFields: fields });
    return data;
  },

  async getStatus(appId: string): Promise<GameServerStatus> {
    const { data } = await api.get<GameServerStatus>(`/apps/${appId}/game-status`);
    return data;
  },

  async getPlayers(appId: string): Promise<PlayerInfo[]> {
    const { data } = await api.get<PlayerInfo[]>(`/apps/${appId}/game-players`);
    return data;
  },
};
