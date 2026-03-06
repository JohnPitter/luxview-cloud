import { useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { useAuthStore } from '../stores/auth.store';
import { authApi } from '../api/auth';

export function Login() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const login = useAuthStore((s) => s.login);

  useEffect(() => {
    const code = params.get('code');
    if (!code) {
      navigate('/');
      return;
    }

    authApi
      .handleCallback(code)
      .then(({ token, user }) => {
        login(token, user);
        navigate('/dashboard');
      })
      .catch(() => {
        navigate('/');
      });
  }, [params, login, navigate]);

  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
      <div className="flex flex-col items-center gap-4 animate-fade-in">
        <Loader2 size={32} className="text-amber-400 animate-spin" />
        <p className="text-zinc-400 text-sm">Authenticating with GitHub...</p>
      </div>
    </div>
  );
}
