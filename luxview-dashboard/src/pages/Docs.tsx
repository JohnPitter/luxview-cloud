import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  ArrowLeft,
  BookOpen,
  ChevronRight,
  Database,
  Globe,
  Moon,
  Rocket,
  Server,
  Settings,
  ShieldCheck,
  Sun,
  Zap,
} from 'lucide-react';
import { useThemeStore } from '../stores/theme.store';

interface DocSection {
  id: string;
  icon: React.ComponentType<{ size?: number; className?: string }>;
  titleKey: string;
  descriptionKey: string;
  accent: string;
  content: string[];
}

export function Docs() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const theme = useThemeStore((s) => s.theme);
  const toggleTheme = useThemeStore((s) => s.toggleTheme);
  const isDark = theme === 'dark';
  const [activeSection, setActiveSection] = useState<string | null>(null);

  const sections: DocSection[] = [
    {
      id: 'getting-started',
      icon: Rocket,
      titleKey: 'docs.gettingStarted.title',
      descriptionKey: 'docs.gettingStarted.description',
      accent: 'text-amber-400',
      content: [
        'docs.gettingStarted.step1',
        'docs.gettingStarted.step2',
        'docs.gettingStarted.step3',
        'docs.gettingStarted.step4',
      ],
    },
    {
      id: 'deploying',
      icon: Zap,
      titleKey: 'docs.deploying.title',
      descriptionKey: 'docs.deploying.description',
      accent: 'text-emerald-400',
      content: [
        'docs.deploying.envVars',
        'docs.deploying.autoDeploy',
        'docs.deploying.rollback',
        'docs.deploying.monitoring',
      ],
    },
    {
      id: 'services',
      icon: Database,
      titleKey: 'docs.services.title',
      descriptionKey: 'docs.services.description',
      accent: 'text-violet-400',
      content: [
        'docs.services.postgres',
        'docs.services.redis',
        'docs.services.mongo',
        'docs.services.rabbitmq',
        'docs.services.s3',
      ],
    },
    {
      id: 'stacks',
      icon: Globe,
      titleKey: 'docs.stacks.title',
      descriptionKey: 'docs.stacks.description',
      accent: 'text-sky-400',
      content: [
        'docs.stacks.nodejs',
        'docs.stacks.python',
        'docs.stacks.go',
        'docs.stacks.rust',
        'docs.stacks.java',
        'docs.stacks.docker',
        'docs.stacks.static',
      ],
    },
    {
      id: 'security',
      icon: ShieldCheck,
      titleKey: 'docs.security.title',
      descriptionKey: 'docs.security.description',
      accent: 'text-rose-400',
      content: [
        'docs.security.encryption',
        'docs.security.ssl',
        'docs.security.isolation',
        'docs.security.oauth',
      ],
    },
    {
      id: 'self-hosting',
      icon: Server,
      titleKey: 'docs.selfHosting.title',
      descriptionKey: 'docs.selfHosting.description',
      accent: 'text-cyan-400',
      content: [
        'docs.selfHosting.requirements',
        'docs.selfHosting.setup',
        'docs.selfHosting.dns',
        'docs.selfHosting.deploy',
      ],
    },
    {
      id: 'api',
      icon: Settings,
      titleKey: 'docs.api.title',
      descriptionKey: 'docs.api.description',
      accent: 'text-amber-400',
      content: [
        'docs.api.auth',
        'docs.api.apps',
        'docs.api.services',
        'docs.api.metrics',
      ],
    },
  ];

  return (
    <div className={`min-h-screen ${isDark ? 'bg-zinc-950 text-white' : 'bg-[#f7f4ec] text-zinc-950'}`}>
      {/* Header */}
      <div className="mx-auto max-w-4xl px-6 pt-8 sm:px-8">
        <div className="flex items-center justify-between">
          <button
            onClick={() => navigate('/')}
            className={`inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm transition-all duration-200 ${
              isDark ? 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100' : 'text-zinc-600 hover:bg-white hover:text-zinc-950'
            }`}
          >
            <ArrowLeft size={16} />
            LuxView Cloud
          </button>
          <button
            onClick={toggleTheme}
            className={`flex h-9 w-9 items-center justify-center rounded-xl transition-all duration-200 ${
              isDark ? 'text-zinc-400 hover:text-amber-400' : 'text-zinc-600 hover:text-amber-600'
            }`}
          >
            {isDark ? <Sun size={16} /> : <Moon size={16} />}
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="mx-auto max-w-4xl px-6 py-12 sm:px-8">
        <div className="text-center">
          <div className="flex items-center justify-center gap-2">
            <BookOpen size={20} className="text-amber-400" />
            <h1 className={`text-2xl font-semibold tracking-tight ${isDark ? 'text-zinc-50' : 'text-zinc-950'}`}>
              {t('docs.title')}
            </h1>
          </div>
          <p className={`mt-3 text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
            {t('docs.subtitle')}
          </p>
        </div>

        {/* Section grid */}
        <div className="mt-10 space-y-3">
          {sections.map((section) => {
            const Icon = section.icon;
            const isOpen = activeSection === section.id;

            return (
              <div key={section.id}>
                <button
                  onClick={() => setActiveSection(isOpen ? null : section.id)}
                  className={`w-full rounded-2xl border px-5 py-4 text-left transition-all duration-200 ${
                    isDark
                      ? `border-zinc-800/70 ${isOpen ? 'bg-zinc-900/60' : 'bg-zinc-900/30 hover:bg-zinc-900/50'}`
                      : `border-zinc-200/80 ${isOpen ? 'bg-white' : 'bg-white/60 hover:bg-white/90'}`
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <Icon size={18} className={section.accent} />
                      <div>
                        <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                          {t(section.titleKey)}
                        </h3>
                        <p className={`mt-0.5 text-xs ${isDark ? 'text-zinc-500' : 'text-zinc-500'}`}>
                          {t(section.descriptionKey)}
                        </p>
                      </div>
                    </div>
                    <ChevronRight
                      size={16}
                      className={`transition-transform duration-200 ${isDark ? 'text-zinc-600' : 'text-zinc-400'} ${isOpen ? 'rotate-90' : ''}`}
                    />
                  </div>

                  {isOpen && (
                    <div className="mt-4 space-y-2 border-t border-zinc-800/30 pt-4">
                      {section.content.map((key) => (
                        <div
                          key={key}
                          className={`flex items-start gap-2 text-sm leading-6 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}
                        >
                          <span className={`mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full ${section.accent.replace('text-', 'bg-')}`} />
                          {t(key)}
                        </div>
                      ))}

                      <a
                        href={`https://github.com/JohnPitter/luxview-cloud/blob/main/docs/${section.id === 'getting-started' ? 'getting-started' : section.id === 'deploying' ? 'deploying-apps' : section.id === 'self-hosting' ? 'self-hosting' : section.id === 'api' ? 'api-reference' : section.id}.md`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className={`mt-2 inline-flex items-center gap-1 text-xs font-medium ${section.accent} hover:underline`}
                      >
                        {t('docs.readFull')}
                        <ChevronRight size={12} />
                      </a>
                    </div>
                  )}
                </button>
              </div>
            );
          })}
        </div>

        {/* GitHub link */}
        <div className={`mt-10 text-center text-xs ${isDark ? 'text-zinc-600' : 'text-zinc-400'}`}>
          {t('docs.fullDocs')}{' '}
          <a
            href="https://github.com/JohnPitter/luxview-cloud/tree/main/docs"
            target="_blank"
            rel="noopener noreferrer"
            className="text-amber-400 hover:underline"
          >
            GitHub
          </a>
        </div>
      </div>
    </div>
  );
}
