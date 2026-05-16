import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Loader2 } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { repositoriesApi } from '../api/repositories';

export function NewRepository() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [name, setName] = useState('');
  const [defaultBranch, setDefaultBranch] = useState('main');
  const [visibility, setVisibility] = useState<'private' | 'public'>('private');
  const [creating, setCreating] = useState(false);

  const inputClass = `w-full px-3 py-2 text-sm rounded-lg border ${
    isDark
      ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600'
      : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
  }`;

  async function handleCreate() {
    if (!name.trim()) return;
    setCreating(true);
    try {
      const repo = await repositoriesApi.create({ name: name.trim(), defaultBranch, visibility });
      addNotification({ type: 'success', title: t('repo.new.created') });
      navigate(`/dashboard/new?source=luxview&repoId=${repo.id}`);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('repo.new.createFailed') });
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="animate-fade-in">
      <div className="flex items-center gap-4 mb-8">
        <PillButton variant="ghost" size="sm" onClick={() => navigate(-1)} icon={<ArrowLeft size={16} />}>
          {t('common.back')}
        </PillButton>
        <div>
          <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('repo.new.title')}
          </h1>
          <p className="text-sm text-zinc-500 mt-0.5">{t('repo.new.subtitle')}</p>
        </div>
      </div>

      <div className="max-w-lg mx-auto">
        <GlassCard padding="lg" className="space-y-4">
          <div>
            <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {t('repo.new.name')}
            </label>
            <input
              type="text"
              placeholder={t('repo.new.namePlaceholder')}
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={inputClass}
              autoFocus
            />
          </div>

          <div>
            <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {t('repo.new.defaultBranch')}
            </label>
            <input
              type="text"
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
              className={inputClass}
            />
          </div>

          <div>
            <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {t('repo.new.visibility')}
            </label>
            <select
              value={visibility}
              onChange={(e) => setVisibility(e.target.value as 'private' | 'public')}
              className={`px-3 py-2 text-sm rounded-lg border ${
                isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'
              }`}
            >
              <option value="private">{t('repo.new.private')}</option>
              <option value="public">{t('repo.new.public')}</option>
            </select>
          </div>

          <div className="pt-2">
            <PillButton
              variant="primary"
              size="md"
              icon={creating ? <Loader2 size={14} className="animate-spin" /> : undefined}
              onClick={handleCreate}
              disabled={creating || !name.trim()}
            >
              {t('repo.new.create')}
            </PillButton>
          </div>
        </GlassCard>
      </div>
    </div>
  );
}
