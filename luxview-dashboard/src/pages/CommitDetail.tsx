import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, GitCommit, Loader2, FileCode } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { gitApi, type CommitEntry, type FileDiff } from '../api/git';

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

export function CommitDetail() {
  const { repoId, sha } = useParams<{ repoId: string; sha: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const [commit, setCommit] = useState<CommitEntry | null>(null);
  const [files, setFiles] = useState<FileDiff[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (!repoId || !sha) return;
    gitApi.commit(repoId, sha)
      .then(({ commit: c, files: f }) => {
        setCommit(c);
        setFiles(f);
        // expand all files if few
        if (f.length <= 5) setExpandedFiles(new Set(f.map((file) => file.path)));
      })
      .finally(() => setLoading(false));
  }, [repoId, sha]);

  function toggleFile(path: string) {
    setExpandedFiles((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  }

  if (loading) {
    return (
      <div className="flex justify-center py-24">
        <Loader2 className="animate-spin text-zinc-400" size={24} />
      </div>
    );
  }

  if (!commit) {
    return (
      <div className="text-center py-24">
        <p className="text-zinc-500">{t('commits.notFound')}</p>
      </div>
    );
  }

  const totalAdditions = files.reduce((s, f) => s + f.additions, 0);
  const totalDeletions = files.reduce((s, f) => s + f.deletions, 0);

  return (
    <div className="animate-fade-in space-y-4">
      <div className="flex items-center gap-4">
        <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}/commits`)} icon={<ArrowLeft size={16} />}>
          {t('common.back')}
        </PillButton>
        <h1 className={`text-lg font-bold tracking-tight truncate ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
          {commit.message}
        </h1>
      </div>

      <GlassCard padding="md" className="space-y-2">
        <div className="flex items-center gap-2">
          <GitCommit size={15} className="text-zinc-500" />
          <span className="font-mono text-xs text-zinc-400">{commit.sha}</span>
        </div>
        <p className="text-xs text-zinc-500">
          {commit.author} · {commit.email} · {formatDate(commit.date)}
        </p>
        <p className="text-xs text-zinc-500">
          <span className="text-emerald-400">+{totalAdditions}</span>{' '}
          <span className="text-red-400">-{totalDeletions}</span>{' '}
          {t('commits.changedFiles', { count: files.length })}
        </p>
      </GlassCard>

      {/* File diffs */}
      <div className="space-y-2">
        {files.map((f) => (
          <GlassCard key={f.path} padding="sm" className="overflow-hidden">
            <button
              onClick={() => toggleFile(f.path)}
              className="w-full flex items-center justify-between gap-3 text-left px-1 py-1"
            >
              <div className="flex items-center gap-2 min-w-0">
                <FileCode size={14} className="text-zinc-500 flex-shrink-0" />
                <span className={`text-xs font-mono truncate ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{f.path}</span>
              </div>
              <div className="flex items-center gap-2 flex-shrink-0 text-xs">
                <span className="text-emerald-400">+{f.additions}</span>
                <span className="text-red-400">-{f.deletions}</span>
              </div>
            </button>
            {expandedFiles.has(f.path) && f.patch && (
              <pre className={`mt-2 text-xs font-mono overflow-x-auto p-3 rounded-lg ${isDark ? 'bg-zinc-900/70 text-zinc-300' : 'bg-zinc-100 text-zinc-700'}`}>
                {f.patch.split('\n').map((line, i) => (
                  <div key={i} className={
                    line.startsWith('+') && !line.startsWith('+++') ? 'text-emerald-400' :
                    line.startsWith('-') && !line.startsWith('---') ? 'text-red-400' :
                    line.startsWith('@@') ? 'text-blue-400' : ''
                  }>{line || ' '}</div>
                ))}
              </pre>
            )}
          </GlassCard>
        ))}
      </div>
    </div>
  );
}
