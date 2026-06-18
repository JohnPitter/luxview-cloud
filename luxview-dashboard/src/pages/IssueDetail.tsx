import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, CircleDot, CheckCircle2, Loader2, Trash2 } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { Markdown } from '../components/common/Markdown';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { useAuthStore } from '../stores/auth.store';
import { issuesApi, type Issue, type IssueComment } from '../api/issues';

export function IssueDetail() {
  const { repoId, number } = useParams<{ repoId: string; number: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);
  const currentUser = useAuthStore((s) => s.user);

  const issueNumber = parseInt(number ?? '0', 10);
  const [issue, setIssue] = useState<Issue | null>(null);
  const [comments, setComments] = useState<IssueComment[]>([]);
  const [commentBody, setCommentBody] = useState('');
  const [posting, setPosting] = useState(false);
  const [updatingStatus, setUpdatingStatus] = useState(false);

  const load = useCallback(async () => {
    if (!repoId || !issueNumber) return;
    try {
      const [i, c] = await Promise.all([
        issuesApi.get(repoId, issueNumber),
        issuesApi.listComments(repoId, issueNumber),
      ]);
      setIssue(i);
      setComments(c);
    } catch {
      addNotification({ type: 'error', title: t('issues.notFound') });
      navigate(`/dashboard/repositories/${repoId}/issues`);
    }
  }, [repoId, issueNumber, addNotification, t, navigate]);

  useEffect(() => { load(); }, [load]);

  async function toggleStatus() {
    if (!repoId || !issue) return;
    setUpdatingStatus(true);
    try {
      const next = issue.status === 'open' ? 'closed' : 'open';
      const updated = await issuesApi.setStatus(repoId, issue.number, next);
      setIssue(updated);
      addNotification({ type: 'success', title: t('issues.statusUpdated') });
    } catch {
      addNotification({ type: 'error', title: t('issues.statusFailed') });
    } finally {
      setUpdatingStatus(false);
    }
  }

  async function handleComment() {
    if (!repoId || !issueNumber || !commentBody.trim()) return;
    setPosting(true);
    try {
      const c = await issuesApi.addComment(repoId, issueNumber, commentBody);
      setComments((prev) => [...prev, c]);
      setCommentBody('');
    } catch {
      addNotification({ type: 'error', title: t('issues.commentFailed') });
    } finally {
      setPosting(false);
    }
  }

  async function handleDeleteComment(id: string) {
    if (!repoId || !issueNumber) return;
    try {
      await issuesApi.deleteComment(repoId, issueNumber, id);
      setComments((prev) => prev.filter((c) => c.id !== id));
    } catch {
      addNotification({ type: 'error', title: t('issues.commentDeleteFailed') });
    }
  }

  if (!issue) {
    return <div className="flex justify-center py-24"><Loader2 className="animate-spin text-zinc-400" size={24} /></div>;
  }

  const open = issue.status === 'open';

  return (
    <div className="animate-fade-in space-y-5">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-4">
          <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}/issues`)} icon={<ArrowLeft size={16} />}>
            {t('common.back')}
          </PillButton>
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                {issue.title} <span className="text-zinc-500 font-normal">#{issue.number}</span>
              </h1>
              <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full border text-xs font-medium ${
                open ? 'text-emerald-400 bg-emerald-400/10 border-emerald-400/20' : 'text-purple-400 bg-purple-400/10 border-purple-400/20'
              }`}>
                {open ? <CircleDot size={12} /> : <CheckCircle2 size={12} />}
                {open ? t('issues.statusOpen') : t('issues.statusClosed')}
              </span>
            </div>
            <p className="text-xs text-zinc-500 mt-1">
              {issue.author?.username ?? t('issues.unknownAuthor')} {t('issues.openedBy')} · {new Date(issue.createdAt).toLocaleDateString()}
            </p>
            {issue.labels && issue.labels.length > 0 && (
              <div className="flex flex-wrap gap-2 mt-2">
                {issue.labels.map((l) => (
                  <span key={l.id} className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium border"
                    style={{ color: l.color, borderColor: l.color + '66', backgroundColor: l.color + '1a' }}>
                    {l.name}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>
        <PillButton
          variant="ghost" size="sm"
          onClick={toggleStatus}
          disabled={updatingStatus}
          icon={updatingStatus ? <Loader2 size={13} className="animate-spin" /> : (open ? <CheckCircle2 size={13} /> : <CircleDot size={13} />)}
        >
          {open ? t('issues.close') : t('issues.reopen')}
        </PillButton>
      </div>

      {issue.body && (
        <GlassCard padding="md">
          <Markdown>{issue.body}</Markdown>
        </GlassCard>
      )}

      <div className="space-y-3">
        {comments.map((c) => (
          <GlassCard key={c.id} padding="md">
            <div className="flex items-start justify-between gap-2 mb-2">
              <div className="flex items-center gap-2">
                {c.author?.avatarUrl && <img src={c.author.avatarUrl} alt="" className="w-6 h-6 rounded-full" />}
                <span className={`text-xs font-semibold ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{c.author?.username ?? t('issues.unknownAuthor')}</span>
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
        {comments.length === 0 && <p className="text-sm text-zinc-500">{t('issues.noComments')}</p>}

        <GlassCard padding="md" className="space-y-3">
          <textarea
            value={commentBody}
            onChange={(e) => setCommentBody(e.target.value)}
            placeholder={t('issues.addCommentPlaceholder')}
            rows={3}
            className={`w-full px-3 py-2 text-sm rounded-lg border resize-none ${isDark ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600' : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'}`}
          />
          <PillButton variant="primary" size="sm" onClick={handleComment} disabled={posting || !commentBody.trim()} icon={posting ? <Loader2 size={13} className="animate-spin" /> : undefined}>
            {t('issues.addComment')}
          </PillButton>
        </GlassCard>
      </div>
    </div>
  );
}
