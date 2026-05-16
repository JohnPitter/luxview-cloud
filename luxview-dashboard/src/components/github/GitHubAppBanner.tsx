import { useTranslation } from 'react-i18next';
import { Github, ExternalLink, CheckCircle } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useAuthStore } from '../../stores/auth.store';
import { githubApi } from '../../api/github';

export function GitHubAppBanner() {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const user = useAuthStore((s) => s.user);

  if (!user) return null;

  if (user.appInstalled) {
    return (
      <GlassCard padding="sm" className="flex items-center gap-3 border-emerald-500/30 bg-emerald-500/5">
        <CheckCircle size={16} className="text-emerald-400 flex-shrink-0" />
        <span className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
          {t('github.app.installed')}
        </span>
      </GlassCard>
    );
  }

  return (
    <GlassCard padding="md" className="border-amber-500/30 bg-amber-500/5">
      <div className="flex items-start gap-4">
        <Github size={20} className="text-amber-400 flex-shrink-0 mt-0.5" />
        <div className="flex-1 min-w-0">
          <p className={`font-medium text-sm ${isDark ? 'text-white' : 'text-zinc-900'}`}>
            {t('github.app.installTitle')}
          </p>
          <p className="text-xs text-zinc-500 mt-0.5">{t('github.app.installDescription')}</p>
        </div>
        <PillButton
          variant="primary"
          size="sm"
          icon={<ExternalLink size={13} />}
          onClick={() => window.location.href = githubApi.getInstallUrl()}
        >
          {t('github.app.installButton')}
        </PillButton>
      </div>
    </GlassCard>
  );
}
