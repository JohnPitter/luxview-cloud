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

type Theme = { grad: string; accent: string; tag: string; initials: string };

const THEMES: Record<string, Theme> = {
  rakion:   { grad: 'linear-gradient(135deg,#7f1d1d 0%,#2a0a0a 100%)', accent: '#e0392b', tag: 'Chaos Force', initials: 'R' },
  openmu:   { grad: 'linear-gradient(135deg,#581c87 0%,#1b0a2e 100%)', accent: '#a855f7', tag: 'MMORPG', initials: 'M' },
  muemu:    { grad: 'linear-gradient(135deg,#581c87 0%,#1b0a2e 100%)', accent: '#a855f7', tag: 'MMORPG', initials: 'M' },
  metin2:   { grad: 'linear-gradient(135deg,#7c2d12 0%,#2a1505 100%)', accent: '#fb923c', tag: 'MMORPG', initials: 'M2' },
  priston:  { grad: 'linear-gradient(135deg,#0e7490 0%,#082530 100%)', accent: '#22d3ee', tag: 'Tale', initials: 'PT' },
};
const FALLBACK: Theme = { grad: 'linear-gradient(135deg,#3f3f46 0%,#18181b 100%)', accent: '#71717a', tag: 'Em breve', initials: '?' };
const theme = (g: string): Theme => THEMES[g] || FALLBACK;

function ph(game: string, name: string, desc: string): Card {
  return { app_id: '', name, game, display_name: name, description: desc, enabled: false, download_url: '', server_ip: '', installed: false };
}
// Próximos jogos da LuxView Cloud (cinza até ter servidor deployado + listado).
const PLACEHOLDERS: Card[] = [
  ph('openmu', 'Mu Online', 'MMORPG de ação clássico (OpenMU). Em breve na LuxView Cloud.'),
  ph('metin2', 'Metin2', 'MMORPG de ação oriental. Em breve na LuxView Cloud.'),
  ph('priston', 'Priston Tale', 'MMORPG isométrico clássico. Em breve na LuxView Cloud.'),
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
  mount();
  paintChips();
  paintHero();
  paintFooter();
}

function mount() {
  app.innerHTML = `
    <div class="shell">
      <div class="topbar">
        <div class="brand-mark">L</div>
        <div class="brand-text"><b>LuxView Cloud</b><span>Games</span></div>
        <div class="spacer"></div>
        <div class="status ${online ? '' : 'off'}"><span class="dot"></span>${online ? 'Conectado' : 'Offline'}</div>
      </div>
      <div class="strip" id="strip"></div>
      <div class="hero-wrap" id="hero"></div>
      <div class="footer">
        <div class="progress-wrap">
          <div class="progress-line" id="pline"></div>
          <div class="bar" id="pbarwrap"><i id="pbar"></i></div>
        </div>
        <div class="actions" id="actions"></div>
      </div>
    </div>`;
}

function paintChips() {
  const strip = document.getElementById('strip')!;
  strip.innerHTML = games.map((g, i) => chip(g, i)).join('');
  games.forEach((_, i) => {
    const el = document.getElementById('chip-' + i)!;
    el.style.animationDelay = `${i * 70}ms`;
    el.addEventListener('click', () => {
      if (installing || i === selected) return;
      selected = i;
      document.querySelectorAll('.chip').forEach((c, j) => c.classList.toggle('selected', j === selected));
      paintHero();
      paintFooter();
    });
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
      <div class="ico" style="background:${t.grad};box-shadow:0 0 16px ${t.accent}66">${t.initials}</div>
      <div class="meta">
        <div class="nm">${esc(g.display_name)}</div>
        <div class="tg">${esc(t.tag)}</div>
      </div>
    </div>`;
}

function paintHero() {
  const g = games[selected];
  const host = document.getElementById('hero')!;
  if (!g) { host.innerHTML = '<div class="hero hero-empty">Nenhum jogo disponível</div>'; return; }
  const t = theme(g.game);
  const server = g.enabled
    ? `<div class="server">
         <span>Servidor: <b>${esc(g.name)}</b></span>
         ${g.server_ip ? `<span>IP: <b>${esc(g.server_ip)}</b></span>` : ''}
         <span class="live">Online</span>
       </div>`
    : `<div class="server"><span>Status: <b>Em breve</b></span></div>`;
  host.innerHTML = `
    <div class="hero" style="--hero-grad:${t.grad}">
      <div class="glint"></div>
      <div class="c">
        <span class="tag">${esc(t.tag)}</span>
        <h1>${esc(g.display_name)}</h1>
        <div class="desc">${esc(g.description || '')}</div>
        ${server}
      </div>
    </div>`;
}

function paintFooter() {
  const g = games[selected];
  document.getElementById('pline')!.textContent = footerLine(g);
  const bar = document.getElementById('pbarwrap')!;
  const fill = document.getElementById('pbar') as HTMLElement;
  if (!installing) { bar.classList.remove('active'); fill.style.width = '0%'; }
  const actions = document.getElementById('actions')!;
  actions.innerHTML = `
    <button class="btn icon" id="folderBtn" title="Abrir pasta de instalação" ${g && g.installed ? '' : 'disabled'}>📁</button>
    ${actionBtn(g)}`;
  document.getElementById('actionBtn')?.addEventListener('click', doAction);
  document.getElementById('folderBtn')?.addEventListener('click', () => { if (g && g.app_id) OpenInstallFolder(g.app_id); });
}

function actionBtn(g?: Card): string {
  if (!g) return '';
  if (!g.enabled) return `<button class="btn" disabled>Indisponível</button>`;
  if (installing) return `<button class="btn primary" disabled><span class="spinner"></span> Instalando…</button>`;
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
    try { await LaunchGame(g as any); toast('Abrindo o jogo…'); }
    catch (e) { toast(String(e), true); }
    return;
  }

  installing = true;
  paintFooter();
  document.getElementById('pbarwrap')!.classList.add('active');
  try {
    await InstallGame(g as any);
    g.installed = true;
    toast('Instalado com sucesso!');
  } catch (e) {
    toast(String(e), true);
  } finally {
    installing = false;
    paintFooter();
    paintChips();
  }
}

EventsOn('install:progress', (p: { game: string; phase: string; percent: number }) => {
  const bar = document.getElementById('pbar') as HTMLElement | null;
  const line = document.getElementById('pline');
  if (bar) bar.style.width = `${Math.max(2, p.percent)}%`;
  if (line) {
    line.textContent =
      p.phase === 'download' ? (p.percent > 0 ? `Baixando… ${p.percent}%` : 'Baixando client…') :
      p.phase === 'extract' ? `Extraindo… ${p.percent}%` :
      p.phase === 'done' ? 'Concluído!' : '';
  }
});

let toastTimer: number | undefined;
function toast(msg: string, err = false) {
  let el = document.getElementById('toast');
  if (!el) { el = document.createElement('div'); el.id = 'toast'; document.body.appendChild(el); }
  el.className = 'toast' + (err ? ' err' : '');
  el.textContent = msg;
  // restart animation
  el.classList.remove('show'); void el.offsetWidth; el.classList.add('show');
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => el!.classList.remove('show'), 3200);
}

function esc(s: string): string {
  return (s || '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]!));
}

load();
