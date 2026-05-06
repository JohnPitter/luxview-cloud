import { useState, useEffect, useRef, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Copy, Check, AlertTriangle, Loader2, CircleCheck, CircleX, Info } from 'lucide-react';
import { appsApi, type DomainCheckResult, type DomainIssue } from '../../api/apps';
import { useThemeStore } from '../../stores/theme.store';

interface Props {
  appId: string;
  /** Currently saved custom domain (null if none). */
  savedDomain: string | null;
  /** Bound to the parent's draft input value. */
  value: string;
  onChange: (v: string) => void;
  inputClassName: string;
}

const POLL_INTERVAL = 10000;

export function CustomDomainSettings({ appId, savedDomain, value, onChange, inputClassName }: Props) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme === 'dark');
  const [check, setCheck] = useState<DomainCheckResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);
  const pollRef = useRef<number | null>(null);

  const runCheck = useCallback(async (probeDomain?: string) => {
    if (!savedDomain && !probeDomain) return;
    setLoading(true);
    try {
      const r = await appsApi.checkDomain(appId, probeDomain);
      setCheck(r);
    } catch {
      // Non-fatal — UI shows last good state.
    } finally {
      setLoading(false);
    }
  }, [appId, savedDomain]);

  // Initial check + polling whenever a domain is saved and not yet ready.
  useEffect(() => {
    if (!savedDomain) {
      setCheck(null);
      return;
    }
    runCheck();
  }, [savedDomain, runCheck]);

  useEffect(() => {
    if (!savedDomain) return;
    if (check?.ready) {
      if (pollRef.current) {
        window.clearInterval(pollRef.current);
        pollRef.current = null;
      }
      return;
    }
    if (pollRef.current) return;
    pollRef.current = window.setInterval(() => runCheck(), POLL_INTERVAL);
    return () => {
      if (pollRef.current) {
        window.clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [savedDomain, check?.ready, runCheck]);

  const copy = async (text: string, key: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      setTimeout(() => setCopied(null), 1500);
    } catch {
      /* ignore */
    }
  };

  const expectedIP = check?.expected_ip || '';
  const showInstructions = !!savedDomain && (!check || !check.ready);

  return (
    <div>
      <label className="block text-xs text-zinc-500 mb-1.5">{t('app.settings.customDomain')}</label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={t('app.settings.customDomainPlaceholder')}
        className={inputClassName}
      />
      <p className="text-[10px] text-zinc-500 mt-1">{t('app.settings.customDomainHint')}</p>

      {/* Instructions card — copy/paste DNS records */}
      {showInstructions && expectedIP && (
        <div
          className={`mt-3 rounded-lg border p-3 ${
            isDark ? 'border-zinc-800 bg-zinc-900/40' : 'border-zinc-200 bg-zinc-50'
          }`}
        >
          <p className={`text-xs font-medium mb-2 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
            {t('app.settings.domain.dnsTitle')}
          </p>
          <div className="space-y-1.5">
            {[
              { type: 'A', name: '@', value: expectedIP, key: 'apex' },
              { type: 'A', name: 'www', value: expectedIP, key: 'www' },
            ].map((rec) => (
              <div
                key={rec.key}
                className={`flex items-center gap-2 rounded px-2 py-1.5 text-[11px] font-mono ${
                  isDark ? 'bg-zinc-950/60 text-zinc-300' : 'bg-white text-zinc-700'
                }`}
              >
                <span className="w-8 text-zinc-500">{rec.type}</span>
                <span className="w-12 text-zinc-500">{rec.name}</span>
                <span className="flex-1">{rec.value}</span>
                <button
                  onClick={() => copy(rec.value, rec.key)}
                  className="text-zinc-500 hover:text-amber-400 transition-colors"
                  title={t('app.settings.domain.copy')}
                >
                  {copied === rec.key ? <Check size={12} /> : <Copy size={12} />}
                </button>
              </div>
            ))}
          </div>
          <p className="text-[10px] text-zinc-500 mt-2">{t('app.settings.domain.dnsExplain')}</p>
        </div>
      )}

      {/* Status */}
      {savedDomain && (
        <div className="mt-3 space-y-1.5">
          <StatusRow
            state={statusFor(check, 'dns', loading)}
            label={t('app.settings.domain.dnsCheck')}
            detail={dnsDetail(check, t)}
          />
          <StatusRow
            state={statusFor(check, 'cert', loading)}
            label={t('app.settings.domain.certCheck')}
            detail={certDetail(check, t)}
          />
          <StatusRow
            state={check?.ready ? 'ok' : loading ? 'pending' : check ? 'pending' : 'idle'}
            label={t('app.settings.domain.online')}
            detail={check?.ready ? `https://${check.domain}` : ''}
          />
        </div>
      )}

      {/* Diagnostics */}
      {check && check.issues.length > 0 && (
        <div className="mt-3 space-y-1.5">
          {check.issues.map((iss) => (
            <DiagnosticItem key={iss} issue={iss} t={t} expectedIP={expectedIP} actualIPs={check.apex.ips} />
          ))}
        </div>
      )}
    </div>
  );
}

type RowState = 'ok' | 'fail' | 'pending' | 'idle';

function statusFor(c: DomainCheckResult | null, kind: 'dns' | 'cert', loading: boolean): RowState {
  if (!c) return loading ? 'pending' : 'idle';
  if (kind === 'dns') {
    if (c.apex.match) return 'ok';
    if (c.apex.ips.length > 0) return 'fail';
    return 'pending';
  }
  if (c.cert.issued) return 'ok';
  if (!c.apex.match) return 'idle';
  return 'pending';
}

function dnsDetail(c: DomainCheckResult | null, t: (k: string) => string): string {
  if (!c) return '';
  if (c.apex.match) return `${c.apex.host} → ${c.expected_ip}`;
  if (c.apex.ips.length > 0) return `${c.apex.host} → ${c.apex.ips.join(', ')}`;
  return t('app.settings.domain.unresolved');
}

function certDetail(c: DomainCheckResult | null, t: (k: string) => string): string {
  if (!c) return '';
  if (c.cert.issued) {
    if (c.cert.not_after) {
      const d = new Date(c.cert.not_after);
      return t('app.settings.domain.certExpires').replace('{{date}}', d.toLocaleDateString());
    }
    return t('app.settings.domain.certActive');
  }
  if (!c.apex.match) return t('app.settings.domain.certWaitingDns');
  return t('app.settings.domain.certIssuing');
}

function StatusRow({ state, label, detail }: { state: RowState; label: string; detail: string }) {
  const isDark = useThemeStore((s) => s.theme === 'dark');
  const Icon =
    state === 'ok' ? CircleCheck : state === 'fail' ? CircleX : state === 'pending' ? Loader2 : Info;
  const color =
    state === 'ok'
      ? 'text-emerald-500'
      : state === 'fail'
      ? 'text-red-500'
      : state === 'pending'
      ? 'text-amber-400'
      : 'text-zinc-500';
  return (
    <div className="flex items-center gap-2 text-[11px]">
      <Icon size={13} className={`${color} ${state === 'pending' ? 'animate-spin' : ''}`} />
      <span className={isDark ? 'text-zinc-300' : 'text-zinc-700'}>{label}</span>
      {detail && <span className="text-zinc-500 truncate">— {detail}</span>}
    </div>
  );
}

function DiagnosticItem({
  issue,
  t,
  expectedIP,
  actualIPs,
}: {
  issue: DomainIssue;
  t: (k: string, opts?: Record<string, unknown>) => string;
  expectedIP: string;
  actualIPs: string[];
}) {
  // cert_pending and apex_unresolved are already shown in status rows; skip duplicates
  if (issue === 'cert_pending' || issue === 'apex_unresolved' || issue === 'empty_domain') return null;

  const messages: Record<DomainIssue, string> = {
    empty_domain: '',
    parking_nameservers: t('app.settings.domain.issue.parking'),
    apex_unresolved: t('app.settings.domain.issue.unresolved'),
    apex_wrong_ip: t('app.settings.domain.issue.wrongIp', { expected: expectedIP, actual: actualIPs.join(', ') }),
    cloudflare_proxy_active: t('app.settings.domain.issue.cloudflare'),
    cert_pending: '',
  };
  const msg = messages[issue];
  if (!msg) return null;

  return (
    <div className="flex items-start gap-2 text-[11px] rounded px-2 py-1.5 bg-amber-500/10 border border-amber-500/20 text-amber-200">
      <AlertTriangle size={12} className="mt-0.5 flex-shrink-0" />
      <span>{msg}</span>
    </div>
  );
}
