import { useNavigate } from 'react-router-dom';
import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { type Plan, plansApi } from '../api/plans';
import {
  Activity,
  ArrowRight,
  BarChart3,
  Box,
  Check,
  Database,
  ExternalLink,
  Github,
  GitBranch,
  Globe,
  Lock,
  Moon,
  RefreshCcw,
  Rocket,
  Server,
  ShieldCheck,
  Sparkles,
  Sun,
  Zap,
} from 'lucide-react';
import { useAuthStore } from '../stores/auth.store';
import { useThemeStore } from '../stores/theme.store';
import { GlassCard } from '../components/common/GlassCard';

function AnimatedTerminal() {
  const { t } = useTranslation();
  const [visibleLines, setVisibleLines] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  const terminalLines = useMemo(
    () => [
      { text: t('landing.terminal.line1'), delay: 0, type: 'command' as const },
      { text: t('landing.terminal.line2'), delay: 800, type: 'info' as const },
      { text: t('landing.terminal.line3'), delay: 1600, type: 'info' as const },
      { text: t('landing.terminal.line4'), delay: 2600, type: 'success' as const },
      { text: t('landing.terminal.line5'), delay: 3600, type: 'info' as const },
      { text: t('landing.terminal.line6'), delay: 4400, type: 'info' as const },
      { text: t('landing.terminal.line7'), delay: 5200, type: 'success' as const },
      { text: t('landing.terminal.line8Suffix'), delay: 6200, type: 'final' as const },
    ],
    [t],
  );

  useEffect(() => {
    const timers: ReturnType<typeof setTimeout>[] = [];
    const duration = terminalLines[terminalLines.length - 1].delay + 2600;

    const runSequence = () => {
      setVisibleLines(0);
      terminalLines.forEach((line, index) => {
        timers.push(setTimeout(() => setVisibleLines(index + 1), line.delay));
      });
    };

    runSequence();
    const interval = setInterval(runSequence, duration);

    return () => {
      timers.forEach(clearTimeout);
      clearInterval(interval);
    };
  }, [terminalLines]);

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [visibleLines]);

  return (
    <div className="overflow-hidden rounded-[28px] border border-[#30363d] bg-[#0d1117] shadow-[0_24px_80px_rgba(0,0,0,0.45)]">
      <div className="flex items-center gap-2 border-b border-[#30363d] bg-[#161b22] px-4 py-3">
        <span className="h-3 w-3 rounded-full bg-[#ff5f57]" />
        <span className="h-3 w-3 rounded-full bg-[#febc2e]" />
        <span className="h-3 w-3 rounded-full bg-[#28c840]" />
        <span className="ml-3 text-xs font-mono text-[#8b949e]">{t('landing.terminal.title')}</span>
      </div>

      <div ref={containerRef} className="min-h-[280px] max-h-[320px] overflow-hidden px-5 py-5 font-mono text-sm leading-7">
        {terminalLines.slice(0, visibleLines).map((line, index) => (
          <div key={`${line.text}-${index}`} className="animate-[fadeSlideIn_0.28s_ease-out]" style={{ animationFillMode: 'both' }}>
            {line.type === 'command' && <span className="text-[#e6edf3]">{line.text}</span>}
            {line.type === 'info' && (
              <span className="text-[#8b949e]">
                <span className="text-[#f0883e]">{line.text.split(':')[0]}:</span>
                {line.text.substring(line.text.indexOf(':') + 1)}
              </span>
            )}
            {line.type === 'success' && (
              <span className="text-[#3fb950]">
                <span className="text-[#f0883e]">{line.text.split(':')[0]}:</span>
                {line.text.substring(line.text.indexOf(':') + 1)} &#10003;
              </span>
            )}
            {line.type === 'final' && (
              <span className="text-[#3fb950]">
                <span className="text-[#f0883e]">{t('landing.terminal.line8Prefix')}</span> {t('landing.terminal.liveAt')}{' '}
                <span className="text-[#58a6ff] underline decoration-[#58a6ff]/30 underline-offset-2">{line.text}</span> &#10003;
              </span>
            )}
          </div>
        ))}
        {visibleLines < terminalLines.length && <span className="ml-0.5 inline-block h-4 w-2 animate-pulse bg-[#58a6ff]" />}
      </div>
    </div>
  );
}

