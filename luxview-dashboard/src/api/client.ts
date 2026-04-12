import axios from 'axios';

function snakeToCamel(str: string): string {
  if (typeof str !== 'string') return str;
  return str.replace(/_([a-z])/g, (_, c) => c.toUpperCase());
}

function camelToSnake(str: string): string {
  if (typeof str !== 'string') return str;
  return str.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`);
}

// Keys that should NOT have their nested keys transformed (e.g., user-defined env vars).
const PRESERVE_NESTED_KEYS = new Set(['env_vars', 'envVars', 'resource_limits', 'resourceLimits']);

function transformKeys(obj: unknown, fn: (key: string) => string, preserveValues = false): unknown {
  if (Array.isArray(obj)) return obj.map((item) => transformKeys(item, fn, preserveValues));
  if (obj !== null && typeof obj === 'object' && !(obj instanceof Date)) {
    return Object.fromEntries(
      Object.entries(obj as Record<string, unknown>).map(([key, value]) => [
        preserveValues ? key : fn(key),
        PRESERVE_NESTED_KEYS.has(key)
          ? value // preserve user-defined keys as-is
          : transformKeys(value, fn, preserveValues),
      ]),
    );
  }
  return obj;
}

export const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true,
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('lv_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  // Send current locale for AI responses
  const lang = localStorage.getItem('i18nextLng') || navigator.language || 'en';
  config.headers['Accept-Language'] = lang;
  if (config.data && typeof config.data === 'object') {
    config.data = transformKeys(config.data, camelToSnake);
  }
  return config;
});

// Raw API instance that preserves response keys as-is (no snakeToCamel transform).
// Used for endpoints where original keys matter (e.g., DB Explorer column names).
export const apiRaw = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
  withCredentials: true,
});

apiRaw.interceptors.request.use((config) => {
  const token = localStorage.getItem('lv_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => {
    if (response.data && typeof response.data === 'object' && !(response.data instanceof Blob) && !(response.data instanceof ArrayBuffer)) {
      response.data = transformKeys(response.data, snakeToCamel);
    }
    return response;
  },
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('lv_token');
      window.location.href = '/';
    }
    return Promise.reject(error);
  },
);
