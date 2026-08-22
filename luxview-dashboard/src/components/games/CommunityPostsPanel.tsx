import { useCallback, useEffect, useState } from 'react';
import { Loader2, Megaphone, Trash2 } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';
import { communityApi, type CommunityPost } from '../../api/community';

interface CommunityPostsPanelProps {
  appId: string;
}

function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  const delta = Math.max(0, Date.now() - then);
  const min = Math.floor(delta / 60_000);
  if (min < 1) return 'agora';
  if (min < 60) return `${min} min`;
  const hours = Math.floor(min / 60);
  if (hours < 24) return `${hours} h`;
  const days = Math.floor(hours / 24);
  return `${days} d`;
}

export function CommunityPostsPanel({ appId }: CommunityPostsPanelProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);
  const [posts, setPosts] = useState<CommunityPost[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');

  const load = useCallback(async () => {
    try {
      setPosts(await communityApi.listPosts(appId));
    } catch {
      addNotification({ type: 'error', title: 'Falha ao carregar postagens da comunidade' });
    } finally {
      setLoading(false);
    }
  }, [appId, addNotification]);

  useEffect(() => {
    void load();
  }, [load]);

  const publish = async () => {
    setSaving(true);
    try {
      const created = await communityApi.createPost(appId, title, body);
      setPosts((prev) => [created, ...prev]);
      setTitle('');
      setBody('');
      addNotification({ type: 'success', title: 'Postagem publicada na comunidade' });
    } catch {
      addNotification({ type: 'error', title: 'Não foi possível publicar' });
    } finally {
      setSaving(false);
    }
  };

  const remove = async (postId: string) => {
    try {
      await communityApi.deletePost(appId, postId);
      setPosts((prev) => prev.filter((p) => p.id !== postId));
    } catch {
      addNotification({ type: 'error', title: 'Não foi possível remover a postagem' });
    }
  };

  const muted = isDark ? 'text-zinc-400' : 'text-zinc-500';
  const inputCls = `w-full rounded-xl border px-3 py-2 text-sm outline-none ${
    isDark
      ? 'bg-zinc-950/60 border-zinc-800 text-zinc-100 placeholder:text-zinc-600'
      : 'bg-white border-zinc-200 text-zinc-900 placeholder:text-zinc-400'
  }`;

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <GlassCard>
        <div className="flex items-start gap-3 mb-4">
          <div className="rounded-xl bg-amber-400/15 p-2 text-amber-400">
            <Megaphone size={18} />
          </div>
          <div>
            <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              Postagens da comunidade
            </h3>
            <p className={`text-xs mt-1 ${muted}`}>
              Eventos, avisos e novidades deste servidor aparecem no launcher, na aba Comunidade,
              como um feed de postagens.
            </p>
          </div>
        </div>
        <div className="space-y-3">
          <input
            className={inputCls}
            maxLength={80}
            placeholder="Título do evento"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <textarea
            className={`${inputCls} min-h-[110px] resize-y`}
            maxLength={2000}
            placeholder="O que está acontecendo no servidor?"
            value={body}
            onChange={(e) => setBody(e.target.value)}
          />
          <div className="flex justify-end">
            <PillButton onClick={() => void publish()} disabled={saving || !title.trim() || !body.trim()}>
              {saving ? <Loader2 size={14} className="animate-spin" /> : null}
              Publicar
            </PillButton>
          </div>
        </div>
      </GlassCard>

      {loading ? (
        <div className={`flex items-center gap-2 text-sm ${muted}`}>
          <Loader2 size={14} className="animate-spin" /> Carregando postagens…
        </div>
      ) : posts.length === 0 ? (
        <p className={`text-sm ${muted}`}>Nenhuma postagem ainda. Publique o primeiro evento.</p>
      ) : (
        posts.map((post) => (
          <GlassCard key={post.id}>
            <div className="flex items-start justify-between gap-3">
              <div>
                <h4 className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                  {post.title}
                </h4>
                <p className={`text-xs mt-1 ${muted}`}>{relativeTime(post.createdAt)}</p>
              </div>
              <button
                type="button"
                className="rounded-lg p-1.5 text-zinc-500 hover:text-red-400 hover:bg-red-500/10"
                title="Remover"
                onClick={() => void remove(post.id)}
              >
                <Trash2 size={14} />
              </button>
            </div>
            <p className={`text-sm mt-3 whitespace-pre-wrap ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {post.body}
            </p>
          </GlassCard>
        ))
      )}
    </div>
  );
}
