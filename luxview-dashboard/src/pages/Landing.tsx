import { useNavigate } from 'react-router-dom';
import { Github, Zap, Shield, BarChart3, Globe, ArrowRight, Layers } from 'lucide-react';
import { PillButton } from '../components/common/PillButton';
import { GlassCard } from '../components/common/GlassCard';
import { useAuthStore } from '../stores/auth.store';
import { useEffect } from 'react';

const features = [
  {
    icon: Zap,
    title: 'Auto-detect',
    description: 'Zero-config deployments. We detect your stack automatically from Node.js to Python, Go, Rust, and more.',
    color: 'text-amber-400',
  },
  {
    icon: Shield,
    title: 'Free SSL',
    description: 'Every app gets HTTPS out of the box with auto-renewing certificates via Let\'s Encrypt.',
    color: 'text-emerald-400',
  },
  {
    icon: BarChart3,
    title: 'Monitoring',
    description: 'Real-time metrics, log streaming, and custom alerts. Know exactly how your app is performing.',
    color: 'text-blue-400',
  },
  {
    icon: Layers,
    title: 'Managed Services',
    description: 'One-click PostgreSQL, Redis, MongoDB, and RabbitMQ. Credentials auto-injected.',
    color: 'text-violet-400',
  },
];

export function Landing() {
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/dashboard');
    }
  }, [isAuthenticated, navigate]);

  return (
    <div className="min-h-screen bg-zinc-950 text-white overflow-hidden">
      {/* Background gradient */}
      <div className="fixed inset-0 pointer-events-none">
        <div
          className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[600px]"
          style={{
            background: 'radial-gradient(ellipse at center, rgba(251, 191, 36, 0.06) 0%, transparent 70%)',
          }}
        />
        <div
          className="absolute bottom-0 left-0 w-[600px] h-[400px]"
          style={{
            background: 'radial-gradient(ellipse at center, rgba(59, 130, 246, 0.04) 0%, transparent 70%)',
          }}
        />
      </div>

      {/* Nav */}
      <nav className="relative z-10 flex items-center justify-between px-8 py-6 max-w-6xl mx-auto">
        <div className="flex items-center gap-3">
          <div className="flex items-center justify-center w-9 h-9 rounded-xl bg-zinc-900 ring-1 ring-amber-400/30 font-semibold text-sm tracking-tighter">
            LV
          </div>
          <span className="text-lg font-semibold tracking-tight">LuxView</span>
          <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-amber-400/10 text-amber-400 border border-amber-400/20">
            Cloud
          </span>
        </div>
        <PillButton
          variant="ghost"
          size="sm"
          onClick={() => window.location.href = '/api/auth/github'}
          icon={<Github size={16} />}
        >
          Sign In
        </PillButton>
      </nav>

      {/* Hero */}
      <section className="relative z-10 flex flex-col items-center text-center px-8 pt-20 pb-32 max-w-4xl mx-auto">
        <div className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full bg-zinc-900/80 border border-zinc-800/50 text-xs text-zinc-400 mb-8 backdrop-blur-sm">
          <Globe size={14} className="text-amber-400" />
          Deploy to <span className="text-amber-400 font-medium">*.luxview.cloud</span> in seconds
        </div>

        <h1 className="text-5xl sm:text-6xl lg:text-7xl font-extrabold tracking-tight leading-[1.1] mb-6">
          Deploy anything.
          <br />
          <span className="text-gradient">Zero config.</span>
        </h1>

        <p className="text-lg sm:text-xl text-zinc-400 max-w-2xl mb-10 leading-relaxed">
          Connect your GitHub, pick a repo, and watch it go live. Auto-detection,
          SSL, monitoring, and managed services — all on your own infrastructure.
        </p>

        <div className="flex items-center gap-4">
          <PillButton
            size="lg"
            onClick={() => window.location.href = '/api/auth/github'}
            icon={<Github size={20} />}
          >
            Login with GitHub
          </PillButton>
          <PillButton
            variant="secondary"
            size="lg"
            onClick={() => document.getElementById('features')?.scrollIntoView({ behavior: 'smooth' })}
          >
            Learn More
            <ArrowRight size={16} />
          </PillButton>
        </div>

        {/* Terminal preview */}
        <div className="mt-16 w-full max-w-2xl">
          <div className="rounded-2xl bg-zinc-900/60 border border-zinc-800/50 backdrop-blur-md overflow-hidden shadow-2xl">
            <div className="flex items-center gap-2 px-4 py-3 border-b border-zinc-800/50">
              <span className="w-3 h-3 rounded-full bg-red-500/70" />
              <span className="w-3 h-3 rounded-full bg-amber-500/70" />
              <span className="w-3 h-3 rounded-full bg-emerald-500/70" />
              <span className="ml-4 text-xs text-zinc-500 font-mono">luxview deploy</span>
            </div>
            <div className="p-5 font-mono text-sm leading-7 text-left">
              <div className="text-zinc-500">$ git push origin main</div>
              <div className="text-emerald-400">Detected: Next.js (Node 20)</div>
              <div className="text-zinc-400">Building image... <span className="text-amber-400">done</span> (32s)</div>
              <div className="text-zinc-400">Starting container... <span className="text-amber-400">done</span></div>
              <div className="text-zinc-400">Configuring SSL... <span className="text-emerald-400">done</span></div>
              <div className="text-emerald-400 mt-2">
                Live at <span className="text-amber-400 underline">https://my-app.luxview.cloud</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Features */}
      <section id="features" className="relative z-10 px-8 pb-32 max-w-6xl mx-auto">
        <div className="text-center mb-16">
          <h2 className="text-3xl sm:text-4xl font-bold tracking-tight mb-4">
            Everything you need
          </h2>
          <p className="text-zinc-400 max-w-lg mx-auto">
            A complete platform for deploying, managing, and monitoring your applications.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {features.map((feature) => {
            const Icon = feature.icon;
            return (
              <GlassCard
                key={feature.title}
                hover
                className="group"
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="flex items-center justify-center w-10 h-10 rounded-xl bg-zinc-800/50 group-hover:bg-zinc-800 transition-colors">
                    <Icon size={20} className={feature.color} />
                  </div>
                  <h3 className="text-base font-semibold text-zinc-100">{feature.title}</h3>
                </div>
                <p className="text-sm text-zinc-400 leading-relaxed">{feature.description}</p>
              </GlassCard>
            );
          })}
        </div>
      </section>

      {/* Footer */}
      <footer className="relative z-10 border-t border-zinc-900 px-8 py-8 max-w-6xl mx-auto">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-lg bg-zinc-900 ring-1 ring-amber-400/20 flex items-center justify-center text-[10px] font-bold">
              LV
            </div>
            <span className="text-xs text-zinc-500">LuxView Cloud</span>
          </div>
          <p className="text-xs text-zinc-600">Built with care. Deploy with confidence.</p>
        </div>
      </footer>
    </div>
  );
}
