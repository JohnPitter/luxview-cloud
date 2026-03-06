import { useState, useCallback } from 'react';
import { Check, ChevronRight, ChevronLeft, Rocket, Plus, Trash2 } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { RepoSelector } from '../apps/RepoSelector';
import { SubdomainInput } from '../apps/SubdomainInput';
import { useThemeStore } from '../../stores/theme.store';
import type { GithubRepo } from '../../api/github';

interface DeployWizardProps {
  repos: GithubRepo[];
  loadingRepos: boolean;
  branches: string[];
  onRepoSelect: (repo: GithubRepo) => void;
  onDeploy: (config: DeployConfig) => void;
  deploying: boolean;
}

export interface DeployConfig {
  repo: GithubRepo;
  branch: string;
  subdomain: string;
  envVars: Record<string, string>;
}

const steps = ['Select Repository', 'Configure', 'Environment', 'Review & Deploy'];

export function DeployWizard({
  repos,
  loadingRepos,
  branches,
  onRepoSelect,
  onDeploy,
  deploying,
}: DeployWizardProps) {
  const [step, setStep] = useState(0);
  const [selectedRepo, setSelectedRepo] = useState<GithubRepo | null>(null);
  const [branch, setBranch] = useState('');
  const [subdomain, setSubdomain] = useState('');
  const [subdomainAvailable, setSubdomainAvailable] = useState(false);
  const [envVars, setEnvVars] = useState<Array<{ key: string; value: string }>>([]);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const handleRepoSelect = useCallback(
    (repo: GithubRepo) => {
      setSelectedRepo(repo);
      setBranch(repo.defaultBranch);
      setSubdomain(repo.name.toLowerCase().replace(/[^a-z0-9-]/g, '-'));
      onRepoSelect(repo);
    },
    [onRepoSelect],
  );

  const addEnvVar = () => setEnvVars([...envVars, { key: '', value: '' }]);

  const removeEnvVar = (index: number) =>
    setEnvVars(envVars.filter((_, i) => i !== index));

  const updateEnvVar = (index: number, field: 'key' | 'value', val: string) =>
    setEnvVars(envVars.map((e, i) => (i === index ? { ...e, [field]: val } : e)));

  const canProceed = () => {
    switch (step) {
      case 0: return !!selectedRepo;
      case 1: return !!branch && !!subdomain && subdomainAvailable;
      case 2: return true;
      case 3: return true;
      default: return false;
    }
  };

  const handleDeploy = () => {
    if (!selectedRepo) return;
    const envRecord: Record<string, string> = {};
    envVars.forEach((e) => {
      if (e.key.trim()) envRecord[e.key.trim()] = e.value;
    });
    onDeploy({ repo: selectedRepo, branch, subdomain, envVars: envRecord });
  };

  const inputClass = `
    w-full px-4 py-2.5 rounded-xl text-sm border transition-all duration-200
    focus:outline-none focus:ring-2 focus:ring-amber-400/30
    ${isDark ? 'bg-zinc-900/50 border-zinc-800 text-zinc-100 placeholder-zinc-600' : 'bg-white border-zinc-200 text-zinc-900 placeholder-zinc-400'}
  `;

  return (
    <div className="max-w-2xl mx-auto">
      {/* Progress */}
      <div className="flex items-center justify-between mb-8">
        {steps.map((label, i) => (
          <div key={label} className="flex items-center">
            <div className="flex items-center gap-2">
              <div
                className={`
                  w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold
                  transition-all duration-200
                  ${
                    i < step
                      ? 'bg-amber-400 text-zinc-950'
                      : i === step
                        ? 'bg-amber-400/20 text-amber-400 ring-2 ring-amber-400/50'
                        : isDark
                          ? 'bg-zinc-800 text-zinc-500'
                          : 'bg-zinc-200 text-zinc-400'
                  }
                `}
              >
                {i < step ? <Check size={14} /> : i + 1}
              </div>
              <span
                className={`text-xs font-medium hidden sm:inline ${
                  i <= step ? (isDark ? 'text-zinc-200' : 'text-zinc-800') : 'text-zinc-500'
                }`}
              >
                {label}
              </span>
            </div>
            {i < steps.length - 1 && (
              <div
                className={`w-12 h-px mx-3 ${
                  i < step ? 'bg-amber-400' : isDark ? 'bg-zinc-800' : 'bg-zinc-200'
                }`}
              />
            )}
          </div>
        ))}
      </div>

      {/* Step Content */}
      <GlassCard className="animate-fade-in">
        {step === 0 && (
          <div>
            <h2
              className={`text-lg font-semibold mb-4 ${
                isDark ? 'text-zinc-100' : 'text-zinc-900'
              }`}
            >
              Select a Repository
            </h2>
            <RepoSelector
              repos={repos}
              loading={loadingRepos}
              selected={selectedRepo}
              onSelect={handleRepoSelect}
            />
          </div>
        )}

        {step === 1 && (
          <div className="space-y-6">
            <h2
              className={`text-lg font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}
            >
              Configure Deployment
            </h2>

            <div>
              <label className={`block text-sm font-medium mb-2 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                Branch
              </label>
              <select
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                className={inputClass}
              >
                {branches.length > 0 ? (
                  branches.map((b) => (
                    <option key={b} value={b}>
                      {b}
                    </option>
                  ))
                ) : (
                  <option value={branch}>{branch}</option>
                )}
              </select>
            </div>

            <SubdomainInput
              value={subdomain}
              onChange={setSubdomain}
              onAvailabilityChange={setSubdomainAvailable}
            />
          </div>
        )}

        {step === 2 && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <h2
                className={`text-lg font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}
              >
                Environment Variables
              </h2>
              <PillButton variant="ghost" size="sm" onClick={addEnvVar} icon={<Plus size={14} />}>
                Add Variable
              </PillButton>
            </div>

            {envVars.length === 0 ? (
              <p className="text-sm text-zinc-500 py-8 text-center">
                No environment variables. You can add them later.
              </p>
            ) : (
              <div className="space-y-2">
                {envVars.map((env, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <input
                      type="text"
                      value={env.key}
                      onChange={(e) => updateEnvVar(i, 'key', e.target.value)}
                      placeholder="KEY"
                      className={`${inputClass} flex-1 font-mono text-xs`}
                    />
                    <input
                      type="text"
                      value={env.value}
                      onChange={(e) => updateEnvVar(i, 'value', e.target.value)}
                      placeholder="value"
                      className={`${inputClass} flex-1 font-mono text-xs`}
                    />
                    <button
                      onClick={() => removeEnvVar(i)}
                      className="p-2 text-zinc-500 hover:text-red-400 transition-colors"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {step === 3 && selectedRepo && (
          <div className="space-y-6">
            <h2
              className={`text-lg font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}
            >
              Review & Deploy
            </h2>

            <div className="space-y-3">
              {[
                { label: 'Repository', value: selectedRepo.fullName },
                { label: 'Branch', value: branch },
                { label: 'Subdomain', value: `${subdomain}.luxview.cloud` },
                { label: 'Language', value: selectedRepo.language || 'Auto-detect' },
                { label: 'Env Variables', value: `${envVars.filter((e) => e.key).length} variables` },
              ].map(({ label, value }) => (
                <div
                  key={label}
                  className={`flex items-center justify-between py-2 border-b ${
                    isDark ? 'border-zinc-800' : 'border-zinc-200'
                  }`}
                >
                  <span className="text-sm text-zinc-500">{label}</span>
                  <span
                    className={`text-sm font-medium ${
                      isDark ? 'text-zinc-200' : 'text-zinc-800'
                    }`}
                  >
                    {value}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </GlassCard>

      {/* Navigation */}
      <div className="flex items-center justify-between mt-6">
        <PillButton
          variant="ghost"
          size="md"
          onClick={() => setStep(step - 1)}
          disabled={step === 0}
          icon={<ChevronLeft size={16} />}
        >
          Back
        </PillButton>

        {step < steps.length - 1 ? (
          <PillButton
            variant="primary"
            size="md"
            onClick={() => setStep(step + 1)}
            disabled={!canProceed()}
          >
            Continue
            <ChevronRight size={16} />
          </PillButton>
        ) : (
          <PillButton
            variant="primary"
            size="md"
            onClick={handleDeploy}
            disabled={deploying}
            icon={<Rocket size={16} />}
          >
            {deploying ? 'Deploying...' : 'Deploy Now'}
          </PillButton>
        )}
      </div>
    </div>
  );
}
