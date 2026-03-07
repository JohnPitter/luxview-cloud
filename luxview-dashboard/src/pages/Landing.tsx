import { useNavigate } from 'react-router-dom';
import { useEffect, useState, useRef, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { type Plan, plansApi } from '../api/plans';
import {
  Github,
  Zap,
  ShieldCheck,
  Globe,
  ArrowRight,
  Database,
  Activity,
  GitBranch,
  Check,
  ExternalLink,
  Terminal,
  Rocket,
  ChevronRight,
  Server,
  Lock,
  BarChart3,
  RefreshCcw,
  Box,
} from 'lucide-react';
import { useAuthStore } from '../stores/auth.store';

// Animated terminal lines
const terminalLines = [
  { text: '$ git push origin main', delay: 0, type: 'command' as const },
  { text: 'remote: Detected stack: Node.js (Next.js)', delay: 800, type: 'info' as const },
  { text: 'remote: Building image...', delay: 1600, type: 'info' as const },
  { text: 'remote: Build completed in 38s', delay: 2800, type: 'success' as const },
  { text: 'remote: Provisioning SSL certificate...', delay: 3600, type: 'info' as const },
  { text: 'remote: Starting container...', delay: 4200, type: 'info' as const },
  { text: 'remote: Health check passed', delay: 5000, type: 'success' as const },
  { text: 'remote: ', delay: 5800, type: 'final' as const, suffix: 'https://myapp.luxview.cloud' },
];

function AnimatedTerminal() {
  const [visibleLines, setVisibleLines] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const timers: ReturnType<typeof setTimeout>[] = [];
    terminalLines.forEach((line, i) => {
      timers.push(
        setTimeout(() => {
          setVisibleLines(i + 1);
          if (containerRef.current) {
            containerRef.current.scrollTop = containerRef.current.scrollHeight;
          }
        }, line.delay),
      );
    });

    // Reset and replay
    const resetTimer = setTimeout(() => {
      setVisibleLines(0);
      // Restart after brief pause
      setTimeout(() => {
        terminalLines.forEach((line, i) => {
          timers.push(
            setTimeout(() => {
              setVisibleLines(i + 1);
            }, line.delay),
          );
        });
      }, 500);
    }, 9000);
    timers.push(resetTimer);

    return () => timers.forEach(clearTimeout);
  }, []);

  return (
    <div className="rounded-2xl bg-[#0d1117] border border-[#30363d] overflow-hidden shadow-2xl shadow-black/60">
      {/* Title bar */}
      <div className="flex items-center gap-2 px-4 py-3 bg-[#161b22] border-b border-[#30363d]">
        <span className="w-3 h-3 rounded-full bg-[#ff5f57]" />
        <span className="w-3 h-3 rounded-full bg-[#febc2e]" />
        <span className="w-3 h-3 rounded-full bg-[#28c840]" />
        <span className="ml-3 text-xs text-[#8b949e] font-mono">luxview-deploy</span>
      </div>
      {/* Content */}
      <div ref={containerRef} className="p-5 font-mono text-sm leading-7 min-h-[260px] max-h-[300px] overflow-hidden">
        {terminalLines.slice(0, visibleLines).map((line, i) => (
          <div
            key={i}
            className="animate-[fadeSlideIn_0.3s_ease-out]"
            style={{ animationFillMode: 'both' }}
          >
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
                <span className="text-[#f0883e]">remote:</span> Live at{' '}
                <span className="text-[#58a6ff] underline decoration-[#58a6ff]/30 underline-offset-2">
                  {line.suffix}
                </span>{' '}
                &#10003;
              </span>
            )}
          </div>
        ))}
        {visibleLines < terminalLines.length && (
          <span className="inline-block w-2 h-4 bg-[#58a6ff] animate-pulse ml-0.5" />
        )}
      </div>
    </div>
  );
}

