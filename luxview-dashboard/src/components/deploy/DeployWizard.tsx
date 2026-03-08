import { useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Check, ChevronRight, ChevronLeft, Rocket, Plus, Trash2, AlertCircle, RefreshCw } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { RepoSelector } from '../apps/RepoSelector';
import { SubdomainInput } from '../apps/SubdomainInput';
import { DeployAnalysis } from './DeployAnalysis';
import { useThemeStore } from '../../stores/theme.store';
import type { GithubRepo } from '../../api/github';
import type { AnalysisResult } from '../../api/analyze';

interface DeployWizardProps {
  repos: GithubRepo[];
  loadingRepos: boolean;
  branches: string[];
  onRepoSelect: (repo: GithubRepo) => void;
  onCreateAndAnalyze: (config: DeployConfig) => void;
  onRetryAnalysis: () => void;
  onDeploy: (dockerfile: string, envVars: Record<string, string>, serviceModes?: Record<string, string>) => void;
  onDeployWithoutAnalysis: () => void;
  deploying: boolean;
  analysisResult?: AnalysisResult | null;
  analyzing?: boolean;
  analysisError?: string | null;
}

export interface DeployConfig {
  repo: GithubRepo;
  branch: string;
  subdomain: string;
  envVars: Record<string, string>;
}

