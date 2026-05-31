import './style.css';
import './app.css';
import { GetGames, InstallGame, LaunchGame, OpenInstallFolder } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

type Card = {
  app_id: string;
  name: string;
  game: string;
  display_name: string;
  description: string;
  enabled: boolean;
  download_url: string;
  server_ip: string;
  installed: boolean;
};

type Theme = { accent: string; grad: string; tag: string; initials: string };

const THEMES: Record<string, Theme> = {
  rakion:   { accent: '#e0392b', grad: 'linear-gradient(135deg,#7f1d1d 0%,#2a0a0a 100%)', tag: 'Chaos Force', initials: 'R' },
  wolfteam: { accent: '#22d3ee', grad: 'linear-gradient(135deg,#155e75 0%,#0a1f2a 100%)', tag: 'Wolf Team', initials: 'W' },
  gunbound: { accent: '#f59e0b', grad: 'linear-gradient(135deg,#92400e 0%,#2a1c05 100%)', tag: 'World Champion', initials: 'G' },
  openmu:   { accent: '#a855f7', grad: 'linear-gradient(135deg,#581c87 0%,#1e0a2e 100%)', tag: 'MMORPG', initials: 'M' },
  muemu:    { accent: '#a855f7', grad: 'linear-gradient(135deg,#581c87 0%,#1e0a2e 100%)', tag: 'MMORPG', initials: 'M' },
};
const FALLBACK: Theme = { accent: '#71717a', grad: 'linear-gradient(135deg,#3f3f46 0%,#18181b 100%)', tag: 'Em breve', initials: '?' };
const theme = (g: string): Theme => THEMES[g] || FALLBACK;

function ph(game: string, name: string, desc: string): Card {
  return { app_id: '', name, game, display_name: name, description: desc, enabled: false, download_url: '', server_ip: '', installed: false };
}
// Coming-soon placeholders so the grid feels full (à la SoftNyx).
const PLACEHOLDERS: Card[] = [
  ph('wolfteam', 'Wolf Team', 'TPS de ação multiplayer — em breve na LuxView Cloud.'),
  ph('gunbound', 'GunBound', 'Artilharia por turnos clássica — em breve na LuxView Cloud.'),
];

let games: Card[] = [];
let selected = 0;
let installing = false;
let online = false;

const app = document.querySelector<HTMLDivElement>('#app')!;

async function load() {
  try {
    const real = (await GetGames()) as unknown as Card[];
    online = true;
    const have = new Set(real.map((r) => r.game));
    games = [...real, ...PLACEHOLDERS.filter((p) => !have.has(p.game))];
    if (games.length === 0) games = [...PLACEHOLDERS];
    const firstEnabled = games.findIndex((g) => g.enabled);
    selected = firstEnabled >= 0 ? firstEnabled : 0;
  } catch (e) {
    online = false;
    games = [...PLACEHOLDERS];
    selected = 0;
    toast(String(e), true);
  }
  render();
}

function render() {
  const cur = games[selected];
  const t = theme(cur?.game || '');

  app.innerHTML = `
    <div class="shell">
      <div class="topbar">
        <div class="brand-mark">L</div>
        <div class="brand-text"><b>LuxView Cloud</b><span>Games</span></div>
        <div class="spacer"></div>
        <div class="status ${online ? '' : 'off'}"><span class="dot"></span>${online ? 'Conectado' : 'Offline'}</div>
      </div>

      <div class="strip">
        ${games.map((g, i) => chip(g, i)).join('')}
      </div>

      ${cur ? hero(cur, t) : '<div class="hero hero-empty">Nenhum jogo disponível</div>'}

      <div class="footer">
        <div class="progress-wrap">
          <div class="progress-line" id="pline">${footerLine(cur)}</div>
          <div class="bar"><i id="pbar" style="width:0%"></i></div>
        </div>
        <div class="actions">
          <button class="btn icon" id="folderBtn" title="Abrir pasta de instalação" ${cur && cur.installed ? '' : 'disabled'}>📁</button>
          ${actionBtn(cur)}
        </div>
      </div>
    </div>
  `;

  games.forEach((_, i) => {
    document.getElementById('chip-' + i)?.addEventListener('click', () => {
      if (installing) return;
      selected = i;
      render();
    });
  });
  document.getElementById('actionBtn')?.addEventListener('click', doAction);
  document.getElementById('folderBtn')?.addEventListener('click', () => {
    if (cur && cur.app_id) OpenInstallFolder(cur.app_id);
  });
}

