import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Github,
  Plus,
  FileCode,
  Key,
  Loader2,
  Check,
  ChevronDown,
  ChevronRight,
  Lock,
  Globe,
} from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';
import { githubApi, type CreateRepoResponse } from '../../api/github';
import { actionsApi } from '../../api/actions';

const DEFAULT_WORKFLOW = `name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install
        run: npm ci
      - name: Test
        run: npm test
`;

interface Props {
  appId?: string;
}

type Step = 'create' | 'workflow' | 'secrets';

export function GitHubRepoSetup({ appId }: Props) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [openStep, setOpenStep] = useState<Step>('create');
  const [createdRepo, setCreatedRepo] = useState<CreateRepoResponse | null>(null);

  // Create repo state
  const [repoName, setRepoName] = useState('');
  const [repoDesc, setRepoDesc] = useState('');
  const [repoPrivate, setRepoPrivate] = useState(true);
  const [creating, setCreating] = useState(false);

  // Workflow state
  const [workflowContent, setWorkflowContent] = useState(DEFAULT_WORKFLOW);
  const [workflowName, setWorkflowName] = useState('ci');
  const [committingWorkflow, setCommittingWorkflow] = useState(false);
  const [workflowDone, setWorkflowDone] = useState(false);

  // Secrets sync state
  const [syncingSecrets, setSyncingSecrets] = useState(false);
  const [secretsDone, setSecretsDone] = useState(false);

  const inputClass = `w-full px-3 py-2 text-sm rounded-lg border ${
    isDark
      ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600'
      : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
  }`;

  async function handleCreateRepo() {
    if (!repoName.trim()) return;
    setCreating(true);
    try {
      const repo = await githubApi.createRepo({ name: repoName.trim(), description: repoDesc, private: repoPrivate });
      setCreatedRepo(repo);
      addNotification({ type: 'success', title: t('github.setup.repoCreated') });
      setOpenStep('workflow');
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('github.setup.createFailed') });
    } finally {
      setCreating(false);
    }
  }

  async function handleCommitWorkflow() {
    if (!createdRepo) return;
    const [owner, repo] = createdRepo.fullName.split('/');
    setCommittingWorkflow(true);
    try {
      await githubApi.commitWorkflow({
        owner,
        repo,
        branch: createdRepo.defaultBranch || 'main',
        workflowName,
        content: workflowContent,
      });
      setWorkflowDone(true);
      addNotification({ type: 'success', title: t('github.setup.workflowCommitted') });
      setOpenStep('secrets');
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('github.setup.workflowFailed') });
    } finally {
      setCommittingWorkflow(false);
    }
  }

  async function handleSyncSecrets() {
    if (!createdRepo || !appId) return;
    setSyncingSecrets(true);
    try {
      const secrets = await actionsApi.listSecrets(appId);
      if (secrets.length === 0) {
        addNotification({ type: 'info', title: t('github.setup.noSecrets') });
        setSecretsDone(true);
        return;
      }
      // Secrets values are not exposed from the API — we can only sync keys.
      // We inform user to manually set values on GitHub.
      addNotification({ type: 'info', title: t('github.setup.secretsManual') });
      setSecretsDone(true);
    } catch {
      addNotification({ type: 'error', title: t('github.setup.syncFailed') });
    } finally {
      setSyncingSecrets(false);
    }
  }

  function StepHeader({ id, title, done }: { id: Step; title: string; done?: boolean }) {
    return (
      <button
        className={`w-full flex items-center gap-3 p-3 rounded-lg transition-colors ${
          isDark ? 'hover:bg-white/5' : 'hover:bg-black/5'
        }`}
        onClick={() => setOpenStep(id)}
      >
        <span className={`flex-shrink-0 w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold ${
          done ? 'bg-emerald-500 text-white' :
          openStep === id ? 'bg-violet-500 text-white' : 'bg-white/10 text-zinc-400'
        }`}>
          {done ? <Check size={11} /> : (id === 'create' ? '1' : id === 'workflow' ? '2' : '3')}
        </span>
        <span className={`flex-1 text-left text-sm font-medium ${isDark ? 'text-white' : 'text-zinc-900'}`}>
          {title}
        </span>
        {openStep === id ? <ChevronDown size={14} className="text-zinc-500" /> : <ChevronRight size={14} className="text-zinc-500" />}
      </button>
    );
  }

  return (
    <GlassCard padding="md">
      <div className="flex items-center gap-2 mb-4">
        <Github size={16} className={isDark ? 'text-white' : 'text-zinc-900'} />
        <h3 className={`font-semibold text-sm ${isDark ? 'text-white' : 'text-zinc-900'}`}>
          {t('github.setup.title')}
        </h3>
      </div>

      <div className="space-y-1">
        {/* Step 1: Create Repo */}
        <div>
          <StepHeader id="create" title={t('github.setup.createRepo')} done={!!createdRepo} />
          {openStep === 'create' && (
            <div className="px-3 pb-3 space-y-3">
              <div className="flex gap-2">
                <input
                  type="text"
                  placeholder={t('github.setup.repoNamePlaceholder')}
                  value={repoName}
                  onChange={(e) => setRepoName(e.target.value.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-_]/g, ''))}
                  className={`${inputClass} flex-1`}
                />
              </div>
              <input
                type="text"
                placeholder={t('github.setup.repoDescPlaceholder')}
                value={repoDesc}
                onChange={(e) => setRepoDesc(e.target.value)}
                className={inputClass}
              />
              <div className="flex gap-3">
                <button
                  onClick={() => setRepoPrivate(true)}
                  className={`flex items-center gap-2 px-3 py-2 text-xs rounded-lg border transition-colors ${
                    repoPrivate
                      ? 'border-violet-500 bg-violet-500/10 text-violet-400'
                      : isDark ? 'border-white/10 text-zinc-500' : 'border-black/10 text-zinc-500'
                  }`}
                >
                  <Lock size={12} /> {t('github.setup.private')}
                </button>
                <button
                  onClick={() => setRepoPrivate(false)}
                  className={`flex items-center gap-2 px-3 py-2 text-xs rounded-lg border transition-colors ${
                    !repoPrivate
                      ? 'border-violet-500 bg-violet-500/10 text-violet-400'
                      : isDark ? 'border-white/10 text-zinc-500' : 'border-black/10 text-zinc-500'
                  }`}
                >
                  <Globe size={12} /> {t('github.setup.public')}
                </button>
              </div>
              <PillButton
                variant="primary"
                size="sm"
                icon={creating ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}
                onClick={handleCreateRepo}
                disabled={creating || !repoName.trim()}
              >
                {t('github.setup.createButton')}
              </PillButton>
              {createdRepo && (
                <a href={createdRepo.htmlUrl} target="_blank" rel="noopener noreferrer"
                  className="text-xs text-violet-400 hover:underline block">
                  {createdRepo.htmlUrl}
                </a>
              )}
            </div>
          )}
        </div>

        {/* Step 2: Commit Workflow */}
        <div>
          <StepHeader id="workflow" title={t('github.setup.commitWorkflow')} done={workflowDone} />
          {openStep === 'workflow' && (
            <div className="px-3 pb-3 space-y-3">
              <div className="flex gap-2 items-center">
                <FileCode size={14} className="text-zinc-500 flex-shrink-0" />
                <input
                  type="text"
                  placeholder="ci"
                  value={workflowName}
                  onChange={(e) => setWorkflowName(e.target.value)}
                  className={`${inputClass} flex-1`}
                />
                <span className="text-xs text-zinc-500 flex-shrink-0">.yml</span>
              </div>
              <textarea
                value={workflowContent}
                onChange={(e) => setWorkflowContent(e.target.value)}
                rows={14}
                className={`${inputClass} font-mono text-xs resize-y`}
                spellCheck={false}
              />
              <PillButton
                variant="primary"
                size="sm"
                icon={committingWorkflow ? <Loader2 size={13} className="animate-spin" /> : <FileCode size={13} />}
                onClick={handleCommitWorkflow}
                disabled={committingWorkflow || !createdRepo}
              >
                {t('github.setup.commitButton')}
              </PillButton>
            </div>
          )}
        </div>

        {/* Step 3: Sync Secrets */}
        {appId && (
          <div>
            <StepHeader id="secrets" title={t('github.setup.syncSecrets')} done={secretsDone} />
            {openStep === 'secrets' && (
              <div className="px-3 pb-3 space-y-3">
                <p className="text-xs text-zinc-500">{t('github.setup.syncSecretsDescription')}</p>
                <PillButton
                  variant="primary"
                  size="sm"
                  icon={syncingSecrets ? <Loader2 size={13} className="animate-spin" /> : <Key size={13} />}
                  onClick={handleSyncSecrets}
                  disabled={syncingSecrets || !createdRepo}
                >
                  {t('github.setup.syncButton')}
                </PillButton>
              </div>
            )}
          </div>
        )}
      </div>
    </GlassCard>
  );
}
