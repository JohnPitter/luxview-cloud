import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  ArrowLeft,
  Plus,
  Trash2,
  Loader2,
  RefreshCw,
  Copy,
  Check,
  KeyRound,
  Mail,
  ExternalLink,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { ConfirmDialog } from '../components/common/ConfirmDialog';
import { useThemeStore } from '../stores/theme.store';
import { servicesApi, type Mailbox } from '../api/services';

export function EmailManager() {
  const { serviceId } = useParams<{ serviceId: string }>();
  const navigate = useNavigate();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const { t } = useTranslation();

  const [mailboxes, setMailboxes] = useState<Mailbox[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [newLocalPart, setNewLocalPart] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [createdPassword, setCreatedPassword] = useState<{ address: string; password: string } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Mailbox | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const fetchMailboxes = useCallback(async () => {
    if (!serviceId) return;
    setLoading(true);
    try {
      const data = await servicesApi.listMailboxes(serviceId);
      setMailboxes(data);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [serviceId]);

  useEffect(() => {
    fetchMailboxes();
  }, [fetchMailboxes]);

  const handleCreate = async () => {
    if (!serviceId || !newLocalPart.trim()) return;
    setCreating(true);
    try {
      const result = await servicesApi.createMailbox(serviceId, newLocalPart.trim().toLowerCase());
      setCreatedPassword({ address: result.address, password: result.password });
      setNewLocalPart('');
      setShowCreateForm(false);
      await fetchMailboxes();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create mailbox';
      alert(msg);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await servicesApi.deleteMailbox(deleteTarget.id);
      setDeleteTarget(null);
      await fetchMailboxes();
    } catch {
      // ignore
    }
  };

  const handleResetPassword = async (mailbox: Mailbox) => {
    try {
      const result = await servicesApi.resetMailboxPassword(mailbox.id);
      setCreatedPassword({ address: result.address, password: result.password });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to reset password';
      alert(msg);
    }
  };

  const copyToClipboard = async (text: string, id: string) => {
    await navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <button
            onClick={() => navigate('/dashboard/resources')}
            className={`p-2 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800' : 'hover:bg-zinc-100'}`}
          >
            <ArrowLeft size={20} className={isDark ? 'text-zinc-400' : 'text-zinc-600'} />
          </button>
          <div>
            <h1 className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
              {t('resources.email.title')}
            </h1>
            <p className={`text-sm ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
              {t('resources.email.subtitle')}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <a
            href="https://mail.luxview.cloud"
            target="_blank"
            rel="noopener noreferrer"
            className={`
              flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors
              ${isDark ? 'bg-zinc-800 text-cyan-400 hover:bg-zinc-700' : 'bg-zinc-100 text-cyan-600 hover:bg-zinc-200'}
            `}
          >
            <ExternalLink size={14} />
            Webmail
          </a>
          <PillButton variant="ghost" size="sm" onClick={fetchMailboxes}>
            <RefreshCw size={14} />
          </PillButton>
          <PillButton variant="primary" size="sm" onClick={() => setShowCreateForm(true)}>
            <Plus size={14} />
            {t('resources.email.addMailbox')}
          </PillButton>
        </div>
      </div>

      {/* Password reveal banner */}
      {createdPassword && (
        <GlassCard>
          <div className="flex items-start justify-between">
            <div className="space-y-2">
              <h3 className={`text-sm font-semibold ${isDark ? 'text-emerald-400' : 'text-emerald-600'}`}>
                {t('resources.email.passwordCreated')}
              </h3>
              <p className={`text-xs ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
                {t('resources.email.passwordWarning')}
              </p>
              <div className="grid grid-cols-2 gap-4 mt-3">
                <div>
                  <span className="text-[10px] text-zinc-500 uppercase tracking-wider">
                    {t('resources.email.address')}
                  </span>
                  <div className="flex items-center gap-2 mt-1">
                    <p className={`text-sm font-mono ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {createdPassword.address}
                    </p>
                    <button
                      onClick={() => copyToClipboard(createdPassword.address, 'addr')}
                      className="text-zinc-500 hover:text-amber-400 transition-colors"
                    >
                      {copiedId === 'addr' ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
                    </button>
                  </div>
                </div>
                <div>
                  <span className="text-[10px] text-zinc-500 uppercase tracking-wider">
                    {t('resources.email.password')}
                  </span>
                  <div className="flex items-center gap-2 mt-1">
                    <p className={`text-sm font-mono ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {createdPassword.password}
                    </p>
                    <button
                      onClick={() => copyToClipboard(createdPassword.password, 'pwd')}
                      className="text-zinc-500 hover:text-amber-400 transition-colors"
                    >
                      {copiedId === 'pwd' ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
                    </button>
                  </div>
                </div>
              </div>
            </div>
            <button
              onClick={() => setCreatedPassword(null)}
              className="text-zinc-500 hover:text-zinc-300 transition-colors"
            >
              &times;
            </button>
          </div>
        </GlassCard>
      )}

      {/* Create mailbox form */}
      {showCreateForm && (
        <GlassCard>
          <h3 className={`text-sm font-semibold mb-3 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            {t('resources.email.createTitle')}
          </h3>
          <div className="flex items-center gap-2">
            <input
              type="text"
              value={newLocalPart}
              onChange={(e) => setNewLocalPart(e.target.value.replace(/[^a-zA-Z0-9._-]/g, ''))}
              placeholder={t('resources.email.localPartPlaceholder')}
              className={`
                flex-1 px-3 py-2 rounded-lg text-sm font-mono
                ${isDark ? 'bg-zinc-900 border border-zinc-700 text-zinc-200 placeholder:text-zinc-600' : 'bg-white border border-zinc-300 text-zinc-800 placeholder:text-zinc-400'}
                focus:outline-none focus:ring-2 focus:ring-cyan-500/50
              `}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              autoFocus
            />
            <span className={`text-sm font-mono ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
              @app.luxview.cloud
            </span>
          </div>
          <div className="flex justify-end gap-2 mt-3">
            <PillButton variant="ghost" size="sm" onClick={() => { setShowCreateForm(false); setNewLocalPart(''); }}>
              {t('common.cancel')}
            </PillButton>
            <PillButton variant="primary" size="sm" onClick={handleCreate} disabled={creating || !newLocalPart.trim()}>
              {creating ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />}
              {t('resources.email.create')}
            </PillButton>
          </div>
        </GlassCard>
      )}

      {/* Mailbox list */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 size={24} className="animate-spin text-zinc-500" />
        </div>
      ) : mailboxes.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 space-y-3">
          <Mail size={48} className="text-zinc-600" />
          <p className={`text-sm ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
            {t('resources.email.empty')}
          </p>
          <PillButton variant="primary" size="sm" onClick={() => setShowCreateForm(true)}>
            <Plus size={14} />
            {t('resources.email.addMailbox')}
          </PillButton>
        </div>
      ) : (
        <div className="space-y-2">
          {mailboxes.map((mb) => (
            <GlassCard key={mb.id}>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className={`flex items-center justify-center w-9 h-9 rounded-lg ${isDark ? 'bg-cyan-500/10' : 'bg-cyan-50'}`}>
                    <Mail size={16} className="text-cyan-400" />
                  </div>
                  <div>
                    <p className={`text-sm font-mono font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {mb.address}
                    </p>
                    <p className="text-[11px] text-zinc-500">
                      {new Date(mb.created_at).toLocaleDateString()}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => copyToClipboard(mb.address, mb.id)}
                    className="p-2 text-zinc-500 hover:text-amber-400 transition-colors"
                    title={t('services.card.copyToClipboard')}
                  >
                    {copiedId === mb.id ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                  </button>
                  <button
                    onClick={() => handleResetPassword(mb)}
                    className="p-2 text-zinc-500 hover:text-cyan-400 transition-colors"
                    title={t('resources.email.resetPassword')}
                  >
                    <KeyRound size={14} />
                  </button>
                  <button
                    onClick={() => setDeleteTarget(mb)}
                    className="p-2 text-zinc-500 hover:text-red-400 transition-colors"
                    title={t('common.delete')}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            </GlassCard>
          ))}
        </div>
      )}

      {/* Delete confirm dialog */}
      <ConfirmDialog
        open={!!deleteTarget}
        title={t('resources.email.deleteTitle')}
        message={t('resources.email.deleteMessage', { address: deleteTarget?.address })}
        confirmLabel={t('common.delete')}
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
        variant="danger"
      />
    </div>
  );
}
