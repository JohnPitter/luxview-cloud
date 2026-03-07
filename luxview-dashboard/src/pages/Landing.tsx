import { useNavigate } from 'react-router-dom';
import { useEffect, useState, useRef } from 'react';
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

const features = [
  {
    icon: Zap,
    title: 'Auto-detect Stack',
    description: 'Node.js, Python, Go, Rust, Java, Docker, or static — detected automatically from your repo.',
    color: 'text-amber-400',
    bg: 'from-amber-500/20 to-amber-500/5',
    border: 'border-amber-500/10',
  },
  {
    icon: ShieldCheck,
    title: 'Free SSL Certificates',
    description: "Every app gets HTTPS via Let's Encrypt. Auto-renewed, zero configuration needed.",
    color: 'text-emerald-400',
    bg: 'from-emerald-500/20 to-emerald-500/5',
    border: 'border-emerald-500/10',
  },
  {
    icon: Globe,
    title: 'Instant Subdomains',
    description: 'yourapp.luxview.cloud is live the moment your build finishes. Custom domains coming soon.',
    color: 'text-blue-400',
    bg: 'from-blue-500/20 to-blue-500/5',
    border: 'border-blue-500/10',
  },
  {
    icon: Database,
    title: 'Managed Databases',
    description: 'PostgreSQL, Redis, MongoDB, RabbitMQ — provisioned in one click, credentials auto-injected.',
    color: 'text-violet-400',
    bg: 'from-violet-500/20 to-violet-500/5',
    border: 'border-violet-500/10',
  },
  {
    icon: BarChart3,
    title: 'Real-time Monitoring',
    description: 'CPU, memory, runtime logs, and custom alerts — all live in the dashboard.',
    color: 'text-rose-400',
    bg: 'from-rose-500/20 to-rose-500/5',
    border: 'border-rose-500/10',
  },
  {
    icon: RefreshCcw,
    title: 'Auto-deploy & Rollback',
    description: 'Push to main triggers a deploy. Roll back to any previous version in one click.',
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
    title: 'Connect GitHub',
    description: 'Sign in with your GitHub account and authorize LuxView to access your repositories.',
    visual: 'github',
  },
  {
    icon: Box,
    title: 'Pick & Configure',
    description: 'Select a repo, choose a branch, set environment variables, and pick a subdomain.',
    visual: 'configure',
  },
  {
    icon: Rocket,
    title: 'Ship It',
    description: 'Hit deploy. We build, provision SSL, start your container, and route traffic — all automatically.',
    visual: 'deploy',
  },
];

