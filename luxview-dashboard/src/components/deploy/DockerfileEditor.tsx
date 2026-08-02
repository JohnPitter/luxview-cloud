import { useEffect, useState } from 'react';
import { FileCode2, Loader2, Save, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { ConfirmDialog } from '../common/ConfirmDialog';
import { useNotificationsStore } from '../../stores/notifications.store';
import { analyzeApi } from '../../api/analyze';

interface DockerfileEditorProps {
  appId: string;
  savedContent?: string | null;
  isDark: boolean;
  onSaved: () => void;
}

export function DockerfileEditor({ appId, savedContent, isDark, onSaved }: DockerfileEditorProps) {
  const { t } = useTranslation();
  const addNotification = useNotificationsStore((s) => s.add);
  const [content, setContent] = useState(savedContent ?? '');
  const [lastSavedContent, setLastSavedContent] = useState(savedContent ?? '');
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  useEffect(() => {
    if (content === lastSavedContent) {
      const nextContent = savedContent ?? '';
      setContent(nextContent);
      setLastSavedContent(nextContent);
    }
  }, [savedContent]);

  const hasSavedDockerfile = lastSavedContent.trim().length > 0;
  const hasUnsavedChanges = content !== lastSavedContent;

  const handleSave = async () => {
    if (!content.trim() || !hasUnsavedChanges) return;
    setSaving(true);
    try {
      await analyzeApi.saveDockerfile(appId, content);
      setLastSavedContent(content);
      addNotification({ type: 'success', title: t('app.notifications.dockerfileSaved') });
      onSaved();
    } catch {
      addNotification({ type: 'error', title: t('app.notifications.dockerfileSaveFailed') });
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await analyzeApi.deleteDockerfile(appId);
      setContent('');
      setLastSavedContent('');
      setShowDeleteDialog(false);
      addNotification({ type: 'success', title: t('app.notifications.dockerfileDeleted') });
      onSaved();
    } catch {
      addNotification({ type: 'error', title: t('app.notifications.dockerfileDeleteFailed') });
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      <GlassCard padding="none">
        <div className={`flex items-start justify-between gap-4 p-6 border-b ${isDark ? 'border-zinc-800/60' : 'border-zinc-200/70'}`}>
          <div className="flex items-start gap-3">
            <div className={`flex items-center justify-center w-10 h-10 rounded-xl ${isDark ? 'bg-amber-400/10 text-amber-400' : 'bg-amber-50 text-amber-600'}`}>
              <FileCode2 size={20} />
            </div>
            <div>
              <h2 className={`text-base font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                {t('app.dockerfile.title')}
              </h2>
              <p className="text-xs text-zinc-500 mt-1 max-w-2xl">
                {t('app.dockerfile.description')}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {hasUnsavedChanges && (
              <span className="text-[11px] text-amber-400 hidden sm:inline">
                {t('app.dockerfile.unsaved')}
              </span>
            )}
            {!hasUnsavedChanges && hasSavedDockerfile && (
              <span className="text-[11px] text-emerald-400 hidden sm:inline">
                {t('app.dockerfile.saved')}
              </span>
            )}
          </div>
        </div>

        <div className="p-6">
          {!hasSavedDockerfile && (
            <div className={`mb-4 rounded-xl border px-4 py-3 text-xs ${isDark ? 'border-zinc-800 bg-zinc-950/40 text-zinc-400' : 'border-zinc-200 bg-zinc-50 text-zinc-500'}`}>
              {t('app.dockerfile.empty')}
            </div>
          )}
          <textarea
            value={content}
            onChange={(event) => setContent(event.target.value)}
            placeholder={t('app.dockerfile.placeholder')}
            spellCheck={false}
            aria-label={t('app.dockerfile.title')}
            className={`w-full min-h-[520px] resize-y rounded-xl border p-4 font-mono text-xs leading-6 outline-none transition-colors focus:ring-2 focus:ring-amber-400/30 ${isDark ? 'bg-zinc-950/70 border-zinc-800 text-zinc-200 placeholder:text-zinc-700 focus:border-amber-400/50' : 'bg-zinc-50 border-zinc-200 text-zinc-800 placeholder:text-zinc-400 focus:border-amber-500/50'}`}
          />
        </div>

        <div className={`flex flex-wrap items-center justify-between gap-3 px-6 py-4 border-t ${isDark ? 'border-zinc-800/60' : 'border-zinc-200/70'}`}>
          <p className="text-[11px] text-zinc-500">
            {t('app.dockerfile.deployHint')}
          </p>
          <div className="flex items-center gap-2">
            {hasSavedDockerfile && (
              <PillButton
                variant="danger"
                size="sm"
                onClick={() => setShowDeleteDialog(true)}
                disabled={saving || deleting}
                icon={deleting ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
              >
                {t('app.dockerfile.delete')}
              </PillButton>
            )}
            <PillButton
              variant="primary"
              size="sm"
              onClick={handleSave}
              disabled={saving || deleting || !content.trim() || !hasUnsavedChanges}
              icon={saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
            >
              {saving ? t('common.saving') : t('app.dockerfile.save')}
            </PillButton>
          </div>
        </div>
      </GlassCard>

      <ConfirmDialog
        open={showDeleteDialog}
        title={t('app.dockerfile.deleteTitle')}
        message={t('app.dockerfile.deleteMessage')}
        confirmLabel={t('app.dockerfile.deleteConfirm')}
        onConfirm={handleDelete}
        onCancel={() => setShowDeleteDialog(false)}
        loading={deleting}
      />
    </div>
  );
}
