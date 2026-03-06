import { useNavigate } from 'react-router-dom';
import {
  Github,
  Zap,
  ShieldCheck,
  Globe,
  ArrowRight,
  Database,
  Activity,
  GitBranch,
  Cloud,
  Check,
  ExternalLink,
  Container,
  Terminal,
} from 'lucide-react';
import { PillButton } from '../components/common/PillButton';
import { GlassCard } from '../components/common/GlassCard';
import { useAuthStore } from '../stores/auth.store';
import { useEffect } from 'react';

const features = [
  {
    icon: Zap,
    title: 'Auto-detect Stack',
    description: 'Push any repo. We detect Node, Python, Go, Rust, Docker, or static sites automatically.',
    color: 'text-amber-400',
    bg: 'bg-amber-400/10',
  },
  {
    icon: ShieldCheck,
    title: 'Free SSL',
    description: 'Every app gets HTTPS via Let\'s Encrypt. Auto-renewed, zero config.',
    color: 'text-emerald-400',
    bg: 'bg-emerald-400/10',
  },
  {
    icon: Globe,
    title: 'Custom Subdomains',
    description: 'yourapp.luxview.cloud instantly. Custom domains coming soon.',
    color: 'text-blue-400',
    bg: 'bg-blue-400/10',
  },
  {
    icon: Database,
    title: 'Managed Databases',
    description: 'PostgreSQL, Redis, MongoDB, RabbitMQ — one click, fully isolated.',
    color: 'text-violet-400',
    bg: 'bg-violet-400/10',
  },
  {
    icon: Activity,
    title: 'Real-time Monitoring',
    description: 'CPU, memory, logs, and alerts in real-time. Know before your users do.',
    color: 'text-rose-400',
    bg: 'bg-rose-400/10',
  },
  {
    icon: GitBranch,
    title: 'Auto-deploy',
    description: 'Push to main, we deploy. Rollback to any previous version in one click.',
    color: 'text-cyan-400',
    bg: 'bg-cyan-400/10',
  },
];

const techBadges = [
  { name: 'Node.js', color: 'text-emerald-400 border-emerald-400/20 bg-emerald-400/5' },
  { name: 'Python', color: 'text-blue-400 border-blue-400/20 bg-blue-400/5' },
  { name: 'Go', color: 'text-cyan-400 border-cyan-400/20 bg-cyan-400/5' },
  { name: 'Rust', color: 'text-orange-400 border-orange-400/20 bg-orange-400/5' },
  { name: 'Docker', color: 'text-sky-400 border-sky-400/20 bg-sky-400/5' },
  { name: 'Static Sites', color: 'text-violet-400 border-violet-400/20 bg-violet-400/5' },
];

const steps = [
  {
    number: '1',
    icon: Github,
    title: 'Connect GitHub',
    description: 'Sign in with GitHub and select your repository.',
  },
  {
    number: '2',
    icon: Terminal,
    title: 'Configure & Deploy',
    description: 'Pick a branch, set env vars if needed, and hit deploy.',
  },
  {
    number: '3',
    icon: Globe,
    title: 'Go Live',
    description: 'Your app is live with SSL, monitoring, and a custom subdomain.',
  },
];

const pricingFeatures = [
  '3 apps included',
  '512MB RAM each',
  'Free SSL certificates',
  'Custom subdomain',
  'PostgreSQL & Redis',
  'Auto-deploy from GitHub',
  'Real-time logs & monitoring',
];

