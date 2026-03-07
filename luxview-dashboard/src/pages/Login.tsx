import { useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';
import { useAuthStore } from '../stores/auth.store';

export function Login() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const setToken = useAuthStore((s) => s.setToken);
  const fetchMe = useAuthStore((s) => s.fetchMe);

  useEffect(() => {
    const token = params.get('token');
    if (!token) {
      navigate('/');
      return;
    }

    setToken(token);
    fetchMe()
      .then(() => {
        navigate('/dashboard', { replace: true });
      })
      .catch(() => {
        navigate('/');
      });
  }, [params, setToken, fetchMe, navigate]);

  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
      <div className="flex flex-col items-center gap-4 animate-fade-in">
        <Loader2 size={32} className="text-amber-400 animate-spin" />
        <p className="text-zinc-400 text-sm">{t('login.authenticating')}</p>
      </div>
    </div>
  );
}
