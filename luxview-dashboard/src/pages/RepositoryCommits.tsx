import { useState, useEffect } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, GitCommit, Loader2, ChevronLeft, ChevronRight } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { gitApi, type CommitEntry } from '../api/git';
import { repositoriesApi } from '../api/repositories';

const PAGE_SIZE = 30;

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  } catch {
    return iso;
  }
}

export function RepositoryCommits() {
  const { repoId } = useParams<{ repoId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const ref = searchParams.get('ref') ?? '';
  const page = parseInt(searchParams.get('page') ?? '0', 10);

  const [defaultBranch, setDefaultBranch] = useState('main');
  const [branches, setBranches] = useState<string[]>([]);
  const [commits, setCommits] = useState<CommitEntry[]>([]);
  const [loading, setLoading] = useState(true);

  const currentRef = ref || defaultBranch;

  useEffect(() => {
    if (!repoId) return;
    Promise.all([
      repositoriesApi.list(100).then((repos) => repos.find((r) => r.id === repoId)?.defaultBranch ?? 'main'),
      repositoriesApi.listBranches(repoId),
    ]).then(([db, b]) => {
      setDefaultBranch(db);
      setBranches(b);
    });
  }, [repoId]);

  useEffect(() => {
    if (!repoId) return;
    setLoading(true);
    gitApi.commits(repoId, currentRef, PAGE_SIZE, page * PAGE_SIZE)
      .then(({ commits: c }) => setCommits(c))
      .finally(() => setLoading(false));
  }, [repoId, currentRef, page]);

  function setPage(p: number) {
    const params = new URLSearchParams(searchParams);
    params.set('page', String(p));
    setSearchParams(params);
  }

  function setRef(r: string) {
    const params = new URLSearchParams();
    params.set('ref', r);
    setSearchParams(params);
  }

  return (
    <div className="animate-fade-in space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}`)} icon={<ArrowLeft size={16} />}>
            {t('common.back')}
          </PillButton>
          <h1 className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('commits.title')}
          </h1>
        </div>
        <select
          value={currentRef}
          onChange={(e) => setRef(e.target.value)}
          className={`px-3 py-1.5 text-xs rounded-lg border ${isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'}`}
        >
          {branches.map((b) => <option key={b} value={b}>{b}</option>)}
          {branches.length === 0 && currentRef && <option value={currentRef}>{currentRef}</option>}
        </select>
      </div>

      {loading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="animate-spin text-zinc-400" size={24} />
        </div>
      ) : commits.length === 0 ? (
        <GlassCard padding="lg" className="text-center py-12">
          <GitCommit size={32} className="mx-auto text-zinc-500 mb-3" />
          <p className="text-sm text-zinc-500">{t('commits.empty')}</p>
        </GlassCard>
      ) : (
        <div className="space-y-2">
          {commits.map((c) => (
            <GlassCard
              key={c.sha}
              padding="md"
              hover
              className="cursor-pointer"
              onClick={() => navigate(`/dashboard/repositories/${repoId}/commits/${c.sha}`)}
            >
              <div className="flex items-start gap-3">
                <GitCommit size={15} className="text-zinc-500 flex-shrink-0 mt-0.5" />
                <div className="flex-1 min-w-0">
                  <p className={`text-sm font-medium truncate ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                    {c.message}
                  </p>
                  <p className="text-xs text-zinc-500 mt-0.5">
                    {c.author} · {formatDate(c.date)}
                  </p>
                </div>
                <span className="font-mono text-xs text-zinc-600 flex-shrink-0">{c.sha.slice(0, 8)}</span>
              </div>
            </GlassCard>
          ))}

          {/* Pagination */}
          <div className="flex items-center justify-between pt-2">
            <PillButton
              variant="ghost"
              size="sm"
              icon={<ChevronLeft size={14} />}
              onClick={() => setPage(Math.max(0, page - 1))}
              disabled={page === 0}
            >
              {t('common.previous')}
            </PillButton>
            <span className="text-xs text-zinc-500">{t('commits.page')} {page + 1}</span>
            <PillButton
              variant="ghost"
              size="sm"
              onClick={() => setPage(page + 1)}
              disabled={commits.length < PAGE_SIZE}
            >
              {t('common.next')} <ChevronRight size={14} />
            </PillButton>
          </div>
        </div>
      )}
    </div>
  );
}