function SectionIntro({
  eyebrow,
  title,
  description,
  isDark,
}: {
  eyebrow: string;
  title: string;
  description: string;
  isDark: boolean;
}) {
  return (
    <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
      <div className="max-w-2xl">
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-amber-400">{eyebrow}</p>
        <h2 className={`mt-3 text-3xl font-semibold tracking-tight sm:text-5xl ${isDark ? 'text-zinc-50' : 'text-zinc-950'}`}>{title}</h2>
      </div>
      <p className={`max-w-2xl text-base leading-7 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>{description}</p>
    </div>
  );
}

export function Landing() {
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const theme = useThemeStore((s) => s.theme);
  const toggleTheme = useThemeStore((s) => s.toggleTheme);
  const isDark = theme === 'dark';
  const { t } = useTranslation();

  const [plans, setPlans] = useState<Plan[]>([]);
  const [plansLoading, setPlansLoading] = useState(true);

  const fetchPlans = useCallback(() => {
    setPlansLoading(true);
    plansApi.listActive().then(setPlans).catch(() => {}).finally(() => setPlansLoading(false));
  }, []);

  useEffect(() => {
    fetchPlans();
  }, [fetchPlans]);

  useEffect(() => {
    if (isAuthenticated) navigate('/dashboard');
  }, [isAuthenticated, navigate]);

  const scrollTo = (id: string) => {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  const handleAuth = () => {
    window.location.href = '/api/auth/github';
  };

  const navItems = [
    { label: t('landing.nav.features'), id: 'features' },
    { label: t('landing.nav.howItWorks'), id: 'how-it-works' },
    { label: t('landing.nav.pricing'), id: 'pricing' },
  ];

  const stacks = [
    { name: 'Node.js', color: '#22c55e' },
    { name: 'Python', color: '#60a5fa' },
    { name: 'Go', color: '#38bdf8' },
    { name: 'Rust', color: '#fb923c' },
    { name: 'Java', color: '#f59e0b' },
    { name: 'Docker', color: '#3b82f6' },
    { name: 'Static', color: '#a855f7' },
  ];

  const features = [
    { icon: Zap, title: t('landing.features.autoDetect.title'), description: t('landing.features.autoDetect.description'), accent: 'from-amber-400/20 via-amber-400/10 to-transparent', iconColor: 'text-amber-400', className: 'lg:col-span-2' },
    { icon: ShieldCheck, title: t('landing.features.ssl.title'), description: t('landing.features.ssl.description'), accent: 'from-emerald-400/20 via-emerald-400/10 to-transparent', iconColor: 'text-emerald-400', className: '' },
    { icon: Globe, title: t('landing.features.subdomains.title'), description: t('landing.features.subdomains.description'), accent: 'from-sky-400/20 via-sky-400/10 to-transparent', iconColor: 'text-sky-400', className: '' },
    { icon: Database, title: t('landing.features.databases.title'), description: t('landing.features.databases.description'), accent: 'from-violet-400/20 via-violet-400/10 to-transparent', iconColor: 'text-violet-400', className: '' },
    { icon: BarChart3, title: t('landing.features.monitoring.title'), description: t('landing.features.monitoring.description'), accent: 'from-rose-400/20 via-rose-400/10 to-transparent', iconColor: 'text-rose-400', className: '' },
    { icon: RefreshCcw, title: t('landing.features.autoDeploy.title'), description: t('landing.features.autoDeploy.description'), accent: 'from-cyan-400/20 via-cyan-400/10 to-transparent', iconColor: 'text-cyan-400', className: 'lg:col-span-2' },
  ];

  const steps = [
    { icon: Github, title: t('landing.howItWorks.connectGithub.title'), description: t('landing.howItWorks.connectGithub.description'), visual: 'github' as const },
    { icon: Box, title: t('landing.howItWorks.pickConfigure.title'), description: t('landing.howItWorks.pickConfigure.description'), visual: 'configure' as const },
    { icon: Rocket, title: t('landing.howItWorks.shipIt.title'), description: t('landing.howItWorks.shipIt.description'), visual: 'deploy' as const },
  ];

  const renderStepVisual = (visual: (typeof steps)[number]['visual']) => {
    const insetSurface = isDark ? 'border-zinc-800/70 bg-zinc-950/70 text-zinc-300' : 'border-zinc-200/80 bg-white/80 text-zinc-700';

    if (visual === 'github') {
      return (
        <div className={`rounded-[24px] border p-5 ${insetSurface}`}>
          <div className="mb-5 flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-emerald-500/15 text-emerald-400"><Github size={20} /></div>
            <div>
              <p className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>{t('landing.howItWorks.github.authorization')}</p>
              <p className="text-xs text-zinc-500">{t('landing.howItWorks.github.authorize')}</p>
            </div>
          </div>
          <div className="space-y-2.5">
            {[t('landing.howItWorks.github.readRepoContents'), t('landing.howItWorks.github.accessCommitStatus'), t('landing.howItWorks.github.receiveWebhooks')].map((permission) => (
              <div key={permission} className="flex items-center gap-2 text-sm"><Check size={14} className="text-emerald-400" /><span>{permission}</span></div>
            ))}
          </div>
        </div>
      );
    }

    if (visual === 'configure') {
      return (
        <div className={`rounded-[24px] border p-5 ${insetSurface}`}>
          <div className="space-y-4">
            <div>
              <p className="mb-1.5 text-[11px] uppercase tracking-[0.18em] text-zinc-500">{t('landing.howItWorks.configure.repository')}</p>
              <div className="flex items-center gap-2 rounded-2xl border border-zinc-700/60 bg-zinc-950/70 px-3 py-2.5 text-sm text-zinc-200"><Server size={14} className="text-zinc-500" />JohnPitter/my-awesome-app</div>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <p className="mb-1.5 text-[11px] uppercase tracking-[0.18em] text-zinc-500">{t('landing.howItWorks.configure.branch')}</p>
                <div className="flex items-center gap-2 rounded-2xl border border-zinc-700/60 bg-zinc-950/70 px-3 py-2.5 text-sm text-zinc-200"><GitBranch size={14} className="text-zinc-500" />main</div>
              </div>
              <div>
                <p className="mb-1.5 text-[11px] uppercase tracking-[0.18em] text-zinc-500">{t('landing.howItWorks.configure.subdomain')}</p>
                <div className="rounded-2xl border border-zinc-700/60 bg-zinc-950/70 px-3 py-2.5 text-sm text-zinc-200">my-app<span className="text-zinc-500">.luxview.cloud</span></div>
              </div>
            </div>
            <div>
              <p className="mb-1.5 text-[11px] uppercase tracking-[0.18em] text-zinc-500">{t('landing.howItWorks.configure.envVars')}</p>
              <div className="rounded-2xl border border-zinc-700/60 bg-zinc-950/70 px-3 py-2.5 font-mono text-sm text-zinc-400"><Lock size={12} className="mr-1.5 inline" />{t('landing.howItWorks.configure.variablesConfigured', { count: 3 })}</div>
            </div>
          </div>
        </div>
      );
    }

    return (
      <div className={`rounded-[24px] border p-5 ${insetSurface}`}>
        <div className="mb-4 flex items-center gap-3">
          <div className="h-2.5 w-2.5 rounded-full bg-emerald-400 shadow-[0_0_18px_rgba(74,222,128,0.55)]" />
          <span className="text-sm font-semibold text-emerald-400">{t('landing.howItWorks.deploy.successful')}</span>
          <span className="ml-auto text-xs font-mono text-zinc-500">38s</span>
        </div>
        <div className="space-y-3">
          {[
            { label: t('landing.howItWorks.deploy.build'), status: t('landing.howItWorks.deploy.done'), time: '32s' },
            { label: t('landing.howItWorks.deploy.ssl'), status: t('landing.howItWorks.deploy.provisioned'), time: '2s' },
            { label: t('landing.howItWorks.deploy.container'), status: t('landing.howItWorks.deploy.running'), time: '3s' },
            { label: t('landing.howItWorks.deploy.healthCheck'), status: t('landing.howItWorks.deploy.passed'), time: '1s' },
          ].map((item) => (
            <div key={item.label} className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2.5"><Check size={14} className="text-emerald-400" /><span className={`text-sm ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>{item.label}</span></div>
              <div className="flex items-center gap-3 text-xs"><span className="text-emerald-400">{item.status}</span><span className="font-mono text-zinc-500">{item.time}</span></div>
            </div>
          ))}
        </div>
        <div className="mt-5 rounded-2xl border border-zinc-700/60 bg-zinc-950/70 px-3 py-3"><div className="flex items-center gap-2"><Globe size={14} className="text-sky-400" /><span className="truncate text-sm text-sky-300">https://my-app.luxview.cloud</span><ExternalLink size={12} className="text-zinc-500" /></div></div>
      </div>
    );
  };

  const pricingVisible = plansLoading || plans.length > 0;
  const footerLinks = [
    { href: 'https://github.com/JohnPitter/luxview-cloud', label: t('landing.footer.source'), icon: Github },
    { href: 'https://github.com/JohnPitter/luxview-cloud/tree/main/docs', label: t('landing.footer.docs'), icon: ExternalLink },
    { href: 'https://github.com/JohnPitter/luxview-cloud/actions', label: t('landing.footer.status'), icon: Activity },
  ];

  return (
    <div className={`min-h-screen overflow-x-hidden ${isDark ? 'bg-zinc-950 text-white' : 'bg-[#f7f4ec] text-zinc-950'}`}>
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div
          className={`absolute left-1/2 top-[-14rem] h-[38rem] w-[78rem] -translate-x-1/2 rounded-full blur-3xl ${isDark ? 'opacity-40' : 'opacity-70'}`}
          style={{
            background: isDark
              ? 'radial-gradient(ellipse at center, rgba(245,158,11,0.22), rgba(56,189,248,0.12) 45%, transparent 72%)'
              : 'radial-gradient(ellipse at center, rgba(245,158,11,0.16), rgba(59,130,246,0.1) 45%, transparent 72%)',
          }}
        />
        <div
          className={`absolute left-[-12rem] top-[20%] h-[26rem] w-[26rem] rounded-full blur-3xl ${isDark ? 'opacity-25' : 'opacity-35'}`}
          style={{ background: 'radial-gradient(circle, rgba(56,189,248,0.18), transparent 70%)' }}
        />
        <div
          className={`absolute right-[-10rem] top-[42%] h-[28rem] w-[28rem] rounded-full blur-3xl ${isDark ? 'opacity-20' : 'opacity-30'}`}
          style={{ background: 'radial-gradient(circle, rgba(245,158,11,0.18), transparent 70%)' }}
        />
        <div
          className={`absolute inset-0 ${isDark ? 'opacity-[0.06]' : 'opacity-[0.05]'}`}
          style={{
            backgroundImage:
              'linear-gradient(rgba(255,255,255,0.08) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.08) 1px, transparent 1px)',
            backgroundSize: '72px 72px',
          }}
        />
      </div>

      <div className="fixed inset-x-0 top-4 z-50 px-4">
        <nav
          className={`mx-auto flex max-w-[1240px] items-center justify-between gap-3 rounded-[28px] px-4 py-3 shadow-2xl backdrop-blur-xl ${
            isDark
              ? 'border border-zinc-800/70 bg-zinc-950/80 shadow-black/35'
              : 'border border-zinc-200/80 bg-white/80 shadow-stone-200/70'
          }`}
        >
          <button onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })} className="flex items-center gap-3 text-left">
            <span className={`flex h-11 w-11 items-center justify-center rounded-2xl ring-1 ring-amber-400/50 ${isDark ? 'bg-zinc-900 shadow-[0_0_28px_rgba(251,191,36,0.2)]' : 'bg-white shadow-[0_0_24px_rgba(251,191,36,0.18)]'}`}>
              <img src="/logo.svg" alt="LuxView" className="h-6 w-6" />
            </span>
            <span className="hidden sm:block">
              <span className={`block text-sm font-semibold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-950'}`}>LuxView Cloud</span>
              <span className={`block text-xs ${isDark ? 'text-zinc-500' : 'text-zinc-500'}`}>{t('landing.brand.tagline')}</span>
            </span>
          </button>

          <div className="hidden items-center gap-2 md:flex">
            {navItems.map((item) => (
              <button
                key={item.id}
                onClick={() => scrollTo(item.id)}
                className={`rounded-full px-4 py-2 text-sm font-medium transition-all duration-200 ${
                  isDark ? 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100' : 'text-zinc-600 hover:bg-white hover:text-zinc-950'
                }`}
              >
                {item.label}
              </button>
            ))}
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={toggleTheme}
              className={`flex h-11 w-11 items-center justify-center rounded-2xl transition-all duration-200 ${
                isDark ? 'border border-zinc-800/70 bg-zinc-900 text-zinc-300 hover:text-amber-400' : 'border border-zinc-200/80 bg-white text-zinc-700 hover:text-amber-600'
              }`}
            >
              {isDark ? <Sun size={18} /> : <Moon size={18} />}
            </button>
            <button
              onClick={handleAuth}
              className="inline-flex items-center gap-2 rounded-2xl bg-amber-500 px-4 py-3 text-sm font-semibold text-zinc-950 shadow-[0_14px_40px_rgba(245,158,11,0.28)] transition-all duration-200 hover:bg-amber-400"
            >
              <Github size={16} />
              <span className="hidden sm:inline">{t('landing.nav.signIn')}</span>
            </button>
          </div>
        </nav>
      </div>

      <main className="relative z-10">
        <section className="mx-auto grid max-w-[1240px] gap-14 px-6 pb-20 pt-32 sm:px-8 sm:pt-40 lg:grid-cols-2 lg:items-center">
          <div className="max-w-3xl">
            <div className={`inline-flex items-center gap-2 rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.22em] ${isDark ? 'border border-amber-400/20 bg-amber-400/10 text-amber-300' : 'border border-amber-500/20 bg-amber-50 text-amber-700'}`}>
              <Sparkles size={14} />
              {t('landing.hero.announcement')}
            </div>

            <h1 className={`mt-8 text-5xl font-semibold tracking-[-0.05em] sm:text-6xl lg:text-[82px] lg:leading-[0.95] ${isDark ? 'text-zinc-50' : 'text-zinc-950'}`}>
              <span>{t('landing.hero.title1')}</span>
              <br />
              <span className="bg-clip-text text-transparent" style={{ backgroundImage: 'linear-gradient(135deg, #fcd34d 0%, #f59e0b 40%, #60a5fa 100%)' }}>
                {t('landing.hero.title2')}
              </span>
            </h1>

            <p className={`mt-7 max-w-2xl text-lg leading-8 sm:text-xl ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
              {t('landing.hero.subtitle')}{' '}
              <code className={`rounded-lg px-2 py-1 text-base ${isDark ? 'bg-zinc-900 text-zinc-100' : 'bg-white text-zinc-900 shadow-sm'}`}>
                {t('landing.hero.subtitleCode')}
              </code>
              .
            </p>

            <div className="mt-10 flex flex-col gap-4 sm:flex-row">
              <button onClick={handleAuth} className="inline-flex items-center justify-center gap-2 rounded-2xl bg-amber-500 px-7 py-4 text-base font-semibold text-zinc-950 shadow-[0_18px_50px_rgba(245,158,11,0.28)] transition-all duration-200 hover:bg-amber-400">
                <Github size={18} />
                {t('landing.hero.ctaPrimary')}
              </button>
              <button
                onClick={() => scrollTo('how-it-works')}
                className={`inline-flex items-center justify-center gap-2 rounded-2xl px-7 py-4 text-base font-medium transition-all duration-200 ${
                  isDark ? 'border border-zinc-800 bg-zinc-900/70 text-zinc-100 hover:border-zinc-700' : 'border border-zinc-200 bg-white/80 text-zinc-900 hover:border-zinc-300'
                }`}
              >
                {t('landing.hero.ctaSecondary')}
                <ArrowRight size={16} />
              </button>
            </div>

            <div className="mt-8 flex flex-wrap gap-3">
              {[t('landing.hero.noCreditCard'), t('landing.hero.freeSSL'), t('landing.hero.instantSubdomains')].map((item) => (
                <span key={item} className={`rounded-full px-4 py-2 text-sm ${isDark ? 'border border-zinc-800 bg-zinc-900/70 text-zinc-300' : 'border border-zinc-200 bg-white/80 text-zinc-700'}`}>
                  {item}
                </span>
              ))}
            </div>

          </div>

          <div className="relative">
            <div className="absolute -right-12 top-8 h-48 w-48 rounded-full blur-3xl" style={{ background: 'radial-gradient(circle, rgba(245,158,11,0.16), transparent 70%)' }} />
            <AnimatedTerminal />
          </div>
        </section>

        <section className="mx-auto max-w-[1240px] px-6 pb-24 sm:px-8">
          <GlassCard className="overflow-hidden">
            <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-amber-400">{t('landing.stacks.title')}</p>
                <p className={`mt-2 text-sm leading-6 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                  {t('landing.stacks.subtitle')}
                </p>
              </div>
              <div className="flex flex-wrap gap-2.5">
                {stacks.map((stack) => (
                  <span key={stack.name} className={`inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-medium ${isDark ? 'border border-zinc-800 bg-zinc-950/70 text-zinc-200' : 'border border-zinc-200 bg-white/80 text-zinc-700'}`}>
                    <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: stack.color }} />
                    {stack.name}
                  </span>
                ))}
              </div>
            </div>
          </GlassCard>
        </section>

        <section id="features" className="mx-auto max-w-[1240px] px-6 pb-24 sm:px-8">
          <SectionIntro
            eyebrow={t('landing.nav.features')}
            title={t('landing.features.title')}
            description={t('landing.features.subtitle')}
            isDark={isDark}
          />

          <div className="mt-12 grid gap-4 lg:grid-cols-3">
            {features.map((feature) => {
              const Icon = feature.icon;
              return (
                <GlassCard key={feature.title} className={`group relative overflow-hidden ${feature.className}`} hover>
                  <div className={`absolute inset-0 bg-gradient-to-br ${feature.accent} opacity-70`} />
                  <div className="relative flex h-full flex-col justify-between">
                    <div>
                      <div className={`inline-flex h-12 w-12 items-center justify-center rounded-2xl ${isDark ? 'bg-zinc-950/70 ring-1 ring-white/5' : 'bg-white/85 ring-1 ring-zinc-200/80'}`}>
                        <Icon size={22} className={feature.iconColor} />
                      </div>
                      <h3 className={`mt-5 text-xl font-semibold tracking-tight ${isDark ? 'text-zinc-50' : 'text-zinc-950'}`}>{feature.title}</h3>
                      <p className={`mt-3 max-w-xl text-sm leading-7 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>{feature.description}</p>
                    </div>

                    {(feature.title === t('landing.features.autoDetect.title') || feature.title === t('landing.features.autoDeploy.title')) && (
                      <div className="mt-6 flex flex-wrap gap-2">
                        {(feature.title === t('landing.features.autoDetect.title')
                          ? stacks.map((stack) => stack.name)
                          : ['main', t('common.rollback'), 'webhooks', t('landing.howItWorks.deploy.healthCheck')]).map((item) => (
                          <span key={item} className={`rounded-full px-3 py-1.5 text-xs uppercase tracking-[0.18em] ${isDark ? 'border border-zinc-800 bg-zinc-950/80 text-zinc-400' : 'border border-zinc-200 bg-white/80 text-zinc-600'}`}>
                            {item}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </GlassCard>
              );
            })}
          </div>
        </section>

        <section id="how-it-works" className="mx-auto max-w-[1240px] px-6 pb-24 sm:px-8">
          <SectionIntro
            eyebrow={t('landing.nav.howItWorks')}
            title={t('landing.howItWorks.title')}
            description={t('landing.howItWorks.subtitle')}
            isDark={isDark}
          />

          <div className="relative mt-12">
            <div
              className={`absolute left-1/2 top-16 hidden h-px w-[68%] -translate-x-1/2 lg:block ${
                isDark ? 'bg-[linear-gradient(90deg,transparent,rgba(245,158,11,0.32),transparent)]' : 'bg-[linear-gradient(90deg,transparent,rgba(245,158,11,0.26),transparent)]'
              }`}
            />
            <div className="grid gap-4 lg:grid-cols-3">
              {steps.map((step, index) => {
                const Icon = step.icon;
                return (
                  <GlassCard key={step.title} className="relative h-full">
                    <div className="flex items-center gap-3">
                      <span className="inline-flex rounded-full bg-amber-500/15 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.2em] text-amber-400">
                        {t('landing.howItWorks.step', { number: index + 1 })}
                      </span>
                    </div>
                    <div className="mt-5 flex items-start gap-4">
                      <div className={`flex h-12 w-12 items-center justify-center rounded-2xl ${isDark ? 'bg-zinc-950/70 ring-1 ring-white/5' : 'bg-white/85 ring-1 ring-zinc-200/80'}`}>
                        <Icon size={22} className="text-amber-400" />
                      </div>
                      <div>
                        <h3 className={`text-2xl font-semibold tracking-tight ${isDark ? 'text-zinc-50' : 'text-zinc-950'}`}>{step.title}</h3>
                        <p className={`mt-3 text-sm leading-7 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>{step.description}</p>
                      </div>
                    </div>
                    <div className="mt-8">{renderStepVisual(step.visual)}</div>
                  </GlassCard>
                );
              })}
            </div>
          </div>
        </section>

        {pricingVisible && (
          <section id="pricing" className="mx-auto max-w-[1240px] px-6 pb-24 sm:px-8">
            <SectionIntro
              eyebrow={t('landing.nav.pricing')}
              title={t('landing.pricing.title')}
              description={t('landing.pricing.subtitle')}
              isDark={isDark}
            />

            <div className="mt-12">
              {plansLoading ? (
                <div className="grid gap-4 lg:grid-cols-3">
                  {[1, 2, 3].map((item) => (
                    <GlassCard key={item} className="animate-pulse">
                      <div className={`h-5 w-24 rounded ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`} />
                      <div className={`mt-5 h-12 w-32 rounded ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`} />
                      <div className="mt-8 space-y-3">
                        {[1, 2, 3, 4, 5].map((row) => (
                          <div key={row} className={`h-4 rounded ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`} />
                        ))}
                      </div>
                    </GlassCard>
                  ))}
                </div>
              ) : (
                <div
                  className={`grid gap-4 ${
                    plans.length === 1 ? 'max-w-lg grid-cols-1' : plans.length === 2 ? 'max-w-4xl grid-cols-1 md:grid-cols-2' : 'grid-cols-1 lg:grid-cols-3'
                  } mx-auto`}
                >
                  {plans.map((plan) => (
                    <GlassCard
                      key={plan.id}
                      className={`relative overflow-hidden ${plan.highlighted ? 'scale-[1.02] border-amber-400/40 shadow-[0_24px_60px_rgba(245,158,11,0.14)]' : ''}`}
                    >
                      {plan.highlighted && <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(245,158,11,0.16),transparent_40%)]" />}
                      <div className="relative">
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <h3 className={`text-2xl font-semibold tracking-tight ${isDark ? 'text-zinc-50' : 'text-zinc-950'}`}>{plan.name}</h3>
                            {plan.description && <p className={`mt-2 text-sm leading-6 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>{plan.description}</p>}
                          </div>
                          {plan.highlighted && (
                            <span className="rounded-full border border-amber-400/30 bg-amber-500/15 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-400">
                              {t('landing.pricing.recommended')}
                            </span>
                          )}
                        </div>

                        <div className="mt-8 flex items-end gap-2">
                          <span className={`text-5xl font-semibold tracking-tight ${isDark ? 'text-zinc-50' : 'text-zinc-950'}`}>
                            {plan.price === 0 ? t('landing.pricing.free') : `${plan.currency === 'BRL' ? 'R$' : plan.currency === 'EUR' ? '\u20AC' : '$'}${plan.price}`}
                          </span>
                          {plan.price > 0 && (
                            <span className={`pb-2 text-sm ${isDark ? 'text-zinc-500' : 'text-zinc-600'}`}>
                              /{plan.billingCycle === 'monthly' ? t('landing.pricing.mo') : t('landing.pricing.yr')}
                            </span>
                          )}
                        </div>

                        <div className="mt-8 grid gap-3">
                          {[
                            t('landing.pricing.upToApps', { count: plan.maxApps }),
                            t('landing.pricing.cpuPerApp', { cpu: plan.maxCpuPerApp }),
                            t('landing.pricing.memoryPerApp', { memory: plan.maxMemoryPerApp.toUpperCase() }),
                            t('landing.pricing.servicesPerApp', { count: plan.maxServicesPerApp }),
                          ].map((item) => (
                            <div key={item} className={`flex items-center gap-2 rounded-2xl px-3 py-2.5 text-sm ${isDark ? 'border border-zinc-800/70 bg-zinc-950/70 text-zinc-300' : 'border border-zinc-200/80 bg-white/80 text-zinc-700'}`}>
                              <Check size={14} className="text-emerald-400" />
                              {item}
                            </div>
                          ))}
                        </div>

                        {(plan.features.length > 0 || plan.autoDeployEnabled || plan.customDomainEnabled || plan.priorityBuilds) && (
                          <div className="mt-8 space-y-3">
                            {[
                              ...(plan.autoDeployEnabled ? [t('landing.pricing.autoDeploy')] : []),
                              ...(plan.customDomainEnabled ? [t('landing.pricing.customDomain')] : []),
                              ...(plan.priorityBuilds ? [t('landing.pricing.priorityBuilds')] : []),
                              ...plan.features,
                            ].map((feature) => (
                              <div key={feature} className="flex items-center gap-2 text-sm">
                                <Check size={14} className="text-amber-400" />
                                <span className={isDark ? 'text-zinc-300' : 'text-zinc-700'}>{feature}</span>
                              </div>
                            ))}
                          </div>
                        )}

                        <button
                          onClick={handleAuth}
                          className={`mt-8 inline-flex w-full items-center justify-center gap-2 rounded-2xl py-3.5 text-sm font-semibold transition-all duration-200 ${
                            plan.highlighted ? 'bg-amber-500 text-zinc-950 hover:bg-amber-400' : isDark ? 'bg-zinc-100 text-zinc-950 hover:bg-white' : 'bg-zinc-950 text-white hover:bg-zinc-800'
                          }`}
                        >
                          <Github size={16} />
                          {t('landing.pricing.cta')}
                        </button>
                      </div>
                    </GlassCard>
                  ))}
                </div>
              )}
            </div>
          </section>
        )}
      </main>

      <footer className={`relative z-10 border-t ${isDark ? 'border-zinc-900 bg-zinc-950' : 'border-zinc-200 bg-[#f7f4ec]'}`}>
        <div className="mx-auto max-w-[1240px] px-6 py-12 sm:px-8">
          <div className="flex flex-col gap-10 lg:flex-row lg:items-start lg:justify-between">
            <div className="max-w-lg">
              <div className="flex items-center gap-3">
                <span className={`flex h-10 w-10 items-center justify-center rounded-2xl ring-1 ring-amber-400/50 ${isDark ? 'bg-zinc-900' : 'bg-white'}`}>
                  <img src="/logo.svg" alt="LuxView" className="h-5 w-5" />
                </span>
                <div>
                  <p className={`text-base font-semibold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-950'}`}>LuxView Cloud</p>
                  <p className={`text-xs ${isDark ? 'text-zinc-500' : 'text-zinc-500'}`}>{t('landing.brand.tagline')}</p>
                </div>
              </div>
              <p className={`mt-4 text-sm leading-7 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>{t('landing.footer.description')}</p>
            </div>

            <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
              {footerLinks.map((link) => {
                const Icon = link.icon;
                return (
                  <a
                    key={link.label}
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={`inline-flex items-center gap-2 rounded-full px-4 py-2.5 text-sm transition-all duration-200 ${
                      isDark ? 'border border-zinc-800 bg-zinc-900/70 text-zinc-300 hover:text-zinc-100' : 'border border-zinc-200 bg-white/80 text-zinc-700 hover:text-zinc-950'
                    }`}
                  >
                    <Icon size={14} />
                    {link.label}
                  </a>
                );
              })}
            </div>
          </div>

          <div className={`mt-10 flex flex-col gap-3 border-t pt-6 text-xs sm:flex-row sm:items-center sm:justify-between ${isDark ? 'border-zinc-900 text-zinc-500' : 'border-zinc-200 text-zinc-500'}`}>
            <p>{t('landing.footer.builtWith')}</p>
            <p>&copy; {t('landing.footer.copyright')}</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
