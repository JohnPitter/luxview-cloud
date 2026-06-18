import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { GitBranch, Plus, Loader2, Settings } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { repositoriesApi, type LuxViewRepository } from '../api/repositories';

export function Repositories() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const [repos, setRepos] = useState<LuxViewRepository[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    repositoriesApi
      .list()
      .then(setRepos)
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('repo.list.title')}
          </h1>
          <p className="text-sm text-zinc-500 mt-0.5">{t('repo.list.subtitle')}</p>
        </div>
        <PillButton
          variant="primary"
          size="sm"
          icon={<Plus size={14} />}
          onClick={() => navigate('/dashboard/repositories/new')}
        >
          {t('repo.list.new')}
        </PillButton>
      </div>

      {loading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="animate-spin text-zinc-400" size={24} />
        </div>
      ) : repos.length === 0 ? (
        <GlassCard padding="lg" className="text-center py-12">
          <GitBranch size={32} className="mx-auto text-zinc-500 mb-3" />
          <p className={`text-sm font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
            {t('repo.list.empty')}
          </p>
          <p className="text-xs text-zinc-500 mt-1 mb-4">{t('repo.list.emptyHint')}</p>
          <PillButton
            variant="primary"
            size="sm"
            icon={<Plus size={14} />}
            onClick={() => navigate('/dashboard/repositories/new')}
          >
            {t('repo.list.new')}
          </PillButton>
        </GlassCard>
      ) : (
        <div className="space-y-2">
          {repos.map((repo) => (
            <GlassCard
              key={repo.id}
              padding="md"
              hover
              className="cursor-pointer"
              onClick={() => navigate(`/dashboard/repositories/${repo.id}`)}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg bg-indigo-500/10">
                    <GitBranch size={16} className="text-indigo-400" />
                  </div>
                  <div>
                    <p className={`text-sm font-semibold ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                      {repo.name}
                    </p>
                    {repo.description && (
                      <p className={`text-xs mt-0.5 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                        {repo.description}
                      </p>
                    )}
                    <p className="text-xs text-zinc-500 font-mono mt-0.5">
                      {repo.slug} · {repo.defaultBranch}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <span className={`text-xs px-2 py-0.5 rounded-full border ${
                    repo.visibility === 'public'
                      ? 'text-emerald-400 border-emerald-400/20 bg-emerald-400/5'
                      : 'text-zinc-500 border-zinc-700/30 bg-zinc-500/5'
                  }`}>
                    {t(`repo.new.${repo.visibility}`)}
                  </span>
                  <button
                    onClick={(e) => { e.stopPropagation(); navigate(`/dashboard/repositories/${repo.id}`); }}
                    className="p-1.5 text-zinc-500 hover:text-zinc-300 transition-colors rounded-lg hover:bg-white/5"
                  >
                    <Settings size={14} />
                  </button>
                </div>
              </div>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
}
