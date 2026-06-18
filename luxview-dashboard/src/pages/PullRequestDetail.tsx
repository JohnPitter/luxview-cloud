import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  ArrowLeft, GitMerge, XCircle, GitPullRequest,
  GitCommit, FileCode2, MessageSquare, Loader2, Trash2,
  Check, CheckCircle2, CircleDot, Clock, ShieldQuestion, Plus,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { Markdown } from '../components/common/Markdown';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { useAuthStore } from '../stores/auth.store';
import {
  pullRequestsApi,
  type PullRequest, type PRCommit, type PRFileDiff, type PRComment,
  type PRReview, type ReviewComment, type StatusCheck, type ReviewState, type MergeStrategy,
} from '../api/repositories';

type Tab = 'commits' | 'diff' | 'comments' | 'review';

interface DiffRow {
  kind: 'meta' | 'hunk' | 'add' | 'del' | 'ctx';
  text: string;
  newLine?: number;
  oldLine?: number;
}

function parsePatch(patch: string): DiffRow[] {
  const rows: DiffRow[] = [];
  let oldLine = 0, newLine = 0;
  for (const line of patch.split('\n')) {
    if (line.startsWith('@@')) {
      const m = line.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (m) { oldLine = parseInt(m[1], 10); newLine = parseInt(m[2], 10); }
      rows.push({ kind: 'hunk', text: line });
    } else if (/^(diff |index |---|\+\+\+|new file|deleted file|similarity|rename )/.test(line)) {
      rows.push({ kind: 'meta', text: line });
    } else if (line.startsWith('+')) {
      rows.push({ kind: 'add', text: line, newLine }); newLine++;
    } else if (line.startsWith('-')) {
      rows.push({ kind: 'del', text: line, oldLine }); oldLine++;
    } else {
      rows.push({ kind: 'ctx', text: line, newLine, oldLine }); newLine++; oldLine++;
    }
  }
  return rows;
}

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
  const [reviews, setReviews] = useState<PRReview[]>([]);
  const [reviewComments, setReviewComments] = useState<ReviewComment[]>([]);
  const [checks, setChecks] = useState<StatusCheck[]>([]);
  const [loadingTab, setLoadingTab] = useState(false);
  const [merging, setMerging] = useState(false);
  const [closing, setClosing] = useState(false);
  const [mergeStrategy, setMergeStrategy] = useState<MergeStrategy>('merge');
  const [commentBody, setCommentBody] = useState('');
  const [postingComment, setPostingComment] = useState(false);
  const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set());

  // Inline review comment composer
  const [activeLine, setActiveLine] = useState<string | null>(null);
  const [inlineBody, setInlineBody] = useState('');

  // Review submission
  const [reviewBody, setReviewBody] = useState('');
  const [submittingReview, setSubmittingReview] = useState(false);

  const prNumber = parseInt(number ?? '0', 10);

  const refreshChecks = useCallback(() => {
    if (!repoId || !prNumber) return;
    pullRequestsApi.checks(repoId, prNumber).then(setChecks).catch(() => {});
  }, [repoId, prNumber]);

  useEffect(() => {
    if (!repoId || !prNumber) return;
    pullRequestsApi.get(repoId, prNumber).then((p) => {
      setPr(p);
      refreshChecks();
    }).catch(() => {
      addNotification({ type: 'error', title: t('pr.notFound') });
      navigate(-1);
    });
  }, [repoId, prNumber, addNotification, t, navigate, refreshChecks]);

  const loadReviewComments = useCallback(async () => {
    if (!repoId || !prNumber) return;
    setReviewComments(await pullRequestsApi.listReviewComments(repoId, prNumber));
  }, [repoId, prNumber]);

  const loadTab = useCallback(async (which: Tab) => {
    if (!repoId || !prNumber) return;
    setLoadingTab(true);
    try {
      if (which === 'commits') setCommits(await pullRequestsApi.commits(repoId, prNumber));
      if (which === 'diff') {
        setDiffs(await pullRequestsApi.diff(repoId, prNumber));
        await loadReviewComments();
      }
      if (which === 'comments') setComments(await pullRequestsApi.listComments(repoId, prNumber));
      if (which === 'review') setReviews(await pullRequestsApi.listReviews(repoId, prNumber));
    } finally {
      setLoadingTab(false);
    }
  }, [repoId, prNumber, loadReviewComments]);

  useEffect(() => { loadTab(tab); }, [tab, loadTab]);

  async function handleMerge() {
    if (!repoId || !prNumber) return;
    setMerging(true);
    try {
      const updated = await pullRequestsApi.merge(repoId, prNumber, mergeStrategy);
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

  async function submitReview(state: ReviewState) {
    if (!repoId || !prNumber) return;
    setSubmittingReview(true);
    try {
      await pullRequestsApi.addReview(repoId, prNumber, state, reviewBody);
      setReviewBody('');
      setReviews(await pullRequestsApi.listReviews(repoId, prNumber));
      addNotification({ type: 'success', title: t('pr.review.submitted') });
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('pr.review.failed') });
    } finally {
      setSubmittingReview(false);
    }
  }

  async function postInlineComment(path: string, line: number, side: 'old' | 'new') {
    if (!repoId || !prNumber || !inlineBody.trim()) return;
    try {
      await pullRequestsApi.addReviewComment(repoId, prNumber, { path, line, side, body: inlineBody });
      setInlineBody('');
      setActiveLine(null);
      await loadReviewComments();
    } catch {
      addNotification({ type: 'error', title: t('pr.inline.failed') });
    }
  }

  async function toggleResolve(c: ReviewComment) {
    if (!repoId || !prNumber) return;
    try {
      await pullRequestsApi.resolveReviewComment(repoId, prNumber, c.id, !c.resolved);
      await loadReviewComments();
    } catch {
      addNotification({ type: 'error', title: t('pr.inline.failed') });
    }
  }

  async function deleteInline(c: ReviewComment) {
    if (!repoId || !prNumber) return;
    try {
      await pullRequestsApi.deleteReviewComment(repoId, prNumber, c.id);
      await loadReviewComments();
    } catch {
      addNotification({ type: 'error', title: t('pr.inline.failed') });
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
    { key: 'review',  label: t('pr.tabReview'),  icon: <ShieldQuestion size={14} /> },
    { key: 'comments',label: t('pr.tabComments'),icon: <MessageSquare size={14} /> },
  ];

  const commentsByLine = (path: string, line: number, side: string) =>
    reviewComments.filter((c) => c.path === path && c.line === line && c.side === side);

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
              <div className="mt-2 max-w-2xl"><Markdown>{pr.description}</Markdown></div>
            )}
          </div>
        </div>

        {pr.status === 'open' && (
          <div className="flex items-center gap-2 flex-shrink-0">
            <PillButton
              variant="ghost"
              size="sm"
              onClick={handleClose}
              disabled={closing}
              icon={closing ? <Loader2 size={13} className="animate-spin" /> : <XCircle size={13} className="text-red-400" />}
            >
              <span className="text-red-400">{t('pr.close')}</span>
            </PillButton>
            <select
              value={mergeStrategy}
              onChange={(e) => setMergeStrategy(e.target.value as MergeStrategy)}
              className={`px-2 py-1.5 text-xs rounded-lg border ${isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'}`}
            >
              <option value="merge">{t('pr.mergeMethod.merge')}</option>
              <option value="squash">{t('pr.mergeMethod.squash')}</option>
              <option value="rebase">{t('pr.mergeMethod.rebase')}</option>
            </select>
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

      {/* Status checks */}
      {pr.status === 'open' && checks.length > 0 && (
        <GlassCard padding="sm" className="space-y-1.5">
          <p className="text-xs font-semibold uppercase tracking-wider text-zinc-500 mb-1">{t('pr.checks.title')}</p>
          {checks.map((c) => {
            const ok = c.status === 'success';
            const failed = c.status === 'failed' || c.status === 'cancelled';
            return (
              <div key={c.runId} className="flex items-center gap-2 text-xs">
                {ok ? <CheckCircle2 size={13} className="text-emerald-400" />
                  : failed ? <XCircle size={13} className="text-red-400" />
                  : <Clock size={13} className="text-amber-400" />}
                <span className={isDark ? 'text-zinc-300' : 'text-zinc-700'}>{c.name}</span>
                <span className="text-zinc-500">· {c.status}</span>
              </div>
            );
          })}
        </GlassCard>
      )}

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
              <button onClick={() => toggleFile(f.path)} className="w-full flex items-center justify-between gap-3 text-left">
                <span className={`text-xs font-mono truncate ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{f.path}</span>
                <div className="flex items-center gap-2 flex-shrink-0">
                  <span className="text-xs text-emerald-400">+{f.additions}</span>
                  <span className="text-xs text-red-400">-{f.deletions}</span>
                </div>
              </button>
              {expandedFiles.has(f.path) && f.patch && (
                <div className={`mt-3 text-xs font-mono overflow-x-auto rounded-lg ${isDark ? 'bg-zinc-900/70' : 'bg-zinc-100'}`}>
                  {parsePatch(f.patch).map((row, i) => {
                    const canComment = pr.status === 'open' && (row.kind === 'add' || row.kind === 'del' || row.kind === 'ctx');
                    const side = row.kind === 'del' ? 'old' : 'new';
                    const lineNo = row.kind === 'del' ? row.oldLine : row.newLine;
                    const key = `${f.path}:${side}:${lineNo}`;
                    const rowComments = lineNo != null ? commentsByLine(f.path, lineNo, side) : [];
                    return (
                      <div key={i}>
                        <div className={`group flex items-start ${
                          row.kind === 'add' ? 'bg-emerald-500/10' :
                          row.kind === 'del' ? 'bg-red-500/10' :
                          row.kind === 'hunk' ? 'bg-blue-500/10' : ''
                        }`}>
                          <span className="select-none w-10 text-right pr-2 text-zinc-600 flex-shrink-0">{lineNo ?? ''}</span>
                          {canComment && (
                            <button
                              onClick={() => { setActiveLine(key); setInlineBody(''); }}
                              className="opacity-0 group-hover:opacity-100 text-amber-400 px-1 flex-shrink-0 transition-opacity"
                              title={t('pr.inline.add')}
                            >
                              <Plus size={12} />
                            </button>
                          )}
                          <span className={`flex-1 whitespace-pre pl-1 pr-3 ${
                            row.kind === 'add' ? 'text-emerald-400' :
                            row.kind === 'del' ? 'text-red-400' :
                            row.kind === 'hunk' ? 'text-blue-400' :
                            row.kind === 'meta' ? 'text-zinc-500' :
                            isDark ? 'text-zinc-300' : 'text-zinc-700'
                          }`}>{row.text || ' '}</span>
                        </div>

                        {/* existing inline comments */}
                        {rowComments.map((c) => (
                          <div key={c.id} className={`ml-10 my-1 mr-2 p-2 rounded-lg border ${isDark ? 'bg-zinc-800/60 border-white/10' : 'bg-white border-black/10'} ${c.resolved ? 'opacity-60' : ''}`}>
                            <div className="flex items-center justify-between gap-2 mb-1">
                              <span className="text-[11px] font-semibold text-zinc-400">{c.author?.username}</span>
                              <div className="flex items-center gap-2">
                                <button onClick={() => toggleResolve(c)} className="text-[11px] text-zinc-500 hover:text-amber-400 flex items-center gap-0.5">
                                  <Check size={11} /> {c.resolved ? t('pr.inline.unresolve') : t('pr.inline.resolve')}
                                </button>
                                {currentUser && c.authorId === currentUser.id && (
                                  <button onClick={() => deleteInline(c)} className="text-zinc-500 hover:text-red-400"><Trash2 size={11} /></button>
                                )}
                              </div>
                            </div>
                            <div className="font-sans"><Markdown>{c.body}</Markdown></div>
                          </div>
                        ))}

                        {/* inline composer */}
                        {activeLine === key && lineNo != null && (
                          <div className="ml-10 my-1 mr-2 space-y-2">
                            <textarea
                              value={inlineBody}
                              onChange={(e) => setInlineBody(e.target.value)}
                              placeholder={t('pr.inline.placeholder')}
                              rows={2}
                              autoFocus
                              className={`w-full px-2 py-1.5 text-xs rounded-lg border font-sans resize-none ${isDark ? 'bg-zinc-800 border-white/10 text-white' : 'bg-white border-black/10 text-zinc-900'}`}
                            />
                            <div className="flex gap-2">
                              <PillButton variant="ghost" size="sm" onClick={() => setActiveLine(null)}>{t('pr.inline.cancel')}</PillButton>
                              <PillButton variant="primary" size="sm" onClick={() => postInlineComment(f.path, lineNo, side)} disabled={!inlineBody.trim()}>
                                {t('pr.inline.comment')}
                              </PillButton>
                            </div>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </GlassCard>
          ))}
        </div>
      ) : tab === 'review' ? (
        <div className="space-y-3">
          {reviews.length === 0 && <p className="text-sm text-zinc-500">{t('pr.review.none')}</p>}
          {reviews.map((rv) => {
            const cfg = {
              approved: { icon: <CheckCircle2 size={14} className="text-emerald-400" />, label: t('pr.review.approved') },
              changes_requested: { icon: <XCircle size={14} className="text-red-400" />, label: t('pr.review.changesRequested') },
              commented: { icon: <CircleDot size={14} className="text-zinc-400" />, label: t('pr.review.commented') },
            }[rv.state];
            return (
              <GlassCard key={rv.id} padding="md">
                <div className="flex items-center gap-2 mb-1">
                  {rv.reviewer?.avatarUrl && <img src={rv.reviewer.avatarUrl} alt="" className="w-6 h-6 rounded-full" />}
                  {cfg.icon}
                  <span className={`text-xs font-semibold ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{rv.reviewer?.username}</span>
                  <span className="text-xs text-zinc-500">{cfg.label}</span>
                  <span className="text-xs text-zinc-600">· {new Date(rv.createdAt).toLocaleDateString()}</span>
                </div>
                {rv.body && <div className="mt-1"><Markdown>{rv.body}</Markdown></div>}
              </GlassCard>
            );
          })}

          {pr.status === 'open' && (
            <GlassCard padding="md" className="space-y-3">
              <p className="text-xs font-semibold uppercase tracking-wider text-zinc-500">{t('pr.tabReview')}</p>
              <textarea
                value={reviewBody}
                onChange={(e) => setReviewBody(e.target.value)}
                placeholder={t('pr.review.bodyPlaceholder')}
                rows={3}
                className={`w-full px-3 py-2 text-sm rounded-lg border resize-none ${isDark ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600' : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'}`}
              />
              <div className="flex flex-wrap gap-2">
                <PillButton variant="ghost" size="sm" onClick={() => submitReview('commented')} disabled={submittingReview} icon={<MessageSquare size={13} />}>
                  {t('pr.review.comment')}
                </PillButton>
                <PillButton variant="ghost" size="sm" onClick={() => submitReview('changes_requested')} disabled={submittingReview} icon={<XCircle size={13} className="text-red-400" />}>
                  <span className="text-red-400">{t('pr.review.requestChanges')}</span>
                </PillButton>
                <PillButton variant="primary" size="sm" onClick={() => submitReview('approved')} disabled={submittingReview} icon={submittingReview ? <Loader2 size={13} className="animate-spin" /> : <CheckCircle2 size={13} />}>
                  {t('pr.review.approve')}
                </PillButton>
              </div>
            </GlassCard>
          )}
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
              <Markdown>{c.body}</Markdown>
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
