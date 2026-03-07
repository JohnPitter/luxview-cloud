import { useState, useEffect } from 'react';
import { FileText, RefreshCw, Search, ExternalLink } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useAppsStore } from '../stores/apps.store';
import { useThemeStore } from '../stores/theme.store';
import { deploymentsApi, type Deployment } from '../api/deployments';
import { formatRelativeTime } from '../lib/format';

const statusColors: Record<string, string> = {
  live: 'text-emerald-400 bg-emerald-400/10',
  building: 'text-amber-400 bg-amber-400/10',
  deploying: 'text-blue-400 bg-blue-400/10',
  failed: 'text-red-400 bg-red-400/10',
  rolled_back: 'text-zinc-400 bg-zinc-400/10',
  pending: 'text-zinc-400 bg-zinc-400/10',
};

interface DeploymentWithApp extends Deployment {
  appName: string;
  appSubdomain: string;
}

export function Logs() {
  const navigate = useNavigate();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const { apps, fetchApps } = useAppsStore();
  const [deployments, setDeployments] = useState<DeploymentWithApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [expandedLog, setExpandedLog] = useState<string | null>(null);
  const [logContent, setLogContent] = useState<Record<string, string>>({});

  const fetchAllDeployments = async () => {
    setLoading(true);
    try {
      const allDeps: DeploymentWithApp[] = [];
      for (const app of apps) {
        const deps = await deploymentsApi.list(app.id, 10, 0);
        allDeps.push(
          ...deps.map((d) => ({
            ...d,
            appName: app.name,
            appSubdomain: app.subdomain,
          })),
        );
      }
      allDeps.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
      setDeployments(allDeps);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchApps();
  }, [fetchApps]);

  useEffect(() => {
    if (apps.length > 0) {
      fetchAllDeployments();
    }
  }, [apps.length]);

  const toggleLog = async (deployId: string) => {
    if (expandedLog === deployId) {
      setExpandedLog(null);
      return;
    }
    setExpandedLog(deployId);
    if (!logContent[deployId]) {
      try {
        const data = await deploymentsApi.getLogs(deployId);
        setLogContent((prev) => ({ ...prev, [deployId]: data.buildLog || 'No logs available.' }));
      } catch {
        setLogContent((prev) => ({ ...prev, [deployId]: 'Failed to load logs.' }));
      }
    }
  };

  const filtered = deployments.filter(
    (d) =>
      !filter ||
      d.appName.toLowerCase().includes(filter.toLowerCase()) ||
      d.commitMessage.toLowerCase().includes(filter.toLowerCase()) ||
      d.status.toLowerCase().includes(filter.toLowerCase()),
  );

  return (
    <div className="animate-fade-in">
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <FileText size={24} className="text-amber-400" />
          <div>
            <h1
              className={`text-2xl font-bold tracking-tight ${
                isDark ? 'text-zinc-100' : 'text-zinc-900'
              }`}
            >
              Deployment Logs
            </h1>
            <p className="text-sm text-zinc-500">Build and deployment history across all apps</p>
          </div>
        </div>
        <PillButton
          variant="secondary"
          size="sm"
          onClick={fetchAllDeployments}
          icon={<RefreshCw size={14} />}
        >
          Refresh
        </PillButton>
      </div>

      {/* Search */}
      <div className="mb-6">
        <div
          className={`
            flex items-center gap-2 px-4 py-2.5 rounded-xl
            ${isDark ? 'bg-zinc-900/50 border border-zinc-800/50' : 'bg-white border border-zinc-200'}
          `}
        >
          <Search size={16} className="text-zinc-500" />
          <input
            type="text"
            placeholder="Filter by app, commit, or status..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className={`
              flex-1 bg-transparent text-sm outline-none
              ${isDark ? 'text-zinc-200 placeholder:text-zinc-600' : 'text-zinc-800 placeholder:text-zinc-400'}
            `}
          />
        </div>
      </div>

      {/* Deployments List */}
      <GlassCard padding="none">
        {loading ? (
          <div className="py-16 text-center text-sm text-zinc-500">Loading deployments...</div>
        ) : filtered.length === 0 ? (
          <div className="py-16 text-center text-sm text-zinc-500">
            {filter ? 'No deployments match your filter.' : 'No deployments found.'}
          </div>
        ) : (
          <div className="divide-y divide-zinc-800/30">
            {filtered.map((dep) => (
              <div key={dep.id}>
                <button
                  onClick={() => toggleLog(dep.id)}
                  className={`
                    w-full flex items-center gap-4 px-6 py-4 text-left transition-colors
                    ${isDark ? 'hover:bg-zinc-800/20' : 'hover:bg-zinc-50'}
                  `}
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span
                        className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
                      >
                        {dep.appName}
                      </span>
                      <span
                        className={`text-[10px] font-mono px-1.5 py-0.5 rounded ${statusColors[dep.status] || 'text-zinc-400 bg-zinc-400/10'}`}
                      >
                        {dep.status}
                      </span>
                      <span className="text-[10px] font-mono text-zinc-500">
                        {dep.commitSha?.slice(0, 7)}
                      </span>
                    </div>
                    <p className="text-xs text-zinc-500 truncate">{dep.commitMessage}</p>
                  </div>
                  <div className="text-right flex-shrink-0">
                    <p className="text-[11px] text-zinc-500">{formatRelativeTime(dep.createdAt)}</p>
                    {dep.durationMs > 0 && (
                      <p className="text-[10px] text-zinc-600">
                        {(dep.durationMs / 1000).toFixed(1)}s
                      </p>
                    )}
                  </div>
                  <ExternalLink
                    size={14}
                    className="text-zinc-600 flex-shrink-0 cursor-pointer hover:text-amber-400"
                    onClick={(e) => {
                      e.stopPropagation();
                      navigate(`/dashboard/apps/${dep.appId}`);
                    }}
                  />
                </button>
                {expandedLog === dep.id && (
                  <div className={`px-6 pb-4 ${isDark ? 'bg-zinc-950/50' : 'bg-zinc-50'}`}>
                    <pre
                      className={`
                        text-[11px] font-mono leading-relaxed p-4 rounded-lg max-h-64 overflow-auto
                        ${isDark ? 'bg-black/50 text-zinc-400' : 'bg-white text-zinc-600 border border-zinc-200'}
                      `}
                    >
                      {logContent[dep.id] || 'Loading...'}
                    </pre>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </GlassCard>
    </div>
  );
}
