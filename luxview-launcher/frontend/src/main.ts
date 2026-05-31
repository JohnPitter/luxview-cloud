import './style.css';
import './app.css';
import { GetGames, InstallGame, Play, GetSettings, SaveSettings, OpenInstallFolder } from '../wailsjs/go/main/App';
import { EventsOn, WindowMinimise, WindowToggleMaximise, Quit } from '../wailsjs/runtime/runtime';
import rakionImg from './assets/games/rakion.jpg';

// Arte de fundo do hero por jogo (quando houver; senão usa o gradiente do tema).
const IMAGES: Record<string, string> = {
  rakion: rakionImg,
};

type Card = {
  app_id: string;
  name: string;
  game: string;
  display_name: string;
  description: string;
  enabled: boolean;
  download_url: string;
  server_ip: string;
  auth_host: string;
  installed: boolean;
};

type Settings = {
  screen_width: number; screen_height: number; fullscreen: boolean;
  mouse_sensitivity: number; invert_mouse: boolean; mouse_accel: boolean;
  sound_volume: number; music_volume: number; gamma: number;
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

// Frases amigáveis (substituem a descrição técnica do servidor no hero).
const BLURBS: Record<string, string> = {
  rakion:  'Ação 3D em arenas frenéticas. Escolha seu mercenário e domine a batalha.',
  openmu:  'O MMORPG de ação clássico. Evolua seu herói, cace e enfrente chefes épicos.',
  muemu:   'O MMORPG de ação clássico. Evolua seu herói, cace e enfrente chefes épicos.',
  metin2:  'MMORPG de ação oriental com três reinos em guerra constante.',
  priston: 'MMORPG isométrico clássico, com caçadas intensas e bosses lendários.',
};
const blurb = (g: Card): string => BLURBS[g.game] || g.description || '';

// Nome amigável (sem sufixos técnicos tipo "(SoftNyx v258)").
const NAMES: Record<string, string> = {
  rakion: 'Rakion', openmu: 'Mu Online', muemu: 'Mu Online', metin2: 'Metin2', priston: 'Priston Tale',
};
const niceName = (g: Card): string => NAMES[g.game] || g.display_name;

function ph(game: string, name: string, desc: string): Card {
  return { app_id: '', name, game, display_name: name, description: desc, enabled: false, download_url: '', server_ip: '', auth_host: '', installed: false };
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
      <div class="topbar" style="--wails-draggable:drag">
        <div class="brand-mark">L</div>
        <div class="brand-text"><b>LuxView Cloud</b><span>Games</span></div>
        <div class="spacer"></div>
        <div class="status ${online ? '' : 'off'}"><span class="dot"></span>${online ? 'Conectado' : 'Offline'}</div>
        <div class="wctrls" style="--wails-draggable:no-drag">
          <button class="wbtn" id="winMin" title="Minimizar" aria-label="Minimizar"></button>
          <button class="wbtn" id="winMax" title="Maximizar" aria-label="Maximizar"></button>
          <button class="wbtn close" id="winClose" title="Fechar" aria-label="Fechar"></button>
        </div>
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

  document.getElementById('winMin')?.addEventListener('click', () => WindowMinimise());
  document.getElementById('winMax')?.addEventListener('click', () => WindowToggleMaximise());
  document.getElementById('winClose')?.addEventListener('click', () => Quit());
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
        <div class="nm">${esc(niceName(g))}</div>
        <div class="tg">${esc(t.tag)}</div>
      </div>
    </div>`;
}

function paintHero() {
  const g = games[selected];
  const host = document.getElementById('hero')!;
  if (!g) { host.innerHTML = '<div class="hero hero-empty">Nenhum jogo disponível</div>'; return; }
  const t = theme(g.game);
  const status = g.enabled
    ? `<div class="server"><span class="live">Online</span></div>`
    : `<div class="server"><span>Em breve</span></div>`;
  const img = IMAGES[g.game];
  host.innerHTML = `
    <div class="hero ${img ? 'has-img' : ''}" style="--hero-grad:${t.grad}">
      ${img ? `<div class="hero-bg" style="background-image:url('${img}')"></div>` : ''}
      <div class="glint"></div>
      <div class="c">
        <span class="tag">${esc(t.tag)}</span>
        <h1>${esc(niceName(g))}</h1>
        <div class="desc">${esc(blurb(g))}</div>
        ${status}
      </div>
    </div>`;
}

function paintFooter() {
  const g = games[selected];
  document.getElementById('pline')!.textContent = footerLine(g);
  const bar = document.getElementById('pbarwrap')!;
  const fill = document.getElementById('pbar') as HTMLElement;
  if (!installing) { bar.classList.remove('active', 'indet'); fill.style.width = '0%'; }
  const actions = document.getElementById('actions')!;
  actions.innerHTML = `
    ${g && g.installed ? `<button class="btn icon" id="optionsBtn" title="Opções (mouse, resolução, som)">⚙</button>` : ''}
    <button class="btn icon" id="folderBtn" title="Abrir pasta de instalação" ${g && g.installed ? '' : 'disabled'}>📁</button>
    ${actionBtn(g)}`;
  document.getElementById('actionBtn')?.addEventListener('click', doAction);
  document.getElementById('optionsBtn')?.addEventListener('click', () => { if (g) openOptions(g); });
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
    openLogin(g);
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

// ---------- modais (login + opções) ----------
function showModal(inner: string): HTMLElement {
  closeModal();
  const ov = document.createElement('div');
  ov.id = 'modal';
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal">${inner}</div>`;
  ov.addEventListener('mousedown', (e) => { if (e.target === ov) closeModal(); });
  document.body.appendChild(ov);
  requestAnimationFrame(() => ov.classList.add('show'));
  return ov;
}
function closeModal() { document.getElementById('modal')?.remove(); }

function openLogin(g: Card) {
  const lastUser = localStorage.getItem('luxview:user:' + g.game) || '';
  showModal(`
    <h3>Entrar — ${esc(niceName(g))}</h3>
    <div class="field"><label>Usuário</label><input id="loginUser" type="text" autocomplete="username" value="${esc(lastUser)}" maxlength="32"></div>
    <div class="field"><label>Senha</label><input id="loginPass" type="password" autocomplete="current-password" maxlength="32"></div>
    <div class="modal-err" id="loginErr"></div>
    <div class="modal-actions">
      <button class="btn" id="loginCancel">Cancelar</button>
      <button class="btn primary" id="loginGo">▶ Entrar e Jogar</button>
    </div>
    <div class="modal-hint">Não tem conta? Crie no painel admin do servidor.</div>
  `);
  const userEl = document.getElementById('loginUser') as HTMLInputElement;
  const passEl = document.getElementById('loginPass') as HTMLInputElement;
  const errEl = document.getElementById('loginErr')!;
  const goBtn = document.getElementById('loginGo') as HTMLButtonElement;
  (lastUser ? passEl : userEl).focus();

  const go = async () => {
    const user = userEl.value.trim();
    const pass = passEl.value;
    if (!user || !pass) { errEl.textContent = 'Informe usuário e senha.'; return; }
    goBtn.disabled = true; goBtn.innerHTML = '<span class="spinner"></span> Entrando…';
    errEl.textContent = '';
    try {
      await Play(g as any, user, pass);
      localStorage.setItem('luxview:user:' + g.game, user);
      closeModal();
      toast('Iniciando o jogo…');
    } catch (e) {
      errEl.textContent = String(e).replace(/^Error:\s*/, '');
      goBtn.disabled = false; goBtn.innerHTML = '▶ Entrar e Jogar';
    }
  };
  goBtn.onclick = go;
  passEl.onkeydown = (e) => { if (e.key === 'Enter') go(); };
  document.getElementById('loginCancel')!.onclick = closeModal;
}

const RESOLUTIONS = [[1280,720],[1366,768],[1600,900],[1920,1080],[2560,1440],[3840,2160]];

async function openOptions(g: Card) {
  let s: Settings;
  try { s = (await GetSettings(g as any)) as unknown as Settings; }
  catch (e) { toast(String(e), true); return; }
  const pct = (v: number) => Math.round(v * 100) + '%';
  const inRes = RESOLUTIONS.some(([w,h]) => w === s.screen_width && h === s.screen_height);
  showModal(`
    <h3>Opções — ${esc(niceName(g))}</h3>
    <div class="opt-grid">
      <label>Resolução</label>
      <select id="optRes">
        ${RESOLUTIONS.map(([w,h]) => `<option value="${w}x${h}" ${s.screen_width===w&&s.screen_height===h?'selected':''}>${w} × ${h}</option>`).join('')}
        ${inRes ? '' : `<option value="${s.screen_width}x${s.screen_height}" selected>${s.screen_width} × ${s.screen_height} (atual)</option>`}
      </select>
      <label>Modo de exibição</label>
      <select id="optDisp">
        <option value="full" ${s.fullscreen?'selected':''}>Tela cheia</option>
        <option value="window" ${!s.fullscreen?'selected':''}>Janela</option>
      </select>
      <label>Sensibilidade do mouse <b id="lSens">${s.mouse_sensitivity}</b></label>
      <input type="range" id="optSens" min="0.1" max="5" step="0.1" value="${s.mouse_sensitivity}">
      <label>Inverter mouse</label><label class="sw"><input type="checkbox" id="optInv" ${s.invert_mouse?'checked':''}><span></span></label>
      <label>Aceleração do mouse</label><label class="sw"><input type="checkbox" id="optAcc" ${s.mouse_accel?'checked':''}><span></span></label>
      <label>Volume — som <b id="lSnd">${pct(s.sound_volume)}</b></label>
      <input type="range" id="optSnd" min="0" max="1" step="0.01" value="${s.sound_volume}">
      <label>Volume — música <b id="lMus">${pct(s.music_volume)}</b></label>
      <input type="range" id="optMus" min="0" max="1" step="0.01" value="${s.music_volume}">
      <label>Brilho (gamma) <b id="lGam">${s.gamma}</b></label>
      <input type="range" id="optGam" min="0.5" max="2" step="0.05" value="${s.gamma}">
    </div>
    <div class="modal-err" id="optErr"></div>
    <div class="modal-actions">
      <button class="btn" id="optCancel">Cancelar</button>
      <button class="btn primary" id="optSave">Salvar</button>
    </div>
  `);
  const live = (id: string, lbl: string, fmt: (v: number) => string) => {
    const el = document.getElementById(id) as HTMLInputElement;
    const lb = document.getElementById(lbl)!;
    el.oninput = () => { lb.textContent = fmt(parseFloat(el.value)); };
  };
  live('optSens', 'lSens', (v) => String(v));
  live('optSnd', 'lSnd', (v) => Math.round(v*100) + '%');
  live('optMus', 'lMus', (v) => Math.round(v*100) + '%');
  live('optGam', 'lGam', (v) => String(v));

  document.getElementById('optCancel')!.onclick = closeModal;
  document.getElementById('optSave')!.onclick = async () => {
    const val = (id: string) => (document.getElementById(id) as HTMLInputElement);
    const [w, h] = val('optRes').value.split('x').map(Number);
    const out: Settings = {
      screen_width: w, screen_height: h,
      fullscreen: val('optDisp').value === 'full',
      mouse_sensitivity: parseFloat(val('optSens').value),
      invert_mouse: val('optInv').checked,
      mouse_accel: val('optAcc').checked,
      sound_volume: parseFloat(val('optSnd').value),
      music_volume: parseFloat(val('optMus').value),
      gamma: parseFloat(val('optGam').value),
    };
    try {
      await SaveSettings(g as any, out as any);
      closeModal();
      toast('Opções salvas!');
    } catch (e) {
      document.getElementById('optErr')!.textContent = String(e).replace(/^Error:\s*/, '');
    }
  };
}

EventsOn('install:progress', (p: { game: string; phase: string; percent: number; detail?: string }) => {
  const bar = document.getElementById('pbar') as HTMLElement | null;
  const wrap = document.getElementById('pbarwrap');
  const line = document.getElementById('pline');
  if (p.percent < 0) {
    wrap?.classList.add('indet');
    if (bar) bar.style.width = '';
  } else {
    wrap?.classList.remove('indet');
    if (bar) bar.style.width = `${Math.max(2, p.percent)}%`;
  }
  if (line) {
    line.textContent =
      p.phase === 'download' ? (p.detail ? `Baixando… ${p.detail}` : 'Baixando client…') :
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
