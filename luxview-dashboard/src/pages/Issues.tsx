import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, CircleDot, CheckCircle2, Plus, Loader2 } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { issuesApi, type Issue, type IssueStatus } from '../api/issues';

function LabelChip({ name, color }: { name: string; color: string }) {
  return (
    <span
      className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium border"
      style={{ color, borderColor: color + '66', backgroundColor: color + '1a' }}
    >
      {name}
    </span>
  );
}

export function Issues() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const [issues, setIssues] = useState<Issue[]>([]);
  const [filter, setFilter] = useState<IssueStatus>('open');
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!repoId) return;
    setLoading(true);
    try {
      const { issues: list } = await issuesApi.list(repoId, filter);
      setIssues(list);
    } finally {
      setLoading(false);
    }
  }, [repoId, filter]);

  useEffect(() => { load(); }, [load]);

  return (
    <div className="animate-fade-in space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}`)} icon={<ArrowLeft size={16} />}>
            {t('common.back')}
          </PillButton>
          <h1 className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('issues.title')}
          </h1>
        </div>
        <PillButton variant="primary" size="sm" icon={<Plus size={14} />} onClick={() => navigate(`/dashboard/repositories/${repoId}/issues/new`)}>
          {t('issues.new')}
        </PillButton>
      </div>

      <div className="flex gap-1">
        {(['open', 'closed'] as IssueStatus[]).map((s) => (
          <button
            key={s}
            onClick={() => setFilter(s)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
              filter === s
                ? 'bg-amber-400/10 text-amber-400'
                : isDark ? 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50' : 'text-zinc-400 hover:text-zinc-700 hover:bg-zinc-100'
            }`}
          >
            {s === 'open' ? <CircleDot size={13} /> : <CheckCircle2 size={13} />}
            {s === 'open' ? t('issues.filterOpen') : t('issues.filterClosed')}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="flex justify-center py-16"><Loader2 className="animate-spin text-zinc-400" size={22} /></div>
      ) : issues.length === 0 ? (
        <GlassCard padding="lg" className="text-center py-12">
          <p className="text-sm text-zinc-500">{t('issues.empty')}</p>
          <p className="text-xs text-zinc-600 mt-1">{t('issues.emptyHint')}</p>
        </GlassCard>
      ) : (
        <GlassCard padding="sm" className="divide-y divide-zinc-800/30">
          {issues.map((i) => (
            <button
              key={i.id}
              onClick={() => navigate(`/dashboard/repositories/${repoId}/issues/${i.number}`)}
              className="w-full flex items-start gap-3 px-3 py-3 text-left hover:bg-white/5 transition-colors"
            >
              {i.status === 'open'
                ? <CircleDot size={16} className="text-emerald-400 flex-shrink-0 mt-0.5" />
                : <CheckCircle2 size={16} className="text-purple-400 flex-shrink-0 mt-0.5" />}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{i.title}</span>
                  {i.labels?.map((l) => <LabelChip key={l.id} name={l.name} color={l.color} />)}
                </div>
                <p className="text-xs text-zinc-500 mt-0.5">
                  #{i.number} · {i.author?.username ?? t('issues.unknownAuthor')} · {new Date(i.createdAt).toLocaleDateString()}
                </p>
              </div>
            </button>
          ))}
        </GlassCard>
      )}
    </div>
  );
}
