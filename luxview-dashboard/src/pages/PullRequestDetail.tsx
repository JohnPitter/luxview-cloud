import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  ArrowLeft, GitMerge, XCircle, GitPullRequest,
  GitCommit, FileCode2, MessageSquare, Loader2, Trash2,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { useAuthStore } from '../stores/auth.store';
import { pullRequestsApi, type PullRequest, type PRCommit, type PRFileDiff, type PRComment } from '../api/repositories';

type Tab = 'commits' | 'diff' | 'comments';

function StatusBadge({ status }: { status: PullRequest['status'] }) {
  const { t } = useTranslation();
  const config = {
    open:   { icon: <GitPullRequest size={12} />, cls: 'text-emerald-400 bg-emerald-400/10 border-emerald-400/20' },
    merged: { icon: <GitMerge size={12} />,       cls: 'text-purple-400 bg-purple-400/10 border-purple-400/20' },
    closed: { icon: <XCircle size={12} />,         cls: 'text-zinc-500 bg-zinc-500/10 border-zinc-500/20' },
  }[status];
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full border text-xs font-medium ${config.cls}`}>
      {config.icon} {t(`pr.status.${status}`)}
    </span>
  );
}

export function PullRequestDetail() {
  const { repoId, number } = useParams<{ repoId: string; number: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);
  const currentUser = useAuthStore((s) => s.user);

  const [pr, setPr] = useState<PullRequest | null>(null);
  const [tab, setTab] = useState<Tab>('commits');
  const [commits, setCommits] = useState<PRCommit[]>([]);
  const [diffs, setDiffs] = useState<PRFileDiff[]>([]);
  const [comments, setComments] = useState<PRComment[]>([]);
  const [loadingTab, setLoadingTab] = useState(false);
  const [merging, setMerging] = useState(false);
  const [closing, setClosing] = useState(false);
  const [commentBody, setCommentBody] = useState('');
  const [postingComment, setPostingComment] = useState(false);
  const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set());

  const prNumber = parseInt(number ?? '0', 10);

  useEffect(() => {
    if (!repoId || !prNumber) return;
    pullRequestsApi.get(repoId, prNumber).then(setPr).catch(() => {
      addNotification({ type: 'error', title: t('pr.notFound') });
      navigate(-1);
    });
  }, [repoId, prNumber, addNotification, t, navigate]);

  const loadTab = useCallback(async (t: Tab) => {
    if (!repoId || !prNumber) return;
    setLoadingTab(true);
    try {
      if (t === 'commits') setCommits(await pullRequestsApi.commits(repoId, prNumber));
      if (t === 'diff') setDiffs(await pullRequestsApi.diff(repoId, prNumber));
      if (t === 'comments') setComments(await pullRequestsApi.listComments(repoId, prNumber));
    } finally {
      setLoadingTab(false);
    }
  }, [repoId, prNumber]);

  useEffect(() => { loadTab(tab); }, [tab, loadTab]);

  async function handleMerge() {
    if (!repoId || !prNumber) return;
    setMerging(true);
    try {
      const updated = await pullRequestsApi.merge(repoId, prNumber);
      setPr(updated);
      addNotification({ type: 'success', title: t('pr.merged') });
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('pr.mergeFailed') });
    } finally {
      setMerging(false);
    }
  }

  async function handleClose() {
    if (!repoId || !prNumber) return;
    setClosing(true);
    try {
      const updated = await pullRequestsApi.close(repoId, prNumber);
      setPr(updated);
      addNotification({ type: 'success', title: t('pr.closed') });
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('pr.closeFailed') });
    } finally {
      setClosing(false);
    }
  }

  async function handleComment() {
    if (!repoId || !prNumber || !commentBody.trim()) return;
    setPostingComment(true);
    try {
      const c = await pullRequestsApi.addComment(repoId, prNumber, commentBody);
      setComments((prev) => [...prev, c]);
      setCommentBody('');
    } catch {
      addNotification({ type: 'error', title: t('pr.commentFailed') });
    } finally {
      setPostingComment(false);
    }
  }

  async function handleDeleteComment(commentId: string) {
    if (!repoId || !prNumber) return;
    try {
      await pullRequestsApi.deleteComment(repoId, prNumber, commentId);
      setComments((prev) => prev.filter((c) => c.id !== commentId));
    } catch {
      addNotification({ type: 'error', title: t('pr.commentDeleteFailed') });
    }
  }

  function toggleFile(path: string) {
    setExpandedFiles((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  }

  if (!pr) {
    return (
      <div className="flex justify-center py-24">
        <Loader2 className="animate-spin text-zinc-400" size={24} />
      </div>
    );
  }

  const TABS: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: 'commits', label: t('pr.tabCommits'), icon: <GitCommit size={14} /> },
    { key: 'diff',    label: t('pr.tabDiff'),    icon: <FileCode2 size={14} /> },
    { key: 'comments',label: t('pr.tabComments'),icon: <MessageSquare size={14} /> },
  ];

  return (
    <div className="animate-fade-in space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-4">
          <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}/pulls`)} icon={<ArrowLeft size={16} />}>
            {t('common.back')}
          </PillButton>
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                #{pr.number} {pr.title}
              </h1>
              <StatusBadge status={pr.status} />
            </div>
            <p className="text-xs text-zinc-500 font-mono mt-1">
              {pr.headBranch} → {pr.baseBranch}
            </p>
            {pr.description && (
              <p className="text-sm text-zinc-400 mt-2 max-w-2xl">{pr.description}</p>
            )}
          </div>
        </div>

        {pr.status === 'open' && (
          <div className="flex gap-2 flex-shrink-0">
            <PillButton
              variant="ghost"
              size="sm"
              onClick={handleClose}
              disabled={closing}
              icon={closing ? <Loader2 size={13} className="animate-spin" /> : <XCircle size={13} className="text-red-400" />}
            >
              <span className="text-red-400">{t('pr.close')}</span>
            </PillButton>
            <PillButton
              variant="primary"
              size="sm"
              onClick={handleMerge}
              disabled={merging}
              icon={merging ? <Loader2 size={13} className="animate-spin" /> : <GitMerge size={13} />}
            >
              {t('pr.merge')}
            </PillButton>
          </div>
        )}
      </div>

      {/* Merge commit info */}
      {pr.mergeCommit && (
        <GlassCard padding="sm" className="border border-purple-500/20 bg-purple-500/5">
          <p className="text-xs text-purple-400 font-mono">{t('pr.mergeCommitLabel')}: {pr.mergeCommit.slice(0, 8)}</p>
        </GlassCard>
      )}

      {/* Tabs */}
      <div className="flex gap-1 border-b border-zinc-800/50 pb-0">
        {TABS.map(({ key, label, icon }) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors -mb-px ${
              tab === key
                ? 'border-amber-400 text-amber-400'
                : 'border-transparent ' + (isDark ? 'text-zinc-500 hover:text-zinc-300' : 'text-zinc-400 hover:text-zinc-700')
            }`}
          >
            {icon} {label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {loadingTab ? (
        <div className="flex justify-center py-12">
          <Loader2 className="animate-spin text-zinc-400" size={20} />
        </div>
      ) : tab === 'commits' ? (
        <div className="space-y-2">
          {commits.length === 0 ? (
            <p className="text-sm text-zinc-500">{t('pr.noCommits')}</p>
          ) : commits.map((c) => (
            <GlassCard key={c.sha} padding="sm">
              <div className="flex items-center gap-3">
                <GitCommit size={14} className="text-zinc-500 flex-shrink-0" />
                <div className="flex-1 min-w-0">
                  <p className={`text-sm truncate ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{c.message}</p>
                  <p className="text-xs text-zinc-500 font-mono mt-0.5">{c.sha.slice(0, 8)} · {c.author}</p>
                </div>
              </div>
            </GlassCard>
          ))}
        </div>
      ) : tab === 'diff' ? (
        <div className="space-y-2">
          {diffs.length === 0 ? (
            <p className="text-sm text-zinc-500">{t('pr.noDiff')}</p>
          ) : diffs.map((f) => (
            <GlassCard key={f.path} padding="sm" className="overflow-hidden">
              <button
                onClick={() => toggleFile(f.path)}
                className="w-full flex items-center justify-between gap-3 text-left"
              >
                <span className={`text-xs font-mono truncate ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{f.path}</span>
                <div className="flex items-center gap-2 flex-shrink-0">
                  <span className="text-xs text-emerald-400">+{f.additions}</span>
                  <span className="text-xs text-red-400">-{f.deletions}</span>
                </div>
              </button>
              {expandedFiles.has(f.path) && f.patch && (
                <pre className={`mt-3 text-xs font-mono overflow-x-auto p-2 rounded-lg whitespace-pre ${
                  isDark ? 'bg-zinc-900/70 text-zinc-300' : 'bg-zinc-100 text-zinc-700'
                }`}>
                  {f.patch.split('\n').map((line, i) => (
                    <div key={i} className={
                      line.startsWith('+') && !line.startsWith('+++') ? 'text-emerald-400' :
                      line.startsWith('-') && !line.startsWith('---') ? 'text-red-400' :
                      line.startsWith('@@') ? 'text-blue-400' : ''
                    }>{line}</div>
                  ))}
                </pre>
              )}
            </GlassCard>
          ))}
        </div>
      ) : (
        /* Comments tab */
        <div className="space-y-3">
          {comments.length === 0 && (
            <p className="text-sm text-zinc-500">{t('pr.noComments')}</p>
          )}
          {comments.map((c) => (
            <GlassCard key={c.id} padding="md">
              <div className="flex items-start justify-between gap-2">
                <div className="flex items-center gap-2 mb-2">
                  {c.author?.avatarUrl && (
                    <img src={c.author.avatarUrl} alt="" className="w-6 h-6 rounded-full" />
                  )}
                  <span className={`text-xs font-semibold ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                    {c.author?.username ?? t('pr.unknownAuthor')}
                  </span>
                  <span className="text-xs text-zinc-500">{new Date(c.createdAt).toLocaleDateString()}</span>
                </div>
                {currentUser && c.authorId === currentUser.id && (
                  <button onClick={() => handleDeleteComment(c.id)} className="text-zinc-600 hover:text-red-400 transition-colors">
                    <Trash2 size={13} />
                  </button>
                )}
              </div>
              <p className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{c.body}</p>
            </GlassCard>
          ))}

          {/* Add comment */}
          <GlassCard padding="md" className="space-y-3">
            <textarea
              value={commentBody}
              onChange={(e) => setCommentBody(e.target.value)}
              placeholder={t('pr.addCommentPlaceholder')}
              rows={3}
              className={`w-full px-3 py-2 text-sm rounded-lg border resize-none ${
                isDark
                  ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600'
                  : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
              }`}
            />
            <PillButton
              variant="primary"
              size="sm"
              onClick={handleComment}
              disabled={postingComment || !commentBody.trim()}
              icon={postingComment ? <Loader2 size={13} className="animate-spin" /> : undefined}
            >
              {t('pr.addComment')}
            </PillButton>
          </GlassCard>
        </div>
      )}
    </div>
  );
}
