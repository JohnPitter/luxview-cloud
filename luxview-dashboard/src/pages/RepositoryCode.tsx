import { useState, useEffect } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Folder, FileCode, Loader2, ChevronRight, Copy, Check } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { gitApi, type TreeEntry } from '../api/git';
import { repositoriesApi, type LuxViewRepository } from '../api/repositories';

function isBinary(content: string): boolean {
  for (let i = 0; i < Math.min(content.length, 8000); i++) {
    const code = content.charCodeAt(i);
    if (code === 0) return true;
  }
  return false;
}


export function RepositoryCode() {
  const { repoId } = useParams<{ repoId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const ref = searchParams.get('ref') ?? '';
  const path = searchParams.get('path') ?? '';

  const [repo, setRepo] = useState<LuxViewRepository | null>(null);
  const [branches, setBranches] = useState<string[]>([]);
  const [entries, setEntries] = useState<TreeEntry[]>([]);
  const [blobContent, setBlobContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);

  const currentRef = ref || repo?.defaultBranch || '';

  useEffect(() => {
    if (!repoId) return;
    Promise.all([
      repositoriesApi.list(100).then((repos) => repos.find((r) => r.id === repoId) ?? null),
      repositoriesApi.listBranches(repoId),
    ]).then(([r, b]) => {
      setRepo(r);
      setBranches(b);
    });
  }, [repoId]);

  useEffect(() => {
    if (!repoId) return;
    setLoading(true);
    setBlobContent(null);
    // Detect if path points to a file by looking at current entries or assume tree
    gitApi.tree(repoId, currentRef, path)
      .then(({ entries: e }) => {
        setEntries(e);
        setLoading(false);
      })
      .catch(() => {
        // Try blob
        gitApi.blob(repoId, path, currentRef)
          .then(({ content }) => {
            setBlobContent(content);
            setLoading(false);
          })
          .catch(() => {
            setLoading(false);
            addNotification({ type: 'error', title: t('code.notFound') });
          });
      });
  }, [repoId, currentRef, path, t, addNotification]);

  function navigate2(newPath: string, isDir = true) {
    const params = new URLSearchParams();
    if (currentRef) params.set('ref', currentRef);
    if (newPath) params.set('path', newPath);
    setSearchParams(params);
    if (!isDir) {
      // blob: keep in same page (effect reruns)
    }
  }

  function handleEntry(entry: TreeEntry) {
    navigate2(entry.path, entry.type === 'tree');
  }

  function breadcrumbs() {
    if (!path) return [];
    const parts = path.split('/');
    return parts.map((p, i) => ({ name: p, path: parts.slice(0, i + 1).join('/') }));
  }

  function copyContent() {
    if (!blobContent) return;
    navigator.clipboard.writeText(blobContent);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  const crumbs = breadcrumbs();
  const fileName = crumbs.length > 0 ? crumbs[crumbs.length - 1].name : '';

  return (
    <div className="animate-fade-in space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}`)} icon={<ArrowLeft size={16} />}>
            {t('common.back')}
          </PillButton>
          <div>
            <h1 className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
              {repo?.name ?? t('code.title')}
            </h1>
          </div>
        </div>
        {/* Branch selector */}
        <select
          value={currentRef}
          onChange={(e) => {
            const params = new URLSearchParams();
            params.set('ref', e.target.value);
            if (path) params.set('path', path);
            setSearchParams(params);
          }}
          className={`px-3 py-1.5 text-xs rounded-lg border ${
            isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'
          }`}
        >
          {branches.map((b) => (
            <option key={b} value={b}>{b}</option>
          ))}
          {branches.length === 0 && currentRef && (
            <option value={currentRef}>{currentRef}</option>
          )}
        </select>
      </div>

      {/* Breadcrumb */}
      <div className="flex items-center gap-1 text-xs font-mono flex-wrap">
        <button
          onClick={() => navigate2('', true)}
          className="text-amber-400 hover:underline"
        >
          {repo?.slug ?? 'root'}
        </button>
        {crumbs.map((c, i) => (
          <span key={c.path} className="flex items-center gap-1">
            <ChevronRight size={12} className="text-zinc-600" />
            {i < crumbs.length - 1 ? (
              <button onClick={() => navigate2(c.path, true)} className="text-amber-400 hover:underline">
                {c.name}
              </button>
            ) : (
              <span className={isDark ? 'text-zinc-200' : 'text-zinc-800'}>{c.name}</span>
            )}
          </span>
        ))}
      </div>

      {loading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="animate-spin text-zinc-400" size={24} />
        </div>
      ) : blobContent !== null ? (
        /* File viewer */
        <GlassCard padding="sm" className="overflow-hidden">
          <div className={`flex items-center justify-between px-3 py-2 border-b ${isDark ? 'border-zinc-800' : 'border-zinc-200'}`}>
            <span className="text-xs font-mono text-zinc-400">{fileName}</span>
            <button onClick={copyContent} className="text-zinc-500 hover:text-zinc-300 transition-colors">
              {copied ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
            </button>
          </div>
          {isBinary(blobContent) ? (
            <p className="px-4 py-8 text-sm text-zinc-500 text-center">{t('code.binaryFile')}</p>
          ) : (
            <pre className={`text-xs font-mono overflow-x-auto p-4 leading-5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {blobContent.split('\n').map((line, i) => (
                <div key={i} className="flex gap-4">
                  <span className="select-none w-8 text-right text-zinc-600 flex-shrink-0">{i + 1}</span>
                  <span className="flex-1">{line}</span>
                </div>
              ))}
            </pre>
          )}
        </GlassCard>
      ) : entries.length === 0 ? (
        <GlassCard padding="lg" className="text-center py-12">
          <p className="text-sm text-zinc-500">{t('code.emptyRepo')}</p>
        </GlassCard>
      ) : (
        /* Tree listing */
        <GlassCard padding="sm" className="overflow-hidden divide-y divide-zinc-800/30">
          {/* Back entry when in subdir */}
          {path && (
            <button
              onClick={() => {
                const parentPath = path.includes('/') ? path.split('/').slice(0, -1).join('/') : '';
                navigate2(parentPath, true);
              }}
              className={`w-full flex items-center gap-3 px-4 py-2.5 text-left text-sm hover:bg-white/5 transition-colors ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}
            >
              <Folder size={15} className="text-zinc-500" />
              <span className="font-mono">..</span>
            </button>
          )}
          {entries.map((entry) => (
            <button
              key={entry.path}
              onClick={() => handleEntry(entry)}
              className={`w-full flex items-center gap-3 px-4 py-2.5 text-left hover:bg-white/5 transition-colors ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
            >
              {entry.type === 'tree' ? (
                <Folder size={15} className="text-blue-400 flex-shrink-0" />
              ) : (
                <FileCode size={15} className="text-zinc-500 flex-shrink-0" />
              )}
              <span className="flex-1 text-sm font-mono truncate">{entry.name}</span>
              {entry.type === 'blob' && entry.size != null && entry.size > 0 && (
                <span className="text-xs text-zinc-600 flex-shrink-0">
                  {entry.size < 1024 ? `${entry.size} B` : `${(entry.size / 1024).toFixed(1)} KB`}
                </span>
              )}
            </button>
          ))}
        </GlassCard>
      )}
    </div>
  );
}