export function DeployWizard({
  repos,
  loadingRepos,
  branches,
  onRepoSelect,
  onCreateAndAnalyze,
  onRetryAnalysis,
  onDeploy,
  onDeployWithoutAnalysis,
  deploying,
  analysisResult,
  analyzing = false,
  analysisError,
}: DeployWizardProps) {
  const { t } = useTranslation();
  const [step, setStep] = useState(0);
  const [selectedRepo, setSelectedRepo] = useState<GithubRepo | null>(null);
  const [branch, setBranch] = useState('');
  const [subdomain, setSubdomain] = useState('');
  const [subdomainAvailable, setSubdomainAvailable] = useState(false);
  const [envVars, setEnvVars] = useState<Array<{ key: string; value: string }>>([]);
  const [appCreated, setAppCreated] = useState(false);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const steps = [
    t('deploy.wizard.steps.selectRepository'),
    t('deploy.wizard.steps.configure'),
    t('deploy.wizard.steps.environment'),
    t('deploy.wizard.steps.aiAnalysis'),
    t('deploy.wizard.steps.reviewDeploy'),
  ];

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
      case 3: return !!analysisResult && !analyzing;
      case 4: return true;
      default: return false;
    }
  };

  const handleNext = () => {
    if (step === 2 && !appCreated && selectedRepo) {
      // Moving to step 3 (AI Analysis) — create app and trigger analysis
      const envRecord: Record<string, string> = {};
      envVars.forEach((e) => {
        if (e.key.trim()) envRecord[e.key.trim()] = e.value;
      });
      setAppCreated(true);
      onCreateAndAnalyze({ repo: selectedRepo, branch, subdomain, envVars: envRecord });
      setStep(3);
      return;
    }
    setStep(step + 1);
  };

  const handleDeploy = () => {
    if (!selectedRepo) return;
    // DeployAnalysis onApprove already handled the data — this just deploys with whatever was approved
    // But we need to pass the final dockerfile/envVars from the analysis component
    // Since we're now in step 4 (review), the analysis was already approved in step 3
    onDeployWithoutAnalysis();
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
          <div key={i} className="flex items-center">
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
                className={`w-8 h-px mx-2 ${
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
              {t('deploy.wizard.selectRepo.title')}
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
              {t('deploy.wizard.configure.title')}
            </h2>

            <div>
              <label className={`block text-sm font-medium mb-2 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('deploy.wizard.configure.branch')}
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
                {t('deploy.wizard.env.title')}
              </h2>
              <PillButton variant="ghost" size="sm" onClick={addEnvVar} icon={<Plus size={14} />}>
                {t('deploy.wizard.env.addVariable')}
              </PillButton>
            </div>

            {envVars.length === 0 ? (
              <p className="text-sm text-zinc-500 py-8 text-center">
                {t('deploy.wizard.env.noVariables')}
              </p>
            ) : (
              <div className="space-y-2">
                {envVars.map((env, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <input
                      type="text"
                      value={env.key}
                      onChange={(e) => updateEnvVar(i, 'key', e.target.value)}
                      placeholder={t('deploy.wizard.env.keyPlaceholder')}
                      className={`${inputClass} flex-1 font-mono text-xs`}
                    />
                    <input
                      type="text"
                      value={env.value}
                      onChange={(e) => updateEnvVar(i, 'value', e.target.value)}
                      placeholder={t('deploy.wizard.env.valuePlaceholder')}
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

        {step === 3 && (
          <div>
            {analysisError ? (
              <div className="flex flex-col items-center gap-4 py-8">
                <div className="flex items-center justify-center w-12 h-12 rounded-xl bg-red-500/10">
                  <AlertCircle size={24} className="text-red-400" />
                </div>
                <p className={`text-sm text-center ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                  {analysisError}
                </p>
                <div className="flex gap-3">
                  <PillButton variant="ghost" size="sm" onClick={onRetryAnalysis} icon={<RefreshCw size={14} />}>
                    {t('common.refresh')}
                  </PillButton>
                  <PillButton variant="ghost" size="sm" onClick={() => setStep(4)}>
                    {t('analyze.skipAnalysis')}
                  </PillButton>
                </div>
              </div>
            ) : analysisResult ? (
              <DeployAnalysis
                result={analysisResult}
                loading={false}
                deploying={deploying}
                mode="first-deploy"
                onApprove={(dockerfile, envVarsFromAnalysis, serviceModes) => {
                  onDeploy(dockerfile, envVarsFromAnalysis, serviceModes);
                }}
                onSkip={() => setStep(4)}
              />
            ) : (
              <DeployAnalysis
                result={{ suggestions: [], dockerfile: '', port: 0, stack: '', envHints: [] }}
                loading={analyzing}
                mode="first-deploy"
                onApprove={() => {}}
                onSkip={() => setStep(4)}
              />
            )}
          </div>
        )}

        {step === 4 && selectedRepo && (
          <div className="space-y-6">
            <h2
              className={`text-lg font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}
            >
              {t('deploy.wizard.review.title')}
            </h2>

            <div className="space-y-3">
              {[
                { label: t('deploy.wizard.review.repository'), value: selectedRepo.fullName },
                { label: t('deploy.wizard.review.branch'), value: branch },
                { label: t('deploy.wizard.review.subdomain'), value: `${subdomain}.luxview.cloud` },
                { label: t('deploy.wizard.review.language'), value: selectedRepo.language || t('deploy.wizard.review.autoDetect') },
                { label: t('deploy.wizard.review.envVariables'), value: t('deploy.wizard.review.variablesCount', { count: envVars.filter((e) => e.key).length }) },
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

      {/* Navigation — hidden on step 3 (analysis has its own buttons) */}
      {step !== 3 && (
        <div className="flex items-center justify-between mt-6">
          <PillButton
            variant="ghost"
            size="md"
            onClick={() => setStep(step - 1)}
            disabled={step === 0 || (step > 3 && appCreated)}
            icon={<ChevronLeft size={16} />}
          >
            {t('deploy.wizard.navigation.back')}
          </PillButton>

          {step < steps.length - 1 ? (
            <PillButton
              variant="primary"
              size="md"
              onClick={handleNext}
              disabled={!canProceed()}
            >
              {t('deploy.wizard.navigation.continue')}
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
              {deploying ? t('deploy.wizard.navigation.deploying') : t('deploy.wizard.navigation.deployNow')}
            </PillButton>
          )}
        </div>
      )}
    </div>
  );
}