const pricingFeatures = [
  'Unlimited apps',
  'Up to 1GB RAM per app',
  'Free SSL certificates',
  'Custom subdomains',
  'PostgreSQL, Redis, MongoDB, RabbitMQ',
  'Auto-deploy from GitHub',
  'Real-time logs & monitoring',
  'Rollback to any deploy',
];

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
            {['Features', 'How it Works', 'Pricing'].map((item) => (
              <button
                key={item}
                onClick={() => scrollTo(item.toLowerCase().replace(/\s+/g, '-'))}
                className="text-sm text-[#8b949e] hover:text-[#e6edf3] transition-colors"
              >
                {item}
              </button>
            ))}
          </div>

          <button
            onClick={handleAuth}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[#238636] hover:bg-[#2ea043] text-white text-sm font-medium transition-all duration-200 hover:shadow-lg hover:shadow-[#238636]/20"
          >
            <Github size={16} />
            Sign in
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
            Now with managed databases
          </span>
          <span className="text-[#484f58]">|</span>
          <span className="group-hover:text-[#e6edf3] transition-colors">
            Read more <ChevronRight size={12} className="inline" />
          </span>
        </a>

        {/* Headline */}
        <h1 className="text-5xl sm:text-7xl lg:text-[80px] font-extrabold tracking-tight leading-[1.05] mb-6 max-w-4xl">
          <span className="text-[#e6edf3]">Build and deploy</span>
          <br />
          <span
            className="bg-clip-text text-transparent"
            style={{
              backgroundImage: 'linear-gradient(135deg, #79c0ff 0%, #d2a8ff 50%, #ffa657 100%)',
            }}
          >
            from your repo.
          </span>
        </h1>

        <p className="text-lg sm:text-xl text-[#8b949e] max-w-2xl mb-10 leading-relaxed">
          LuxView Cloud auto-detects your stack, builds a container, provisions SSL,
          and gives you a live URL — all from a single <code className="text-[#e6edf3] bg-[#161b22] px-1.5 py-0.5 rounded text-base">git push</code>.
        </p>

        {/* CTA buttons */}
        <div className="flex flex-col sm:flex-row items-center gap-4 mb-4">
          <button
            onClick={handleAuth}
            className="flex items-center gap-2.5 px-8 py-3.5 rounded-xl bg-[#238636] hover:bg-[#2ea043] text-white text-base font-semibold transition-all duration-200 shadow-lg shadow-[#238636]/25 hover:shadow-[#238636]/40 hover:scale-[1.02] active:scale-[0.98]"
          >
            <Github size={20} />
            Start deploying — free
          </button>
          <button
            onClick={() => scrollTo('how-it-works')}
            className="flex items-center gap-2 px-6 py-3.5 rounded-xl border border-[#30363d] text-[#e6edf3] text-base font-medium hover:bg-[#161b22] hover:border-[#8b949e]/50 transition-all duration-200"
          >
            See how it works
            <ArrowRight size={16} />
          </button>
        </div>

        <p className="text-xs text-[#484f58] flex flex-wrap items-center justify-center gap-x-3 gap-y-1 mb-16">
          <span>No credit card</span>
          <span className="text-[#30363d]">·</span>
          <span>Free SSL</span>
          <span className="text-[#30363d]">·</span>
          <span>Instant subdomains</span>
        </p>

        {/* Terminal mockup */}
        <div className="w-full max-w-3xl">
          <AnimatedTerminal />
        </div>
      </section>

      {/* ==================== STACKS ==================== */}
      <section className="relative z-10 px-6 sm:px-8 py-20 max-w-[1280px] mx-auto">
        <p className="text-center text-xs uppercase tracking-[0.2em] text-[#484f58] font-medium mb-8">
          Works with your stack
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
            Everything you need to ship
          </h2>
          <p className="text-[#8b949e] max-w-xl mx-auto text-lg leading-relaxed">
            A complete platform for deploying, managing, and monitoring your applications — on your own infrastructure.
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
            Three steps to production
          </h2>
          <p className="text-[#8b949e] max-w-xl mx-auto text-lg leading-relaxed">
            From repository to live URL in under a minute.
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
                    Step {i + 1}
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
                          <p className="text-sm font-semibold text-[#e6edf3]">GitHub Authorization</p>
                          <p className="text-xs text-[#484f58]">Authorize LuxView Cloud</p>
                        </div>
                      </div>
                      <div className="space-y-2.5">
                        {['Read repository contents', 'Access commit status', 'Receive webhooks'].map((perm) => (
                          <div key={perm} className="flex items-center gap-2.5 text-sm text-[#8b949e]">
                            <Check size={14} className="text-[#3fb950]" />
                            {perm}
                          </div>
                        ))}
                      </div>
                      <button className="mt-5 w-full py-2.5 rounded-lg bg-[#238636] text-white text-sm font-medium">
                        Authorize LuxView
                      </button>
                    </div>
                  )}

                  {step.visual === 'configure' && (
                    <div className="rounded-2xl bg-[#161b22] border border-[#30363d] p-6 shadow-xl shadow-black/20">
                      <div className="space-y-4">
                        <div>
                          <label className="text-xs text-[#8b949e] font-medium mb-1.5 block">Repository</label>
                          <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] text-sm text-[#e6edf3]">
                            <Server size={14} className="text-[#8b949e]" />
                            JohnPitter/my-awesome-app
                          </div>
                        </div>
                        <div className="grid grid-cols-2 gap-3">
                          <div>
                            <label className="text-xs text-[#8b949e] font-medium mb-1.5 block">Branch</label>
                            <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] text-sm text-[#e6edf3]">
                              <GitBranch size={14} className="text-[#8b949e]" />
                              main
                            </div>
                          </div>
                          <div>
                            <label className="text-xs text-[#8b949e] font-medium mb-1.5 block">Subdomain</label>
                            <div className="px-3 py-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] text-sm text-[#e6edf3]">
                              my-app
                              <span className="text-[#484f58]">.luxview.cloud</span>
                            </div>
                          </div>
                        </div>
                        <div>
                          <label className="text-xs text-[#8b949e] font-medium mb-1.5 block">Environment Variables</label>
                          <div className="px-3 py-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] text-sm font-mono text-[#8b949e]">
                            <Lock size={12} className="inline mr-1.5" />
                            3 variables configured
                          </div>
                        </div>
                      </div>
                    </div>
                  )}

                  {step.visual === 'deploy' && (
                    <div className="rounded-2xl bg-[#161b22] border border-[#30363d] p-6 shadow-xl shadow-black/20">
                      <div className="flex items-center gap-3 mb-5">
                        <div className="w-2.5 h-2.5 rounded-full bg-[#3fb950] animate-pulse" />
                        <span className="text-sm font-medium text-[#3fb950]">Deploy successful</span>
                        <span className="text-xs text-[#484f58] ml-auto">38s</span>
                      </div>
                      <div className="space-y-3">
                        {[
                          { label: 'Build', status: 'Done', time: '32s' },
                          { label: 'SSL', status: 'Provisioned', time: '2s' },
                          { label: 'Container', status: 'Running', time: '3s' },
                          { label: 'Health Check', status: 'Passed', time: '1s' },
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
              Simple, transparent pricing
            </h2>
            <p className="text-[#8b949e] max-w-xl mx-auto text-lg leading-relaxed">
              Free while in beta. Self-hosted — you own the infrastructure.
            </p>
          </div>

          <div className="max-w-lg mx-auto">
            <div className="relative rounded-2xl bg-[#161b22] border border-[#30363d] p-8 shadow-2xl shadow-black/30 overflow-hidden">
              {/* Glow border */}
              <div className="absolute inset-0 rounded-2xl opacity-50" style={{
                background: 'linear-gradient(135deg, rgba(56,189,248,0.1), rgba(139,92,246,0.1), rgba(251,191,36,0.1))',
              }} />

              <div className="relative">
                <div className="flex items-baseline gap-3 mb-2">
                  <span className="text-5xl font-extrabold text-[#e6edf3] tracking-tight">$0</span>
                  <span className="text-[#484f58] text-base">/month</span>
                </div>
                <div className="inline-flex items-center px-3 py-1 rounded-full bg-[#238636]/15 text-[#3fb950] text-xs font-semibold border border-[#238636]/30 mb-8">
                  Free During Beta
                </div>

                <ul className="space-y-3.5 mb-8">
                  {pricingFeatures.map((feat) => (
                    <li key={feat} className="flex items-center gap-3 text-sm text-[#e6edf3]">
                      <Check size={16} className="text-[#3fb950] flex-shrink-0" />
                      {feat}
                    </li>
                  ))}
                </ul>

                <button
                  onClick={handleAuth}
                  className="w-full flex items-center justify-center gap-2.5 py-3.5 rounded-xl bg-[#238636] hover:bg-[#2ea043] text-white font-semibold transition-all duration-200 shadow-lg shadow-[#238636]/25 hover:shadow-[#238636]/40"
                >
                  <Github size={18} />
                  Get started free
                </button>

                <p className="text-xs text-[#484f58] text-center mt-4">
                  Deploy on your own VPS. Full control, no vendor lock-in.
                </p>
              </div>
            </div>
          </div>
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
                Self-hosted PaaS platform. Deploy any application from GitHub with automatic stack detection, SSL, and monitoring.
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
                Source
              </a>
              <a
                href="#"
                className="text-sm text-[#8b949e] hover:text-[#e6edf3] transition-colors inline-flex items-center gap-1.5"
              >
                <ExternalLink size={14} />
                Docs
              </a>
              <a
                href="#"
                className="text-sm text-[#8b949e] hover:text-[#e6edf3] transition-colors inline-flex items-center gap-1.5"
              >
                <Activity size={14} />
                Status
              </a>
            </div>
          </div>

          <div className="mt-8 pt-8 border-t border-[#21262d] flex flex-col sm:flex-row items-center justify-between gap-4">
            <p className="text-xs text-[#484f58] font-mono">
              Built with Go + React + Docker + Traefik
            </p>
            <p className="text-xs text-[#484f58]">
              &copy; 2026 LuxView Cloud
            </p>
          </div>
        </div>
      </footer>
    </div>
  );
}
