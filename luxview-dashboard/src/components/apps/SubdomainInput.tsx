import { useState, useEffect, useCallback } from 'react';
import { Check, X, Loader2 } from 'lucide-react';
import { useThemeStore } from '../../stores/theme.store';
import { appsApi } from '../../api/apps';
import { slugify } from '../../lib/format';

interface SubdomainInputProps {
  value: string;
  onChange: (value: string) => void;
  onAvailabilityChange: (available: boolean) => void;
}

export function SubdomainInput({ value, onChange, onAvailabilityChange }: SubdomainInputProps) {
  const [checking, setChecking] = useState(false);
  const [available, setAvailable] = useState<boolean | null>(null);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const checkAvailability = useCallback(
    async (subdomain: string) => {
      if (subdomain.length < 2) {
        setAvailable(null);
        onAvailabilityChange(false);
        return;
      }
      setChecking(true);
      try {
        const result = await appsApi.checkSubdomain(subdomain);
        setAvailable(result.available);
        onAvailabilityChange(result.available);
      } catch {
        setAvailable(null);
        onAvailabilityChange(false);
      } finally {
        setChecking(false);
      }
    },
    [onAvailabilityChange],
  );

  useEffect(() => {
    const timer = setTimeout(() => {
      if (value) checkAvailability(value);
    }, 500);
    return () => clearTimeout(timer);
  }, [value, checkAvailability]);

  return (
    <div>
      <label className={`block text-sm font-medium mb-2 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
        Subdomain
      </label>
      <div className="flex items-center gap-0">
        <div className="relative flex-1">
          <input
            type="text"
            value={value}
            onChange={(e) => onChange(slugify(e.target.value))}
            placeholder="my-app"
            className={`
              w-full px-4 py-2.5 rounded-l-xl text-sm
              border-y border-l transition-all duration-200
              focus:outline-none focus:ring-2 focus:ring-amber-400/30
              ${
                isDark
                  ? 'bg-zinc-900/50 border-zinc-800 text-zinc-100 placeholder-zinc-600'
                  : 'bg-white border-zinc-200 text-zinc-900 placeholder-zinc-400'
              }
              ${
                available === true
                  ? 'border-emerald-500/50'
                  : available === false
                    ? 'border-red-500/50'
                    : ''
              }
            `}
          />
          {/* Indicator */}
          <div className="absolute right-3 top-1/2 -translate-y-1/2">
            {checking ? (
              <Loader2 size={16} className="animate-spin text-zinc-500" />
            ) : available === true ? (
              <Check size={16} className="text-emerald-400" />
            ) : available === false ? (
              <X size={16} className="text-red-400" />
            ) : null}
          </div>
        </div>
        <span
          className={`
            px-4 py-2.5 text-sm rounded-r-xl border-y border-r
            ${isDark ? 'bg-zinc-800/50 border-zinc-800 text-zinc-500' : 'bg-zinc-100 border-zinc-200 text-zinc-500'}
          `}
        >
          .luxview.cloud
        </span>
      </div>
      {available === false && (
        <p className="text-xs text-red-400 mt-1.5">This subdomain is already taken</p>
      )}
      {available === true && (
        <p className="text-xs text-emerald-400 mt-1.5">Subdomain is available</p>
      )}
    </div>
  );
}
