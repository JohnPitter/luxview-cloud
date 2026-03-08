import { useState, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Search, GitBranch, Lock, Globe } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { useThemeStore } from '../../stores/theme.store';
import type { GithubRepo } from '../../api/github';

interface RepoSelectorProps {
  repos: GithubRepo[];
  loading: boolean;
  selected: GithubRepo | null;
  onSelect: (repo: GithubRepo) => void;
}

const langColors: Record<string, string> = {
  TypeScript: 'bg-blue-500',
  JavaScript: 'bg-amber-400',
  Python: 'bg-emerald-500',
  Go: 'bg-cyan-500',
  Rust: 'bg-orange-500',
  Java: 'bg-red-500',
  Ruby: 'bg-red-400',
  PHP: 'bg-violet-500',
};

export function RepoSelector({ repos, loading, selected, onSelect }: RepoSelectorProps) {
  const [search, setSearch] = useState('');
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const filtered = useMemo(
    () =>
      repos
        .filter(
          (r) =>
            r.name.toLowerCase().includes(search.toLowerCase()) ||
            r.fullName.toLowerCase().includes(search.toLowerCase()),
        )
        .sort((a, b) => a.name.localeCompare(b.name)),
    [repos, search],
  );

  return (
    <div className="space-y-4">
      {/* Search */}
      <div className="relative">
        <Search
          size={16}
          className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500"
        />
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t('deploy.wizard.selectRepo.searchPlaceholder')}
          className={`
            w-full pl-10 pr-4 py-2.5 rounded-xl text-sm
            border transition-all duration-200
            focus:outline-none focus:ring-2 focus:ring-amber-400/30
            ${
              isDark
                ? 'bg-zinc-900/50 border-zinc-800 text-zinc-100 placeholder-zinc-600'
                : 'bg-white border-zinc-200 text-zinc-900 placeholder-zinc-400'
            }
          `}
        />
      </div>

      {/* Repo List */}
      <div className="max-h-80 overflow-y-auto space-y-2 pr-1">
        {loading ? (
          Array.from({ length: 5 }).map((_, i) => (
            <div
              key={i}
              className={`h-16 rounded-xl animate-pulse ${
                isDark ? 'bg-zinc-800/50' : 'bg-zinc-100'
              }`}
            />
          ))
        ) : filtered.length === 0 ? (
          <p className="text-center text-sm text-zinc-500 py-8">{t('deploy.wizard.selectRepo.noReposFound')}</p>
        ) : (
          filtered.map((repo) => {
            const isSelected = selected?.id === repo.id;
            return (
              <button
                key={repo.id}
                onClick={() => onSelect(repo)}
                className={`
                  w-full flex items-center gap-3 p-3 rounded-xl text-left
                  transition-all duration-200
                  ${
                    isSelected
                      ? 'ring-2 ring-amber-400/50 bg-amber-400/5'
                      : isDark
                        ? 'hover:bg-zinc-800/50 bg-zinc-900/30'
                        : 'hover:bg-zinc-50 bg-white'
                  }
                  ${isDark ? 'border border-zinc-800/50' : 'border border-zinc-200/60'}
                `}
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span
                      className={`text-sm font-medium truncate ${
                        isDark ? 'text-zinc-200' : 'text-zinc-800'
                      }`}
                    >
                      {repo.name}
                    </span>
                    {repo.private ? (
                      <Lock size={12} className="text-zinc-500 flex-shrink-0" />
                    ) : (
                      <Globe size={12} className="text-zinc-500 flex-shrink-0" />
                    )}
                  </div>
                  {repo.description && (
                    <p className="text-xs text-zinc-500 truncate mt-0.5">
                      {repo.description}
                    </p>
                  )}
                </div>
                <div className="flex items-center gap-3 flex-shrink-0">
                  {repo.language && (
                    <div className="flex items-center gap-1.5">
                      <span
                        className={`w-2.5 h-2.5 rounded-full ${
                          langColors[repo.language] || 'bg-zinc-500'
                        }`}
                      />
                      <span className="text-[11px] text-zinc-500">{repo.language}</span>
                    </div>
                  )}
                  <div className="flex items-center gap-1 text-[11px] text-zinc-500">
                    <GitBranch size={12} />
                    {repo.defaultBranch}
                  </div>
                </div>
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}