function chip(g: Card, i: number): string {
  const t = theme(g.game);
  const cls = ['chip'];
  if (i === selected) cls.push('selected');
  if (!g.enabled) cls.push('disabled');
  const pill = g.enabled ? `<span class="pill on">online</span>` : `<span class="pill soon">em breve</span>`;
  return `
    <div class="${cls.join(' ')}" id="chip-${i}">
      ${pill}
      <div class="ico" style="background:${t.grad};box-shadow:0 0 14px ${t.accent}66">${t.initials}</div>
      <div class="meta">
        <div class="nm">${esc(g.display_name)}</div>
        <div class="tg">${esc(t.tag)}</div>
      </div>
    </div>`;
}

function hero(g: Card, t: Theme): string {
  const server = g.enabled
    ? `<div class="server">
         <span>Servidor: <b>${esc(g.name)}</b></span>
         ${g.server_ip ? `<span>IP: <b>${esc(g.server_ip)}</b></span>` : ''}
         <span>Status: <b style="color:#4ade80">Online</b></span>
       </div>`
    : `<div class="server"><span>Status: <b>Em breve</b></span></div>`;
  return `
    <div class="hero" style="--hero-grad:${t.grad}">
      <span class="tag">${esc(t.tag)}</span>
      <h1>${esc(g.display_name)}</h1>
      <div class="desc">${esc(g.description || '')}</div>
      ${server}
    </div>`;
}

function actionBtn(g?: Card): string {
  if (!g) return '';
  if (!g.enabled) return `<button class="btn" disabled>Indisponível</button>`;
  if (installing) return `<button class="btn primary" disabled>Instalando…</button>`;
  if (g.installed) return `<button class="btn primary" id="actionBtn">▶ JOGAR</button>`;
  return `<button class="btn primary" id="actionBtn">⬇ INSTALAR</button>`;
}

function footerLine(g?: Card): string {
  if (!g) return '';
  if (!g.enabled) return 'Este jogo ainda não está disponível.';
  if (g.installed) return 'Instalado — pronto para jogar.';
  return 'Clique em INSTALAR para baixar o client.';
}

async function doAction() {
  const g = games[selected];
  if (!g || !g.enabled || installing) return;

  if (g.installed) {
    try {
      await LaunchGame(g as any);
      toast('Abrindo o jogo…');
    } catch (e) {
      toast(String(e), true);
    }
    return;
  }

  installing = true;
  render();
  try {
    await InstallGame(g as any);
    g.installed = true;
    toast('Instalado com sucesso!');
  } catch (e) {
    toast(String(e), true);
  } finally {
    installing = false;
    render();
  }
}

EventsOn('install:progress', (p: { game: string; phase: string; percent: number }) => {
  const bar = document.getElementById('pbar') as HTMLElement | null;
  const line = document.getElementById('pline');
  if (bar) bar.style.width = `${p.percent}%`;
  if (line) {
    line.textContent =
      p.phase === 'download' ? `Baixando… ${p.percent}%` :
      p.phase === 'extract' ? `Extraindo… ${p.percent}%` :
      p.phase === 'done' ? 'Concluído!' : '';
  }
});

let toastTimer: number | undefined;
function toast(msg: string, err = false) {
  let el = document.getElementById('toast');
  if (!el) {
    el = document.createElement('div');
    el.id = 'toast';
    document.body.appendChild(el);
  }
  el.className = 'toast' + (err ? ' err' : '');
  el.textContent = msg;
  requestAnimationFrame(() => el!.classList.add('show'));
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => el!.classList.remove('show'), 3200);
}

function esc(s: string): string {
  return (s || '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]!));
}

load();