export function Landing() {
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const { t } = useTranslation();

  const [plans, setPlans] = useState<Plan[]>([]);
  const [plansLoading, setPlansLoading] = useState(true);

  const fetchPlans = useCallback(() => {
    plansApi.listActive()
      .then(setPlans)
      .catch(() => {})
      .finally(() => setPlansLoading(false));
  }, []);

  useEffect(() => {
    fetchPlans();
  }, [fetchPlans]);

  const features = [
    {
      icon: Zap,
      title: t('landing.features.autoDetect.title'),
      description: t('landing.features.autoDetect.description'),
      color: 'text-amber-400',
      bg: 'from-amber-500/20 to-amber-500/5',
      border: 'border-amber-500/10',
    },
    {
      icon: ShieldCheck,
      title: t('landing.features.ssl.title'),
      description: t('landing.features.ssl.description'),
      color: 'text-emerald-400',
      bg: 'from-emerald-500/20 to-emerald-500/5',
      border: 'border-emerald-500/10',
    },
    {
      icon: Globe,
      title: t('landing.features.subdomains.title'),
      description: t('landing.features.subdomains.description'),
      color: 'text-blue-400',
      bg: 'from-blue-500/20 to-blue-500/5',
      border: 'border-blue-500/10',
    },
    {
      icon: Database,
      title: t('landing.features.databases.title'),
      description: t('landing.features.databases.description'),
      color: 'text-violet-400',
      bg: 'from-violet-500/20 to-violet-500/5',
      border: 'border-violet-500/10',
    },
    {
      icon: BarChart3,
      title: t('landing.features.monitoring.title'),
      description: t('landing.features.monitoring.description'),
      color: 'text-rose-400',
      bg: 'from-rose-500/20 to-rose-500/5',
      border: 'border-rose-500/10',
    },
    {
      icon: RefreshCcw,
      title: t('landing.features.autoDeploy.title'),
      description: t('landing.features.autoDeploy.description'),
      color: 'text-cyan-400',
      bg: 'from-cyan-500/20 to-cyan-500/5',
      border: 'border-cyan-500/10',
    },
  ];

  const stacks = [
    { name: 'Node.js', color: '#339933' },
    { name: 'Python', color: '#3776AB' },
    { name: 'Go', color: '#00ADD8' },
    { name: 'Rust', color: '#DEA584' },
    { name: 'Java', color: '#ED8B00' },
    { name: 'Docker', color: '#2496ED' },
    { name: 'Static', color: '#A855F7' },
  ];

  const steps = [
    {
      icon: Github,
      title: t('landing.howItWorks.connectGithub.title'),
      description: t('landing.howItWorks.connectGithub.description'),
      visual: 'github',
    },
    {
      icon: Box,
      title: t('landing.howItWorks.pickConfigure.title'),
      description: t('landing.howItWorks.pickConfigure.description'),
      visual: 'configure',
    },
    {
      icon: Rocket,
      title: t('landing.howItWorks.shipIt.title'),
      description: t('landing.howItWorks.shipIt.description'),
      visual: 'deploy',
    },
  ];

  const navItems = [
    { label: t('landing.nav.features'), id: 'features' },
    { label: t('landing.nav.howItWorks'), id: 'how-it-works' },
    { label: t('landing.nav.pricing'), id: 'pricing' },
  ];

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/dashboard');
    }
  }, [isAuthenticated, navigate]);

  const scrollTo = (id: string) => {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' });
  };

  const handleAuth = () => {
    window.location.href = '/api/auth/github';
  };

  return (
    <div className="min-h-screen bg-[#0d1117] text-white overflow-hidden">
      {/* Animated gradient background */}
      <div className="fixed inset-0 pointer-events-none overflow-hidden">
        {/* Top hero glow */}
        <div
          className="absolute -top-[300px] left-1/2 -translate-x-1/2 w-[1400px] h-[900px] opacity-40"
          style={{
            background:
              'radial-gradient(ellipse 80% 50% at 50% 40%, rgba(56, 189, 248, 0.15), rgba(139, 92, 246, 0.1) 40%, transparent 70%)',
          }}
        />
        {/* Side glows */}
        <div
          className="absolute top-1/3 -left-[200px] w-[600px] h-[600px] opacity-20"
          style={{
            background: 'radial-gradient(circle, rgba(251, 191, 36, 0.2), transparent 70%)',
          }}
        />
        <div
          className="absolute top-2/3 -right-[200px] w-[500px] h-[500px] opacity-15"
          style={{
            background: 'radial-gradient(circle, rgba(139, 92, 246, 0.2), transparent 70%)',
          }}
        />
        {/* Grid pattern */}
        <div
          className="absolute inset-0 opacity-[0.03]"
          style={{
            backgroundImage:
              'linear-gradient(rgba(255,255,255,0.1) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.1) 1px, transparent 1px)',
            backgroundSize: '64px 64px',
          }}
        />
      </div>

      {/* Navigation */}
      <nav className="sticky top-0 z-50 border-b border-[#30363d]/60 bg-[#0d1117]/80 backdrop-blur-xl">
        <div className="flex items-center justify-between px-6 sm:px-8 py-4 max-w-[1280px] mx-auto">
          <div className="flex items-center gap-3">
            <img src="/logo.svg" alt="LuxView" className="w-7 h-7" />
            <span className="text-lg font-semibold tracking-tight text-[#e6edf3]">LuxView Cloud</span>
          </div>

          <div className="hidden md:flex items-center gap-8">
            {navItems.map((item) => (
              <button
                key={item.id}
                onClick={() => scrollTo(item.id)}
                className="text-sm text-[#8b949e] hover:text-[#e6edf3] transition-colors"
              >
                {item.label}
              </button>
            ))}
          </div>

          <button
            onClick={handleAuth}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[#238636] hover:bg-[#2ea043] text-white text-sm font-medium transition-all duration-200 hover:shadow-lg hover:shadow-[#238636]/20"
          >
            <Github size={16} />
            {t('landing.nav.signIn')}
          </button>
        </div>
      </nav>

      {/* ==================== HERO ==================== */}
      <section className="relative z-10 flex flex-col items-center text-center px-6 sm:px-8 pt-20 sm:pt-32 pb-8 max-w-[1280px] mx-auto">
        {/* Announcement pill */}
        <a
          href="#features"
          onClick={(e) => {
            e.preventDefault();
            scrollTo('features');
          }}
          className="group inline-flex items-center gap-2 px-4 py-1.5 rounded-full bg-[#161b22] border border-[#30363d] text-xs text-[#8b949e] mb-10 hover:border-[#58a6ff]/40 transition-colors"
        >
          <span className="inline-flex items-center gap-1.5 text-[#58a6ff]">
            <Rocket size={12} />
            {t('landing.hero.announcement')}
          </span>
          <span className="text-[#484f58]">|</span>
          <span className="group-hover:text-[#e6edf3] transition-colors">
            {t('landing.hero.readMore')} <ChevronRight size={12} className="inline" />
          </span>
        </a>

        {/* Headline */}
        <h1 className="text-5xl sm:text-7xl lg:text-[80px] font-extrabold tracking-tight leading-[1.05] mb-6 max-w-4xl">
          <span className="text-[#e6edf3]">{t('landing.hero.title1')}</span>
          <br />
          <span
            className="bg-clip-text text-transparent"
            style={{
              backgroundImage: 'linear-gradient(135deg, #79c0ff 0%, #d2a8ff 50%, #ffa657 100%)',
            }}
          >
            {t('landing.hero.title2')}
          </span>
        </h1>

        <p className="text-lg sm:text-xl text-[#8b949e] max-w-2xl mb-10 leading-relaxed">
          {t('landing.hero.subtitle')} <code className="text-[#e6edf3] bg-[#161b22] px-1.5 py-0.5 rounded text-base">{t('landing.hero.subtitleCode')}</code>.
        </p>

        {/* CTA buttons */}
        <div className="flex flex-col sm:flex-row items-center gap-4 mb-4">
          <button
            onClick={handleAuth}
            className="flex items-center gap-2.5 px-8 py-3.5 rounded-xl bg-[#238636] hover:bg-[#2ea043] text-white text-base font-semibold transition-all duration-200 shadow-lg shadow-[#238636]/25 hover:shadow-[#238636]/40 hover:scale-[1.02] active:scale-[0.98]"
          >
            <Github size={20} />
            {t('landing.hero.ctaPrimary')}
          </button>
          <button
            onClick={() => scrollTo('how-it-works')}
            className="flex items-center gap-2 px-6 py-3.5 rounded-xl border border-[#30363d] text-[#e6edf3] text-base font-medium hover:bg-[#161b22] hover:border-[#8b949e]/50 transition-all duration-200"
          >
            {t('landing.hero.ctaSecondary')}
            <ArrowRight size={16} />
          </button>
        </div>

        <p className="text-xs text-[#484f58] flex flex-wrap items-center justify-center gap-x-3 gap-y-1 mb-16">
          <span>{t('landing.hero.noCreditCard')}</span>
          <span className="text-[#30363d]">·</span>
          <span>{t('landing.hero.freeSSL')}</span>
          <span className="text-[#30363d]">·</span>
          <span>{t('landing.hero.instantSubdomains')}</span>
        </p>

        {/* Terminal mockup */}
        <div className="w-full max-w-3xl">
          <AnimatedTerminal />
        </div>
      </section>

      {/* ==================== STACKS ==================== */}
      <section className="relative z-10 px-6 sm:px-8 py-20 max-w-[1280px] mx-auto">
        <p className="text-center text-xs uppercase tracking-[0.2em] text-[#484f58] font-medium mb-8">
          {t('landing.stacks.title')}
        </p>
        <div className="flex flex-wrap justify-center gap-4">
          {stacks.map((s) => (
            <div
              key={s.name}
              className="flex items-center gap-2.5 px-5 py-2.5 rounded-full bg-[#161b22] border border-[#30363d] hover:border-[#484f58] transition-all duration-200 hover:scale-105 cursor-default"
            >
              <span className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: s.color }} />
              <span className="text-sm font-medium text-[#e6edf3]">{s.name}</span>
            </div>
          ))}
        </div>
      </section>

      {/* ==================== FEATURES ==================== */}
      <section id="features" className="relative z-10 px-6 sm:px-8 pb-32 max-w-[1280px] mx-auto">
        {/* Section header */}
        <div className="text-center mb-16">
          <h2 className="text-3xl sm:text-5xl font-bold tracking-tight mb-5 text-[#e6edf3]">
            {t('landing.features.title')}
          </h2>
          <p className="text-[#8b949e] max-w-xl mx-auto text-lg leading-relaxed">
            {t('landing.features.subtitle')}
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {features.map((feature) => {
            const Icon = feature.icon;
            return (
              <div
                key={feature.title}
                className={`group relative rounded-2xl bg-[#161b22] border ${feature.border} hover:border-[#30363d] p-6 transition-all duration-300 hover:shadow-xl hover:shadow-black/20 overflow-hidden`}
              >
                {/* Gradient bg on hover */}
                <div
                  className={`absolute inset-0 bg-gradient-to-b ${feature.bg} opacity-0 group-hover:opacity-100 transition-opacity duration-300`}
                />
                <div className="relative">
                  <div className="mb-4">
                    <Icon size={24} className={feature.color} />
                  </div>
                  <h3 className="text-base font-semibold text-[#e6edf3] mb-2">{feature.title}</h3>
                  <p className="text-sm text-[#8b949e] leading-relaxed">{feature.description}</p>
                </div>
              </div>
            );
          })}
        </div>
      </section>

      {/* ==================== HOW IT WORKS ==================== */}
      <section id="how-it-works" className="relative z-10 px-6 sm:px-8 pb-32 max-w-[1280px] mx-auto">
        <div className="text-center mb-20">
          <h2 className="text-3xl sm:text-5xl font-bold tracking-tight mb-5 text-[#e6edf3]">
            {t('landing.howItWorks.title')}
          </h2>
          <p className="text-[#8b949e] max-w-xl mx-auto text-lg leading-relaxed">
            {t('landing.howItWorks.subtitle')}
          </p>
        </div>

        <div className="space-y-24">
          {steps.map((step, i) => {
            const Icon = step.icon;
            const isEven = i % 2 === 1;
            return (
              <div
                key={step.title}
                className={`flex flex-col ${isEven ? 'md:flex-row-reverse' : 'md:flex-row'} items-center gap-12 md:gap-16`}
              >
                {/* Text */}
                <div className="flex-1 text-center md:text-left">
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-[#161b22] border border-[#30363d] text-xs text-[#8b949e] mb-4">
                    {t('landing.howItWorks.step', { number: i + 1 })}
                  </div>
                  <h3 className="text-2xl sm:text-3xl font-bold text-[#e6edf3] mb-3">
                    {step.title}
                  </h3>
                  <p className="text-[#8b949e] text-lg leading-relaxed max-w-md">
                    {step.description}
                  </p>
                </div>

                {/* Visual */}
                <div className="flex-1 w-full max-w-md">
                  {step.visual === 'github' && (
                    <div className="rounded-2xl bg-[#161b22] border border-[#30363d] p-6 shadow-xl shadow-black/20">
                      <div className="flex items-center gap-3 mb-5">
                        <div className="w-10 h-10 rounded-full bg-[#238636]/20 flex items-center justify-center">
                          <Github size={20} className="text-[#3fb950]" />
                        </div>
                        <div>
                          <p className="text-sm font-semibold text-[#e6edf3]">{t('landing.howItWorks.github.authorization')}</p>
                          <p className="text-xs text-[#484f58]">{t('landing.howItWorks.github.authorize')}</p>
                        </div>
                      </div>
                      <div className="space-y-2.5">
                        {[
                          t('landing.howItWorks.github.readRepoContents'),
                          t('landing.howItWorks.github.accessCommitStatus'),
                          t('landing.howItWorks.github.receiveWebhooks'),
                        ].map((perm) => (
                          <div key={perm} className="flex items-center gap-2.5 text-sm text-[#8b949e]">
                            <Check size={14} className="text-[#3fb950]" />
                            {perm}
                          </div>
                        ))}
                      </div>
                      <button className="mt-5 w-full py-2.5 rounded-lg bg-[#238636] text-white text-sm font-medium">
                        {t('landing.howItWorks.github.authButton')}
                      </button>
                    </div>
                  )}

                  {step.visual === 'configure' && (
                    <div className="rounded-2xl bg-[#161b22] border border-[#30363d] p-6 shadow-xl shadow-black/20">
                      <div className="space-y-4">
                        <div>
                          <label className="text-xs text-[#8b949e] font-medium mb-1.5 block">{t('landing.howItWorks.configure.repository')}</label>
                          <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] text-sm text-[#e6edf3]">
                            <Server size={14} className="text-[#8b949e]" />
                            JohnPitter/my-awesome-app
                          </div>
                        </div>
                        <div className="grid grid-cols-2 gap-3">
                          <div>
                            <label className="text-xs text-[#8b949e] font-medium mb-1.5 block">{t('landing.howItWorks.configure.branch')}</label>
                            <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] text-sm text-[#e6edf3]">
                              <GitBranch size={14} className="text-[#8b949e]" />
                              main
                            </div>
                          </div>
                          <div>
                            <label className="text-xs text-[#8b949e] font-medium mb-1.5 block">{t('landing.howItWorks.configure.subdomain')}</label>
                            <div className="px-3 py-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] text-sm text-[#e6edf3]">
                              my-app
                              <span className="text-[#484f58]">.luxview.cloud</span>
                            </div>
                          </div>
                        </div>
                        <div>
                          <label className="text-xs text-[#8b949e] font-medium mb-1.5 block">{t('landing.howItWorks.configure.envVars')}</label>
                          <div className="px-3 py-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] text-sm font-mono text-[#8b949e]">
                            <Lock size={12} className="inline mr-1.5" />
                            {t('landing.howItWorks.configure.variablesConfigured', { count: 3 })}
                          </div>
                        </div>
                      </div>
                    </div>
                  )}

                  {step.visual === 'deploy' && (
                    <div className="rounded-2xl bg-[#161b22] border border-[#30363d] p-6 shadow-xl shadow-black/20">
                      <div className="flex items-center gap-3 mb-5">
                        <div className="w-2.5 h-2.5 rounded-full bg-[#3fb950] animate-pulse" />
                        <span className="text-sm font-medium text-[#3fb950]">{t('landing.howItWorks.deploy.successful')}</span>
                        <span className="text-xs text-[#484f58] ml-auto">38s</span>
                      </div>
                      <div className="space-y-3">
                        {[
                          { label: t('landing.howItWorks.deploy.build'), status: t('landing.howItWorks.deploy.done'), time: '32s' },
                          { label: t('landing.howItWorks.deploy.ssl'), status: t('landing.howItWorks.deploy.provisioned'), time: '2s' },
                          { label: t('landing.howItWorks.deploy.container'), status: t('landing.howItWorks.deploy.running'), time: '3s' },
                          { label: t('landing.howItWorks.deploy.healthCheck'), status: t('landing.howItWorks.deploy.passed'), time: '1s' },
                        ].map((item) => (
                          <div key={item.label} className="flex items-center justify-between">
                            <div className="flex items-center gap-2.5">
                              <Check size={14} className="text-[#3fb950]" />
                              <span className="text-sm text-[#e6edf3]">{item.label}</span>
                            </div>
                            <div className="flex items-center gap-3">
                              <span className="text-xs text-[#3fb950]">{item.status}</span>
                              <span className="text-xs text-[#484f58] font-mono">{item.time}</span>
                            </div>
                          </div>
                        ))}
                      </div>
                      <div className="mt-5 p-3 rounded-lg bg-[#0d1117] border border-[#30363d]">
                        <div className="flex items-center gap-2">
                          <Globe size={14} className="text-[#58a6ff]" />
                          <a href="#" className="text-sm text-[#58a6ff] hover:underline underline-offset-2">
                            https://my-app.luxview.cloud
                          </a>
                          <ExternalLink size={12} className="text-[#484f58]" />
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </section>

      {/* ==================== PRICING ==================== */}
      <section id="pricing" className="relative z-10 px-6 sm:px-8 pb-32">
        <div className="max-w-[1280px] mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-5xl font-bold tracking-tight mb-5 text-[#e6edf3]">
              {t('landing.pricing.title')}
            </h2>
            <p className="text-[#8b949e] max-w-xl mx-auto text-lg leading-relaxed">
              {t('landing.pricing.subtitle')}
            </p>
          </div>

          {plansLoading ? (
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-5xl mx-auto">
              {[1, 2, 3].map((i) => (
                <div key={i} className="rounded-2xl bg-[#161b22] border border-[#30363d] p-8 animate-pulse">
                  <div className="h-6 bg-[#30363d] rounded w-1/2 mb-4" />
                  <div className="h-10 bg-[#30363d] rounded w-1/3 mb-6" />
                  <div className="space-y-3">
                    {[1, 2, 3, 4].map((j) => (
                      <div key={j} className="h-4 bg-[#30363d] rounded" />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          ) : plans.length === 0 ? null : (
            <div className={`grid gap-6 max-w-5xl mx-auto ${
              plans.length === 1 ? 'grid-cols-1 max-w-lg' :
              plans.length === 2 ? 'grid-cols-1 md:grid-cols-2 max-w-3xl' :
              'grid-cols-1 md:grid-cols-2 lg:grid-cols-3'
            }`}>
              {plans.map((plan) => (
                <div
                  key={plan.id}
                  className={`relative rounded-2xl bg-[#161b22] border p-8 shadow-2xl shadow-black/30 overflow-hidden transition-all duration-300 ${
                    plan.highlighted
                      ? 'border-amber-500/50 scale-105 z-10'
                      : 'border-[#30363d]'
                  }`}
                >
                  {plan.highlighted && (
                    <>
                      <div
                        className="absolute inset-0 rounded-2xl opacity-50"
                        style={{
                          background: 'linear-gradient(135deg, rgba(251,191,36,0.1), rgba(245,158,11,0.05))',
                        }}
                      />
                      <div className="absolute top-4 right-4">
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full bg-amber-500/15 text-amber-400 text-[10px] font-semibold border border-amber-500/30">
                          {t('landing.pricing.recommended')}
                        </span>
                      </div>
                    </>
                  )}

                  <div className="relative">
                    <h3 className="text-lg font-bold text-[#e6edf3] mb-1">{plan.name}</h3>
                    {plan.description && (
                      <p className="text-sm text-[#8b949e] mb-4">{plan.description}</p>
                    )}

                    <div className="flex items-baseline gap-2 mb-6">
                      <span className="text-4xl font-extrabold text-[#e6edf3] tracking-tight">
                        {plan.price === 0
                          ? t('landing.pricing.free')
                          : `${plan.currency === 'BRL' ? 'R$' : plan.currency === 'EUR' ? '\u20AC' : '$'}${plan.price}`}
                      </span>
                      {plan.price > 0 && (
                        <span className="text-[#484f58] text-sm">
                          /{plan.billingCycle === 'monthly' ? t('landing.pricing.mo') : t('landing.pricing.yr')}
                        </span>
                      )}
                    </div>

                    {/* Limits summary */}
                    <div className="space-y-2 mb-6 text-sm text-[#8b949e]">
                      <div className="flex items-center gap-2">
                        <Check size={14} className="text-[#3fb950]" />
                        {t('landing.pricing.upToApps', { count: plan.maxApps })}
                      </div>
                      <div className="flex items-center gap-2">
                        <Check size={14} className="text-[#3fb950]" />
                        {t('landing.pricing.cpuPerApp', { cpu: plan.maxCpuPerApp })}
                      </div>
                      <div className="flex items-center gap-2">
                        <Check size={14} className="text-[#3fb950]" />
                        {t('landing.pricing.memoryPerApp', { memory: plan.maxMemoryPerApp.toUpperCase() })}
                      </div>
                      <div className="flex items-center gap-2">
                        <Check size={14} className="text-[#3fb950]" />
                        {t('landing.pricing.servicesPerApp', { count: plan.maxServicesPerApp })}
                      </div>
                      {plan.autoDeployEnabled && (
                        <div className="flex items-center gap-2">
                          <Check size={14} className="text-[#3fb950]" />
                          {t('landing.pricing.autoDeploy')}
                        </div>
                      )}
                      {plan.customDomainEnabled && (
                        <div className="flex items-center gap-2">
                          <Check size={14} className="text-[#3fb950]" />
                          {t('landing.pricing.customDomain')}
                        </div>
                      )}
                      {plan.priorityBuilds && (
                        <div className="flex items-center gap-2">
                          <Check size={14} className="text-[#3fb950]" />
                          {t('landing.pricing.priorityBuilds')}
                        </div>
                      )}
                    </div>

                    {/* Custom features list */}
                    {plan.features.length > 0 && (
                      <ul className="space-y-2 mb-6">
                        {plan.features.map((feat, i) => (
                          <li key={i} className="flex items-center gap-2 text-sm text-[#e6edf3]">
                            <Check size={14} className="text-[#3fb950] flex-shrink-0" />
                            {feat}
                          </li>
                        ))}
                      </ul>
                    )}

                    <button
                      onClick={handleAuth}
                      className={`w-full flex items-center justify-center gap-2.5 py-3 rounded-xl text-white font-semibold transition-all duration-200 ${
                        plan.highlighted
                          ? 'bg-amber-500 hover:bg-amber-600 shadow-lg shadow-amber-500/25'
                          : 'bg-[#238636] hover:bg-[#2ea043] shadow-lg shadow-[#238636]/25'
                      }`}
                    >
                      <Github size={18} />
                      {t('landing.pricing.cta')}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* ==================== FOOTER ==================== */}
      <footer className="relative z-10 border-t border-[#21262d] bg-[#0d1117]">
        <div className="max-w-[1280px] mx-auto px-6 sm:px-8 py-12">
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-8">
            <div>
              <div className="flex items-center gap-2.5 mb-3">
                <img src="/logo.svg" alt="LuxView" className="w-6 h-6" />
                <span className="text-base font-semibold text-[#e6edf3]">LuxView Cloud</span>
              </div>
              <p className="text-xs text-[#484f58] max-w-xs leading-relaxed">
                {t('landing.footer.description')}
              </p>
            </div>

            <div className="flex items-center gap-6">
              <a
                href="https://github.com/JohnPitter/luxview-cloud"
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-[#8b949e] hover:text-[#e6edf3] transition-colors inline-flex items-center gap-1.5"
              >
                <Github size={14} />
                {t('landing.footer.source')}
              </a>
              <a
                href="#"
                className="text-sm text-[#8b949e] hover:text-[#e6edf3] transition-colors inline-flex items-center gap-1.5"
              >
                <ExternalLink size={14} />
                {t('landing.footer.docs')}
              </a>
              <a
                href="#"
                className="text-sm text-[#8b949e] hover:text-[#e6edf3] transition-colors inline-flex items-center gap-1.5"
              >
                <Activity size={14} />
                {t('landing.footer.status')}
              </a>
            </div>
          </div>

          <div className="mt-8 pt-8 border-t border-[#21262d] flex flex-col sm:flex-row items-center justify-between gap-4">
            <p className="text-xs text-[#484f58] font-mono">
              {t('landing.footer.builtWith')}
            </p>
            <p className="text-xs text-[#484f58]">
              &copy; {t('landing.footer.copyright')}
            </p>
          </div>
        </div>
      </footer>
    </div>
  );
}
