import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, GitPullRequest, Plus, Loader2, GitMerge, XCircle } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { pullRequestsApi, type PullRequest, type PullRequestStatus } from '../api/repositories';

const STATUS_TABS: { key: PullRequestStatus | ''; label: string }[] = [
  { key: 'open', label: 'pr.statusOpen' },
  { key: 'merged', label: 'pr.statusMerged' },
  { key: 'closed', label: 'pr.statusClosed' },
];

function statusIcon(status: PullRequestStatus) {
  if (status === 'merged') return <GitMerge size={14} className="text-purple-400" />;
  if (status === 'closed') return <XCircle size={14} className="text-zinc-500" />;
  return <GitPullRequest size={14} className="text-emerald-400" />;
}

function statusColor(status: PullRequestStatus) {
  if (status === 'merged') return 'text-purple-400';
  if (status === 'closed') return 'text-zinc-500';
  return 'text-emerald-400';
}

export function PullRequests() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const [status, setStatus] = useState<PullRequestStatus | ''>('open');
  const [prs, setPrs] = useState<PullRequest[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!repoId) return;
    setLoading(true);
    pullRequestsApi
      .list(repoId, status)
      .then(({ pullRequests, total }) => {
        setPrs(pullRequests);
        setTotal(total);
      })
      .finally(() => setLoading(false));
  }, [repoId, status]);

  if (!repoId) return null;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}`)} icon={<ArrowLeft size={16} />}>
            {t('common.back')}
          </PillButton>
          <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('pr.title')}
          </h1>
        </div>
        <PillButton
          variant="primary"
          size="sm"
          icon={<Plus size={14} />}
          onClick={() => navigate(`/dashboard/repositories/${repoId}/pulls/new`)}
        >
          {t('pr.new')}
        </PillButton>
      </div>

      {/* Status tabs */}
      <div className="flex gap-1">
        {STATUS_TABS.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setStatus(key)}
            className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
              status === key
                ? 'bg-amber-400/10 text-amber-400'
                : isDark
                  ? 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50'
                  : 'text-zinc-400 hover:text-zinc-700 hover:bg-zinc-100'
            }`}
          >
            {t(label)}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="animate-spin text-zinc-400" size={24} />
        </div>
      ) : prs.length === 0 ? (
        <GlassCard padding="lg" className="text-center py-12">
          <GitPullRequest size={32} className="mx-auto text-zinc-500 mb-3" />
          <p className={`text-sm font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{t('pr.empty')}</p>
          {status === 'open' && (
            <div className="mt-4">
              <PillButton variant="primary" size="sm" icon={<Plus size={14} />} onClick={() => navigate(`/dashboard/repositories/${repoId}/pulls/new`)}>
                {t('pr.new')}
              </PillButton>
            </div>
          )}
        </GlassCard>
      ) : (
        <div className="space-y-2">
          <p className="text-xs text-zinc-500">{total} {t('pr.count')}</p>
          {prs.map((pr) => (
            <GlassCard
              key={pr.id}
              padding="md"
              hover
              className="cursor-pointer"
              onClick={() => navigate(`/dashboard/repositories/${repoId}/pulls/${pr.number}`)}
            >
              <div className="flex items-center gap-3">
                <div className="flex-shrink-0">{statusIcon(pr.status)}</div>
                <div className="flex-1 min-w-0">
                  <p className={`text-sm font-semibold truncate ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                    #{pr.number} {pr.title}
                  </p>
                  <p className="text-xs text-zinc-500 mt-0.5 font-mono">
                    {pr.headBranch} → {pr.baseBranch}
                  </p>
                </div>
                <span className={`text-xs font-medium ${statusColor(pr.status)}`}>
                  {t(`pr.status.${pr.status}`)}
                </span>
              </div>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
}
