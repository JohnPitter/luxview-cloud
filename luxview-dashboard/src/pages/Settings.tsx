import { Settings as SettingsIcon, Moon, Sun, Bell, Monitor, Globe, RotateCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { GlassCard } from '../components/common/GlassCard';
import { PageTour, resetAllTours } from '../components/common/PageTour';
import { useThemeStore } from '../stores/theme.store';
import { useAuthStore } from '../stores/auth.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { languages } from '../i18n';
import { settingsTourSteps } from '../tours/settings';

export function Settings() {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const { theme, toggleTheme } = useThemeStore();
  const user = useAuthStore((s) => s.user);
  const addNotification = useNotificationsStore((s) => s.add);
  const { t, i18n } = useTranslation();

  const themeOptions = [
    { value: 'dark', label: t('settings.themeDark'), icon: Moon },
    { value: 'light', label: t('settings.themeLight'), icon: Sun },
  ] as const;

  const handleReplayTours = () => {
    resetAllTours();
    addNotification({ type: 'success', title: t('settings.toursReset') });
  };

  return (
    <div className="animate-fade-in max-w-2xl">
      <PageTour tourId="settings" steps={settingsTourSteps} autoStart />

      <div className="flex items-center gap-3 mb-8">
        <SettingsIcon size={24} className="text-amber-400" />
        <div>
          <h1
            className={`text-2xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            {t('settings.title')}
          </h1>
          <p className="text-sm text-zinc-500">{t('settings.subtitle')}</p>
        </div>
      </div>

      {/* Appearance */}
      <GlassCard className="mb-4" data-tour="theme-selector">
        <div className="flex items-center gap-3 mb-4">
          <Monitor size={18} className="text-zinc-400" />
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            {t('settings.appearance')}
          </h3>
        </div>
        <div className="flex gap-3">
          {themeOptions.map((opt) => {
            const Icon = opt.icon;
            const isActive = theme === opt.value;
            return (
              <button
                key={opt.value}
                onClick={isActive ? undefined : toggleTheme}
                className={`
                  flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-medium
                  transition-all duration-200
                  ${
                    isActive
                      ? 'bg-amber-400/10 text-amber-400 ring-1 ring-amber-400/30'
                      : isDark
                        ? 'bg-zinc-800/50 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
                        : 'bg-zinc-100 text-zinc-500 hover:bg-zinc-200 hover:text-zinc-700'
                  }
                `}
              >
                <Icon size={16} />
                {opt.label}
              </button>
            );
          })}
        </div>
      </GlassCard>

      {/* Language */}
      <GlassCard className="mb-4" data-tour="language-selector">
        <div className="flex items-center gap-3 mb-4">
          <Globe size={18} className="text-zinc-400" />
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            {t('settings.language')}
          </h3>
        </div>
        <p className="text-xs text-zinc-500 mb-3">{t('settings.selectLanguage')}</p>
        <div className="flex gap-3">
          {languages.map((lang) => {
            const isActive = i18n.language === lang.code || (i18n.language.startsWith('pt') && lang.code === 'pt-BR');
            return (
              <button
                key={lang.code}
                onClick={() => i18n.changeLanguage(lang.code)}
                className={`
                  flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-medium
                  transition-all duration-200
                  ${
                    isActive
                      ? 'bg-amber-400/10 text-amber-400 ring-1 ring-amber-400/30'
                      : isDark
                        ? 'bg-zinc-800/50 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
                        : 'bg-zinc-100 text-zinc-500 hover:bg-zinc-200 hover:text-zinc-700'
                  }
                `}
              >
                <span className="text-base">{lang.flag}</span>
                {lang.label}
              </button>
            );
          })}
        </div>
      </GlassCard>

      {/* Account Info */}
      <GlassCard className="mb-4">
        <div className="flex items-center gap-3 mb-4">
          <Bell size={18} className="text-zinc-400" />
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            {t('settings.account')}
          </h3>
        </div>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-500">{t('settings.account.username')}</span>
            <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {user?.username || '-'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-500">{t('settings.account.email')}</span>
            <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {user?.email || '-'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-500">{t('settings.account.role')}</span>
            <span
              className={`
                text-xs font-medium px-2 py-0.5 rounded-full
                ${user?.role === 'admin' ? 'bg-amber-400/10 text-amber-400' : 'bg-zinc-400/10 text-zinc-400'}
              `}
            >
              {user?.role || 'user'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-500">{t('settings.account.memberSince')}</span>
            <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {user?.createdAt
                ? new Date(user.createdAt).toLocaleDateString(i18n.language, {
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                  })
                : '-'}
            </span>
          </div>
        </div>
      </GlassCard>

      {/* GitHub Connection */}
      <GlassCard className="mb-4">
        <div className="flex items-center gap-3 mb-4">
          <svg viewBox="0 0 24 24" width={18} height={18} className="text-zinc-400 fill-current">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
          </svg>
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            {t('settings.githubConnection')}
          </h3>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {user?.avatarUrl && (
              <img src={user.avatarUrl} alt="" className="w-8 h-8 rounded-lg" />
            )}
            <div>
              <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                {user?.username}
              </p>
              <p className="text-[11px] text-emerald-400">{t('common.status.connected')}</p>
            </div>
          </div>
        </div>
      </GlassCard>

      {/* Notification Settings */}
      <GlassCard className="mb-4">
        <div className="flex items-center gap-3 mb-4">
          <Bell size={18} className="text-zinc-400" />
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            {t('settings.notifications.title')}
          </h3>
        </div>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <p className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('settings.notifications.expiration')}
              </p>
              <p className="text-[11px] text-zinc-500">{t('settings.notifications.expirationDescription')}</p>
            </div>
            <select
              value={useNotificationsStore.getState().expirationHours}
              onChange={(e) => {
                useNotificationsStore.getState().setExpirationHours(Number(e.target.value));
                addNotification({ type: 'success', title: t('settings.notifications.saved') });
              }}
              className={`text-xs rounded-lg border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary/50 ${
                isDark
                  ? 'bg-zinc-900 border-zinc-700 text-zinc-300'
                  : 'bg-white border-zinc-200 text-zinc-800'
              }`}
            >
              <option value="24">24h</option>
              <option value="72">3 {t('common.days')}</option>
              <option value="168">7 {t('common.days')}</option>
              <option value="720">30 {t('common.days')}</option>
              <option value="0">{t('settings.notifications.neverExpire')}</option>
            </select>
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('settings.notifications.maxCount')}
              </p>
              <p className="text-[11px] text-zinc-500">{t('settings.notifications.maxCountDescription')}</p>
            </div>
            <select
              value={useNotificationsStore.getState().maxNotifications}
              onChange={(e) => {
                useNotificationsStore.getState().setMaxNotifications(Number(e.target.value));
                addNotification({ type: 'success', title: t('settings.notifications.saved') });
              }}
              className={`text-xs rounded-lg border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary/50 ${
                isDark
                  ? 'bg-zinc-900 border-zinc-700 text-zinc-300'
                  : 'bg-white border-zinc-200 text-zinc-800'
              }`}
            >
              <option value="50">50</option>
              <option value="100">100</option>
              <option value="200">200</option>
              <option value="500">500</option>
            </select>
          </div>
        </div>
      </GlassCard>

      {/* Replay Tutorials */}
      <GlassCard data-tour="replay-tours">
        <div className="flex items-center gap-3 mb-4">
          <RotateCcw size={18} className="text-zinc-400" />
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            {t('settings.tutorials')}
          </h3>
        </div>
        <div className="flex items-center justify-between">
          <div>
            <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {t('settings.replayTours')}
            </p>
            <p className="text-[11px] text-zinc-500">
              {t('settings.replayToursDescription')}
            </p>
          </div>
          <button
            onClick={handleReplayTours}
            className={`
              flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-medium
              transition-all duration-200
              ${
                isDark
                  ? 'bg-zinc-800/50 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
                  : 'bg-zinc-100 text-zinc-500 hover:bg-zinc-200 hover:text-zinc-700'
              }
            `}
          >
            <RotateCcw size={14} />
            {t('settings.replayTours')}
          </button>
        </div>
      </GlassCard>
    </div>
  );
}