export function Landing() {
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/dashboard');
    }
  }, [isAuthenticated, navigate]);

  const scrollTo = (id: string) => {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <div className="min-h-screen bg-zinc-950 text-white overflow-hidden">
      {/* Background gradients */}
      <div className="fixed inset-0 pointer-events-none">
        <div
          className="absolute top-0 left-1/2 -translate-x-1/2 w-[1000px] h-[700px]"
          style={{
            background: 'radial-gradient(ellipse at center, rgba(251, 191, 36, 0.07) 0%, transparent 70%)',
          }}
        />
        <div
          className="absolute bottom-0 left-0 w-[600px] h-[400px]"
          style={{
            background: 'radial-gradient(ellipse at center, rgba(59, 130, 246, 0.04) 0%, transparent 70%)',
          }}
        />
        <div
          className="absolute top-1/2 right-0 w-[500px] h-[500px]"
          style={{
            background: 'radial-gradient(ellipse at center, rgba(139, 92, 246, 0.03) 0%, transparent 70%)',
          }}
        />
      </div>

      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b border-zinc-800/50 bg-zinc-950/80 backdrop-blur-xl">
        <div className="flex items-center justify-between px-6 sm:px-8 py-4 max-w-6xl mx-auto">
          <div className="flex items-center gap-2.5">
            <Cloud size={24} className="text-amber-400" />
            <span className="text-lg font-bold tracking-tight">LuxView Cloud</span>
          </div>

          <div className="hidden md:flex items-center gap-8">
            <button
              onClick={() => scrollTo('features')}
              className="text-sm text-zinc-400 hover:text-white transition-colors"
            >
              Features
            </button>
            <button
              onClick={() => scrollTo('how-it-works')}
              className="text-sm text-zinc-400 hover:text-white transition-colors"
            >
              How it Works
            </button>
            <button
              onClick={() => scrollTo('pricing')}
              className="text-sm text-zinc-400 hover:text-white transition-colors"
            >
              Pricing
            </button>
          </div>

          <PillButton
            variant="ghost"
            size="sm"
            onClick={() => window.location.href = '/api/auth/github'}
            icon={<Github size={16} />}
          >
            Sign In
          </PillButton>
        </div>
      </nav>

      {/* Hero */}
      <section className="relative z-10 flex flex-col items-center text-center px-6 sm:px-8 pt-20 sm:pt-28 pb-20 max-w-4xl mx-auto">
        <div className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full bg-zinc-900/80 border border-zinc-800/50 text-xs text-zinc-400 mb-8 backdrop-blur-sm">
          <Container size={14} className="text-amber-400" />
          Self-hosted PaaS for developers
        </div>

        <h1 className="text-5xl sm:text-6xl lg:text-7xl font-extrabold tracking-tight leading-[1.08] mb-6">
          Deploy from GitHub
          <br />
          <span className="text-gradient">in seconds.</span>
        </h1>

        <p className="text-lg sm:text-xl text-zinc-400 max-w-2xl mb-10 leading-relaxed">
          Push your code. We handle builds, SSL, domains, databases, and monitoring.
          <br className="hidden sm:block" />
          Zero config required.
        </p>

        <div className="flex flex-col sm:flex-row items-center gap-4 mb-6">
          <PillButton
            size="lg"
            onClick={() => window.location.href = '/api/auth/github'}
            icon={<Github size={20} />}
            className="text-base px-10 py-3.5 shadow-lg shadow-amber-400/20 hover:shadow-amber-400/30"
          >
            Start Deploying — It's Free
          </PillButton>
          <PillButton
            variant="secondary"
            size="lg"
            onClick={() => scrollTo('how-it-works')}
          >
            See how it works
            <ArrowRight size={16} />
          </PillButton>
        </div>

        <p className="text-xs text-zinc-500 flex flex-wrap items-center justify-center gap-x-3 gap-y-1">
          <span>No credit card required</span>
          <span className="text-zinc-700">·</span>
          <span>Free SSL</span>
          <span className="text-zinc-700">·</span>
          <span>Custom subdomains</span>
        </p>

        {/* Terminal mockup */}
        <div className="mt-16 w-full max-w-2xl">
          <div className="rounded-2xl bg-zinc-900/60 border border-zinc-800/50 backdrop-blur-md overflow-hidden shadow-2xl shadow-black/40">
            <div className="flex items-center gap-2 px-4 py-3 border-b border-zinc-800/50">
              <span className="w-3 h-3 rounded-full bg-red-500/70" />
              <span className="w-3 h-3 rounded-full bg-amber-500/70" />
              <span className="w-3 h-3 rounded-full bg-emerald-500/70" />
              <span className="ml-4 text-xs text-zinc-500" style={{ fontFamily: 'JetBrains Mono, monospace' }}>
                terminal
              </span>
            </div>
            <div className="p-5 text-sm leading-7 text-left" style={{ fontFamily: 'JetBrains Mono, monospace' }}>
              <div className="text-zinc-500">$ git push origin main</div>
              <div className="text-zinc-400">
                <span className="text-amber-400">→</span> Detected: Node.js (Next.js)
              </div>
              <div className="text-zinc-400">
                <span className="text-amber-400">→</span> Building... <span className="text-emerald-400">done</span> (42s)
              </div>
              <div className="text-zinc-400">
                <span className="text-amber-400">→</span> SSL certificate issued
              </div>
              <div className="text-emerald-400 mt-2">
                <span className="text-amber-400">→</span> Live at{' '}
                <span className="text-amber-400 underline decoration-amber-400/30 underline-offset-2">
                  https://myapp.luxview.cloud
                </span>{' '}
                <span className="text-emerald-400">✓</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Tech badges */}
      <section className="relative z-10 px-6 sm:px-8 pb-24 max-w-4xl mx-auto">
        <p className="text-center text-xs uppercase tracking-widest text-zinc-600 font-medium mb-6">
          Works with
        </p>
        <div className="flex flex-wrap justify-center gap-3">
          {techBadges.map((tech) => (
            <span
              key={tech.name}
              className={`inline-flex items-center px-4 py-2 rounded-full text-sm font-medium border backdrop-blur-sm transition-all duration-200 hover:scale-105 ${tech.color}`}
            >
              {tech.name}
            </span>
          ))}
        </div>
      </section>

      {/* Features */}
      <section id="features" className="relative z-10 px-6 sm:px-8 pb-32 max-w-6xl mx-auto">
        <div className="text-center mb-16">
          <h2 className="text-3xl sm:text-4xl font-bold tracking-tight mb-4">
            Everything you need to ship
          </h2>
          <p className="text-zinc-400 max-w-lg mx-auto leading-relaxed">
            A complete platform for deploying, managing, and monitoring your apps — on your own infrastructure.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {features.map((feature) => {
            const Icon = feature.icon;
            return (
              <GlassCard
                key={feature.title}
                hover
                className="group"
              >
                <div className={`inline-flex items-center justify-center w-11 h-11 rounded-xl ${feature.bg} mb-4 group-hover:scale-110 transition-transform duration-200`}>
                  <Icon size={22} className={feature.color} />
                </div>
                <h3 className="text-base font-semibold text-zinc-100 mb-2">{feature.title}</h3>
                <p className="text-sm text-zinc-400 leading-relaxed">{feature.description}</p>
              </GlassCard>
            );
          })}
        </div>
      </section>

      {/* How it Works */}
      <section id="how-it-works" className="relative z-10 px-6 sm:px-8 pb-32 max-w-4xl mx-auto">
        <div className="text-center mb-16">
          <h2 className="text-3xl sm:text-4xl font-bold tracking-tight mb-4">
            Up and running in 3 steps
          </h2>
          <p className="text-zinc-400 max-w-lg mx-auto leading-relaxed">
            From code to production in under a minute.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-8 relative">
          {/* Connecting line (desktop) */}
          <div className="hidden md:block absolute top-12 left-[calc(16.67%+24px)] right-[calc(16.67%+24px)] h-px bg-gradient-to-r from-amber-400/40 via-amber-400/20 to-amber-400/40" />

          {steps.map((step) => {
            const Icon = step.icon;
            return (
              <div key={step.number} className="flex flex-col items-center text-center">
                <div className="relative mb-6">
                  <div className="w-16 h-16 rounded-2xl bg-zinc-900 border border-zinc-800/50 flex items-center justify-center shadow-lg">
                    <Icon size={28} className="text-amber-400" />
                  </div>
                  <div className="absolute -top-2 -right-2 w-7 h-7 rounded-full bg-amber-400 text-zinc-950 flex items-center justify-center text-xs font-bold shadow-md">
                    {step.number}
                  </div>
                </div>
                <h3 className="text-lg font-semibold text-zinc-100 mb-2">{step.title}</h3>
                <p className="text-sm text-zinc-400 leading-relaxed max-w-[240px]">{step.description}</p>
              </div>
            );
          })}
        </div>
      </section>

      {/* Pricing */}
      <section id="pricing" className="relative z-10 px-6 sm:px-8 pb-32 max-w-xl mx-auto">
        <div className="text-center mb-12">
          <h2 className="text-3xl sm:text-4xl font-bold tracking-tight mb-4">
            Simple pricing
          </h2>
          <p className="text-zinc-400 leading-relaxed">
            Free while in beta. No hidden fees, no surprises.
          </p>
        </div>

        <div className="rounded-2xl bg-zinc-900/60 border border-zinc-800/50 backdrop-blur-md p-8 shadow-2xl shadow-black/20">
          <div className="flex items-baseline gap-3 mb-2">
            <span className="text-4xl font-extrabold tracking-tight">$0</span>
            <span className="text-zinc-500 text-sm">/month</span>
          </div>
          <div className="inline-flex items-center px-3 py-1 rounded-full bg-amber-400/10 text-amber-400 text-xs font-semibold border border-amber-400/20 mb-6">
            Free During Beta
          </div>

          <ul className="space-y-3 mb-8">
            {pricingFeatures.map((feat) => (
              <li key={feat} className="flex items-center gap-3 text-sm text-zinc-300">
                <Check size={16} className="text-emerald-400 flex-shrink-0" />
                {feat}
              </li>
            ))}
          </ul>

          <PillButton
            size="lg"
            className="w-full justify-center shadow-lg shadow-amber-400/20"
            onClick={() => window.location.href = '/api/auth/github'}
            icon={<Github size={18} />}
          >
            Get Started Free
          </PillButton>

          <p className="text-xs text-zinc-500 text-center mt-4">
            Self-hosted on your own VPS. You control everything.
          </p>
        </div>
      </section>

      {/* Footer */}
      <footer className="relative z-10 border-t border-zinc-800/50 bg-zinc-950/80 backdrop-blur-sm">
        <div className="max-w-6xl mx-auto px-6 sm:px-8 py-12">
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-8">
            <div>
              <div className="flex items-center gap-2.5 mb-3">
                <Cloud size={20} className="text-amber-400" />
                <span className="text-base font-bold tracking-tight">LuxView Cloud</span>
              </div>
              <p className="text-xs text-zinc-500 max-w-xs leading-relaxed">
                Self-hosted PaaS platform. Deploy any app from GitHub with zero config.
              </p>
            </div>

            <div className="flex items-center gap-6">
              <a
                href="https://github.com/JohnPitter/luxview-cloud"
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-zinc-400 hover:text-white transition-colors inline-flex items-center gap-1.5"
              >
                <Github size={14} />
                GitHub
              </a>
              <a
                href="#"
                className="text-sm text-zinc-400 hover:text-white transition-colors inline-flex items-center gap-1.5"
              >
                <ExternalLink size={14} />
                Documentation
              </a>
              <a
                href="#"
                className="text-sm text-zinc-400 hover:text-white transition-colors inline-flex items-center gap-1.5"
              >
                <Activity size={14} />
                Status
              </a>
            </div>
          </div>

          <div className="mt-8 pt-8 border-t border-zinc-800/50 flex flex-col sm:flex-row items-center justify-between gap-4">
            <p className="text-xs text-zinc-600" style={{ fontFamily: 'JetBrains Mono, monospace' }}>
              Built with Go, React, Docker & Traefik
            </p>
            <p className="text-xs text-zinc-600">
              &copy; 2026 LuxView Cloud. All rights reserved.
            </p>
          </div>
        </div>
      </footer>
    </div>
  );
}
