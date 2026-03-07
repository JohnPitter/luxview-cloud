import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { X, Database, Zap } from 'lucide-react';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import type { ServiceType } from '../../api/services';

interface AddServiceDialogProps {
  open: boolean;
  onClose: () => void;
  onAdd: (type: ServiceType) => void;
  existingTypes: ServiceType[];
  loading?: boolean;
}

const services: Array<{
  type: ServiceType;
  labelKey: string;
  descriptionKey: string;
  icon: string;
  color: string;
}> = [
  { type: 'postgres', labelKey: 'services.dialog.postgres.label', descriptionKey: 'services.dialog.postgres.description', icon: 'PG', color: 'border-blue-500/30 hover:border-blue-500/60' },
  { type: 'redis', labelKey: 'services.dialog.redis.label', descriptionKey: 'services.dialog.redis.description', icon: 'RD', color: 'border-red-500/30 hover:border-red-500/60' },
  { type: 'mongodb', labelKey: 'services.dialog.mongodb.label', descriptionKey: 'services.dialog.mongodb.description', icon: 'MG', color: 'border-emerald-500/30 hover:border-emerald-500/60' },
  { type: 'rabbitmq', labelKey: 'services.dialog.rabbitmq.label', descriptionKey: 'services.dialog.rabbitmq.description', icon: 'RQ', color: 'border-orange-500/30 hover:border-orange-500/60' },
  { type: 's3', labelKey: 'services.dialog.s3.label', descriptionKey: 'services.dialog.s3.description', icon: 'S3', color: 'border-purple-500/30 hover:border-purple-500/60' },
];

export function AddServiceDialog({ open, onClose, onAdd, existingTypes, loading }: AddServiceDialogProps) {
  const [selected, setSelected] = useState<ServiceType | null>(null);
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const { t } = useTranslation();

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div
        className={`
          relative z-10 w-full max-w-md rounded-2xl p-6
          backdrop-blur-md shadow-2xl animate-slide-up
          ${isDark ? 'bg-zinc-900 border border-zinc-800' : 'bg-white border border-zinc-200'}
        `}
      >
        <div className="flex items-center justify-between mb-6">
          <h2 className={`text-lg font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('services.dialog.title')}
          </h2>
          <button onClick={onClose} className="text-zinc-500 hover:text-zinc-300 transition-colors">
            <X size={18} />
          </button>
        </div>

        <div className="grid grid-cols-2 gap-3 mb-6">
          {services.map((svc) => {
            const exists = existingTypes.includes(svc.type);
            const isSelected = selected === svc.type;
            return (
              <button
                key={svc.type}
                disabled={exists}
                onClick={() => setSelected(svc.type)}
                className={`
                  p-4 rounded-xl border-2 text-left transition-all duration-200
                  ${exists ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'}
                  ${
                    isSelected
                      ? 'ring-2 ring-amber-400/50 border-amber-400/50 bg-amber-400/5'
                      : isDark
                        ? `bg-zinc-800/30 ${svc.color}`
                        : `bg-zinc-50 ${svc.color}`
                  }
                `}
              >
                <span className={`text-lg font-bold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                  {svc.icon}
                </span>
                <p className={`text-sm font-medium mt-2 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                  {t(svc.labelKey)}
                </p>
                <p className="text-[11px] text-zinc-500 mt-0.5">
                  {exists ? t('services.dialog.alreadyAdded') : t(svc.descriptionKey)}
                </p>
              </button>
            );
          })}
        </div>

        <div className="flex justify-end gap-3">
          <PillButton variant="ghost" size="sm" onClick={onClose}>
            {t('services.dialog.cancel')}
          </PillButton>
          <PillButton
            variant="primary"
            size="sm"
            disabled={!selected || loading}
            onClick={() => selected && onAdd(selected)}
          >
            {loading ? t('services.dialog.provisioning') : t('services.dialog.add')}
          </PillButton>
        </div>
      </div>
    </div>
  );
}
