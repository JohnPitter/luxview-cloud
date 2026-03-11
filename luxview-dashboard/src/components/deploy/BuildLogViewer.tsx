import { useTranslation } from 'react-i18next';
import { Terminal, type TerminalLine } from '../common/Terminal';

interface BuildLogViewerProps {
  log: string;
  streaming?: boolean;
  loading?: boolean;
}

function parseBuildLog(log: string): TerminalLine[] {
  if (!log) return [];
  return log.split('\n').map((line) => {
    let level: TerminalLine['level'] = 'info';
    if (line.toLowerCase().includes('error') || line.toLowerCase().includes('failed')) {
      level = 'error';
    } else if (line.toLowerCase().includes('warn')) {
      level = 'warn';
    } else if (line.startsWith('#') || line.startsWith('Step')) {
      level = 'debug';
    }
    return { message: line, level };
  });
}

export function BuildLogViewer({ log, streaming = false, loading = false }: BuildLogViewerProps) {
  const { t } = useTranslation();
  const lines = parseBuildLog(log);

  return (
    <Terminal
      lines={lines}
      title={streaming ? t('deploy.buildLog.titleLive') : t('deploy.buildLog.title')}
      maxHeight="400px"
      loading={loading}
    />
  );
}
