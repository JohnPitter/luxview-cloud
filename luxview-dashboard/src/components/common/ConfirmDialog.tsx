import { useEffect, useRef } from 'react';
import { AlertTriangle, X } from 'lucide-react';
import { PillButton } from './PillButton';
import { useThemeStore } from '../../stores/theme.store';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: 'danger' | 'warning';
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  variant = 'danger',
  onConfirm,
  onCancel,
  loading = false,
}: ConfirmDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  useEffect(() => {
    if (open) {
      dialogRef.current?.showModal();
    } else {
      dialogRef.current?.close();
    }
  }, [open]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onCancel} />
      <dialog
        ref={dialogRef}
        className={`
          relative z-10 rounded-2xl p-0 m-0 max-w-md w-full
          backdrop-blur-md shadow-2xl animate-slide-up
          ${isDark ? 'bg-zinc-900 border border-zinc-800' : 'bg-white border border-zinc-200'}
        `}
        onClose={onCancel}
      >
        <div className="p-6">
          <div className="flex items-start gap-4">
            <div
              className={`flex items-center justify-center w-10 h-10 rounded-xl flex-shrink-0 ${
                variant === 'danger' ? 'bg-red-500/10' : 'bg-amber-500/10'
              }`}
            >
              <AlertTriangle
                size={20}
                className={variant === 'danger' ? 'text-red-400' : 'text-amber-400'}
              />
            </div>
            <div className="flex-1 min-w-0">
              <h3
                className={`text-base font-semibold mb-1 ${
                  isDark ? 'text-zinc-100' : 'text-zinc-900'
                }`}
              >
                {title}
              </h3>
              <p className="text-sm text-zinc-500">{message}</p>
            </div>
            <button
              onClick={onCancel}
              className="text-zinc-500 hover:text-zinc-300 transition-colors"
            >
              <X size={18} />
            </button>
          </div>
          <div className="flex items-center justify-end gap-3 mt-6">
            <PillButton variant="ghost" size="sm" onClick={onCancel}>
              {cancelLabel}
            </PillButton>
            <PillButton
              variant={variant === 'danger' ? 'danger' : 'primary'}
              size="sm"
              onClick={onConfirm}
              disabled={loading}
            >
              {loading ? 'Processing...' : confirmLabel}
            </PillButton>
          </div>
        </div>
      </dialog>
    </div>
  );
}
