import './style.css';
import './app.css';
import { GetGames, InstallGame, Play, PlayMu, GetMuServers, GetSettings, SaveSettings, GetMetin2Settings, SaveMetin2Settings, OpenInstallFolder, Version, IsGameRunning, IsInstalled, CheckForUpdate, ApplyUpdate, PlayerRegister, PlayerLogin, PlayerMe, PlayerLogout, CommunitySnapshot, CommunitySend, CommunityHere, ShopCatalog, ShopBuy } from '../wailsjs/go/main/App';
import { EventsOn, WindowMinimise, WindowToggleMaximise, Quit } from '../wailsjs/runtime/runtime';
import rakionImg from './assets/games/rakion.jpg';
import muImg from './assets/games/mu.jpg';
import metin2Img from './assets/games/metin2.jpg';
import tibiaImg from './assets/games/tibia.jpg';
import pristonImg from './assets/games/priston.jpg';

// Arte por jogo, usada no hero E no ícone do chip (quando houver; senão gradiente).
const IMAGES: Record<string, string> = {
  rakion: rakionImg,
  openmu: muImg,
  muemu: muImg,
  metin2: metin2Img,
  tibia: tibiaImg,
  priston: pristonImg,
};

type MuServerGroupMeta = {
  group: number; name: string; difficulty: string;
  channels?: { id: number; mode: string }[];
};

type Card = {
  app_id: string;
  name: string;
  game: string;
  display_name: string;
  description: string;
  enabled: boolean;
  download_url: string;
  patch_url?: string;
  base_url?: string;
  server_ip: string;
  auth_host: string;
  client_hash: string;
  base_hash?: string;
  server_groups?: MuServerGroupMeta[];
  installed: boolean;
  update_available: boolean;
};

type Settings = {
  screen_width: number; screen_height: number; display_mode: string;
  mouse_sensitivity: number; invert_mouse: boolean; mouse_accel: boolean;
  sound_volume: number; music_volume: number; gamma: number;
};

type Metin2Settings = {
  screen_width: number; screen_height: number; bpp: number; frequency: number;
  windowed: boolean; software_cursor: boolean; object_culling: boolean; visibility: number;
  music_volume: number; sound_volume: number; gamma: number; pre_loading_delay: number;
  decompressed_texture: boolean; always_view_name: boolean; show_refine_dialog: boolean;
  fog_mode: boolean; night_mode: boolean; snow_mode: boolean; snow_texture: boolean;
  show_mob_level: boolean; show_mob_ai_flag: boolean; auto_pickup: boolean; extended_fov: boolean;
  effect_level: number; private_shop_level: number; drop_item_level: number;
  pet_status: boolean; npc_name_status: boolean; show_dice_info: boolean; poly_dog_mode: boolean;
  premium_affect: boolean; time_system: boolean; enb_mode_status: boolean; use_default_ime: boolean;
  software_tiling: number; shadow_level: number;
};

type Theme = { grad: string; accent: string; tag: string; initials: string };

const gameId = (raw: string = ''): string => {
  const id = raw.trim().toLowerCase().replaceAll('_', '-');
  if (id.includes('tibia')) return 'tibia';
  if (id.includes('priston')) return 'priston';
  if (id === 'openmu' || id === 'muemu') return 'muemu';
  return id;
};

const gameFamily = (raw: string = ''): string => {
  const id = gameId(raw);
  if (id === 'muemu') return 'mu';
  return id;
};

const normalizeCard = (card: Card): Card => ({
  ...card,
  game: gameId(card.game || card.display_name || card.name),
});

const cardGame = (card: Card): string => gameId(card.game || card.display_name || card.name);

const THEMES: Record<string, Theme> = {
  rakion:   { grad: 'linear-gradient(135deg,#7f1d1d 0%,#2a0a0a 100%)', accent: '#e0392b', tag: 'Chaos Force', initials: 'R' },
  openmu:   { grad: 'linear-gradient(135deg,#581c87 0%,#1b0a2e 100%)', accent: '#a855f7', tag: 'Season 99d, 2 e 6 pt 3', initials: 'M' },
  muemu:    { grad: 'linear-gradient(135deg,#581c87 0%,#1b0a2e 100%)', accent: '#a855f7', tag: 'Season 99d, 2 e 6 pt 3', initials: 'M' },
  metin2:   { grad: 'linear-gradient(135deg,#7c2d12 0%,#2a1505 100%)', accent: '#fb923c', tag: 'MMORPG', initials: 'M2' },
  tibia:    { grad: 'linear-gradient(135deg,#14532d 0%,#052e16 100%)', accent: '#fbbf24', tag: 'MMORPG', initials: 'T' },
  priston:  { grad: 'linear-gradient(135deg,#0e7490 0%,#082530 100%)', accent: '#22d3ee', tag: 'Tale', initials: 'PT' },
};
const FALLBACK: Theme = { grad: 'linear-gradient(135deg,#3f3f46 0%,#18181b 100%)', accent: '#71717a', tag: 'Em breve', initials: '?' };
const theme = (g: string): Theme => THEMES[gameId(g)] || FALLBACK;

// Frases amigáveis (substituem a descrição técnica do servidor no hero).
const BLURBS: Record<string, string> = {
  rakion:  'Ação 3D em arenas frenéticas. Escolha seu mercenário e domine a batalha.',
  openmu:  'Season 99d, 2 e 6 pt 3 — entre, lute e evolua. O launcher já conecta no servidor LuxView.',
  muemu:   'Season 99d, 2 e 6 pt 3 — entre, lute e evolua. O launcher já conecta no servidor LuxView.',
  metin2:  'MMORPG de ação oriental com três reinos em guerra constante.',
  tibia:   'Aventure-se em um mundo de fantasia medieval: escolha sua vocação (cavaleiro, paladino, druida ou mago) e enfrente monstros, explore masmorras, complete quests épicas e domine o combate por turnos com magias e feitiços.',
  priston: 'MMORPG isométrico clássico, com caçadas intensas e bosses lendários.',
};
const blurb = (g: Card): string => BLURBS[cardGame(g)] || g.description || '';

// Nome amigável (sem sufixos técnicos tipo "(SoftNyx v258)").
const NAMES: Record<string, string> = {
  rakion: 'Rakion', openmu: 'Mu Online', muemu: 'Mu Online', metin2: 'Metin2', tibia: 'Tibia', priston: 'Priston Tale',
};
const niceName = (g: Card): string => NAMES[cardGame(g)] || g.display_name;

function ph(game: string, name: string, desc: string): Card {
  return { app_id: '', name, game, display_name: name, description: desc, enabled: false, download_url: '', server_ip: '', auth_host: '', client_hash: '', installed: false, update_available: false };
}
// Próximos jogos da LuxView Cloud (cinza até ter servidor deployado + listado).
const PLACEHOLDERS: Card[] = [
  ph('metin2', 'Metin2', 'MMORPG de ação oriental. Em breve na LuxView Cloud.'),
];

type CommunityPost = {
  id: string; app_id: string; game: string; game_name: string; display_name: string;
  title: string; body: string; created_at: string;
};
type CommunityMessage = { id: string; username: string; body: string; created_at: string };
type CommunityGame = {
  app_id: string; game: string; name: string; display_name: string;
  players: number; max_players: number;
};
type Snapshot = {
  players_online: number; chat_online: number;
  games: CommunityGame[]; posts: CommunityPost[]; chat: CommunityMessage[];
};

let games: Card[] = [];
let selected = 0;
let communityOpen = false;
let community: Snapshot | null = null;
let communityTimer: number | undefined;
let communitySending = false;
let installing = false;
let online = false;
let version = '';
let runningGame = ''; // game id em execução (botão "● Em execução")
let loadingGame = ''; // game id carregando (botão "Carregando Rakion…")
let update: { version: string; url: string; notes: string } | null = null; // nova versão disponível
let updating = false; // aplicando atualização
let player: { username: string; cash_points: number; reward_points: number } | null = null;

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// monitorGame mantém o botão ocupado do clique em Jogar até o jogo FECHAR, em 2
// fases: o driver desempacota o load.bin + faz checks de MD5 antes de lançar o
// rakion.bin (~15-25s), então (1) mostramos "Carregando Rakion…" até o processo
// APARECER, e (2) "● Em execução" até ele SUMIR. Assim o botão não re-habilita no
// meio do startup.
async function monitorGame(g: Card) {
  loadingGame = g.game; // fase 1: carregando
  runningGame = '';
  paintFooter();
  try {
    let appeared = false;
    for (let i = 0; i < 90; i++) {
      if (await IsGameRunning(g.game)) { appeared = true; break; }
      await sleep(1000);
    }
    if (appeared) {
      loadingGame = '';
      runningGame = g.game; // fase 2: em execução
      paintFooter();
      while (await IsGameRunning(g.game)) await sleep(3000);
    }
  } catch { /* ignore */ }
  loadingGame = '';
  runningGame = '';
  paintFooter();
}

const app = document.querySelector<HTMLDivElement>('#app')!;

async function load() {
  try { version = await Version(); } catch { /* binding nova */ }
  mount();
  paintChips();
  paintHero();
  paintFooter();
  try {
    applyCatalog(((await GetGames()) as unknown as Card[]).map(normalizeCard), true);
  } catch (e) {
    online = false;
    games = [...PLACEHOLDERS];
    selected = 0;
    toast(String(e), true);
  }
  paintChips();
  paintHero();
  paintFooter();
  paintStatus();
  void checkUpdate(true);
  startUpdateWatcher();
  void refreshPlayer();
}

function applyCatalog(real: Card[], pickDefault: boolean) {
  online = true;
  const keepCommunity = !pickDefault && communityOpen;
  const keepID = !pickDefault && games[selected] ? games[selected].app_id : '';
  const have = new Set(real.map((r) => gameFamily(r.game)));
  games = [...real, ...PLACEHOLDERS.filter((p) => !have.has(gameFamily(p.game)))];
  if (games.length === 0) games = [...PLACEHOLDERS];
  if (keepCommunity) {
    communityOpen = true;
    return;
  }
  const kept = keepID ? games.findIndex((g) => g.app_id === keepID) : -1;
  if (kept >= 0) {
    selected = kept;
    return;
  }
  const firstEnabled = games.findIndex((g) => g.enabled);
  selected = firstEnabled >= 0 ? firstEnabled : 0;
}

async function refreshCatalog() {
  if (installing) return;
  try {
    applyCatalog(((await GetGames()) as unknown as Card[]).map(normalizeCard), false);
    paintChips();
    paintHero();
    paintFooter();
    paintStatus();
  } catch { /* offline — ignora */ }
}

function paintStatus() {
  const el = document.querySelector('.topbar .status');
  if (!el) return;
  el.className = 'status' + (online ? '' : ' off');
  el.innerHTML = `<span class="dot"></span>${online ? 'Conectado' : 'Offline'}`;
  const acct = document.getElementById('acctBtn');
  if (acct) acct.textContent = player ? player.username : 'Conta';
}

async function refreshPlayer() {
  try {
    const me = await PlayerMe();
    player = me && me.username ? { username: me.username, cash_points: me.cash_points, reward_points: me.reward_points } : null;
  } catch {
    player = null;
  }
  paintStatus();
}

function fmtPoints(n: number) {
  return new Intl.NumberFormat('pt-BR').format(n ?? 0);
}

function iconCoin() {
  return `<svg class="acct-svg" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9.2" fill="#f59e0b"/><circle cx="12" cy="12" r="7.1" fill="#fbbf24"/><circle cx="12" cy="12" r="5.2" fill="none" stroke="#92400e" stroke-width="1.3"/><path d="M12 8.2v7.6M10.1 9.6c.5-.5 1.2-.8 1.9-.8 1.4 0 2.2.8 2.2 1.8 0 2.4-4.2 1.3-4.2 3.4 0 1 .9 1.8 2.2 1.8.8 0 1.5-.3 2-.8" fill="none" stroke="#78350f" stroke-width="1.35" stroke-linecap="round"/></svg>`;
}

function iconGem() {
  return `<svg class="acct-svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3.4 4.6 9.2 12 20.6 19.4 9.2Z" fill="#a78bfa"/><path d="M12 3.4 8.2 9.2h7.6Z" fill="#c4b5fd"/><path d="M4.6 9.2h14.8L12 20.6Z" fill="#7c3aed"/><path d="M8.2 9.2 12 20.6 15.8 9.2Z" fill="#8b5cf6"/></svg>`;
}

function openPlayerAccount() {
  if (player) {
    const initial = (player.username.trim()[0] || '?').toUpperCase();
    showModal(`
      <h3>Conta LuxView</h3>
      <div class="acct-panel">
        <div class="acct-profile">
          <div class="acct-avatar">${esc(initial)}</div>
          <div class="acct-who">
            <b>${esc(player.username)}</b>
            <span>Jogador</span>
          </div>
        </div>
        <div class="acct-wallet">
          <div class="acct-bal cash">
            <span class="acct-ico">${iconCoin()}</span>
            <div class="acct-amt">
              <span>Cash</span>
              <b id="acctCash">${fmtPoints(player.cash_points)}</b>
            </div>
          </div>
          <div class="acct-bal prize">
            <span class="acct-ico">${iconGem()}</span>
            <div class="acct-amt">
              <span>Prêmios</span>
              <b id="acctPrize">${fmtPoints(player.reward_points)}</b>
            </div>
          </div>
        </div>
        <div class="acct-shop" id="acctShop"><p class="acct-shop-empty">Carregando loja…</p></div>
      </div>
      <div class="modal-actions">
        <button class="btn" id="acctOut">Sair</button>
        <button class="btn primary" id="acctOk">Ok</button>
      </div>
    `);
    document.getElementById('acctOk')!.onclick = closeModal;
    document.getElementById('acctOut')!.onclick = async () => {
      try { await PlayerLogout(); } catch { /* ignore */ }
      player = null;
      closeModal();
      paintStatus();
      paintFooter();
      if (communityOpen) paintHero();
    };
    void fillAccountShop();
    return;
  }
  showModal(`
    <h3>Conta LuxView</h3>
    <p class="modal-hint">Faça login ou crie uma conta para jogar e usar a comunidade.</p>
    <div class="field"><label>Usuário</label><input id="pUser" type="text" maxlength="32"></div>
    <div class="field"><label>Senha</label><input id="pPass" type="password" maxlength="64"></div>
    <div class="modal-err" id="pErr"></div>
    <div class="modal-actions">
      <button class="btn" id="pReg">Criar conta</button>
      <button class="btn primary" id="pGo">Entrar</button>
    </div>
  `);
  const userEl = document.getElementById('pUser') as HTMLInputElement;
  const passEl = document.getElementById('pPass') as HTMLInputElement;
  const errEl = document.getElementById('pErr')!;
  const apply = async (fn: typeof PlayerLogin) => {
    errEl.textContent = '';
    try {
      const me = await fn(userEl.value.trim(), passEl.value);
      player = { username: me.username, cash_points: me.cash_points, reward_points: me.reward_points };
      closeModal();
      paintStatus();
      paintFooter();
      if (communityOpen) void refreshCommunity();
      toast('Conta conectada.');
    } catch (e) {
      errEl.textContent = String(e).replace(/^Error:\s*/, '');
    }
  };
  document.getElementById('pGo')!.onclick = () => apply(PlayerLogin);
  document.getElementById('pReg')!.onclick = () => apply(PlayerRegister);
}

async function fillAccountShop() {
  const box = document.getElementById('acctShop');
  if (!box) return;
  const g = games[selected];
  if (!g) {
    box.innerHTML = `<p class="acct-shop-empty">Nenhum servidor no catálogo.</p>`;
    return;
  }
  try {
    const items = await ShopCatalog(g.game);
    if (!items.length) {
      box.innerHTML = `<p class="acct-shop-empty">A loja do ${esc(g.display_name || g.game)} ainda não entrega itens neste servidor.</p>`;
      return;
    }
    box.innerHTML = `
      <div class="acct-shop-head">Loja · ${esc(g.display_name || g.game)}</div>
      ${items.map((item) => `
        <button class="acct-item" data-item="${esc(item.id)}" type="button">
          <span class="acct-item-ico">${item.currency === 'reward' ? iconGem() : iconCoin()}</span>
          <span class="acct-item-info">
            <b>${esc(item.name)}</b>
            <small>${esc(item.description)}</small>
          </span>
          <span class="acct-item-price">${fmtPoints(item.price)}</span>
        </button>
      `).join('')}
      <div class="modal-err" id="acctShopErr"></div>
    `;
    box.querySelectorAll<HTMLButtonElement>('.acct-item').forEach((btn) => {
      btn.onclick = async () => {
        const errEl = document.getElementById('acctShopErr');
        if (errEl) errEl.textContent = '';
        btn.disabled = true;
        try {
          const result = await ShopBuy(g.app_id, btn.dataset.item || '');
          if (player) {
            player.cash_points = result.cash_points;
            player.reward_points = result.reward_points;
          }
          const cashEl = document.getElementById('acctCash');
          const prizeEl = document.getElementById('acctPrize');
          if (cashEl) cashEl.textContent = fmtPoints(result.cash_points);
          if (prizeEl) prizeEl.textContent = fmtPoints(result.reward_points);
          toast(`${result.item.name} entregue no servidor.`);
        } catch (e) {
          if (errEl) errEl.textContent = String(e).replace(/^Error:\s*/, '');
        } finally {
          btn.disabled = false;
        }
      };
    });
  } catch (e) {
    box.innerHTML = `<p class="acct-shop-empty">${esc(String(e).replace(/^Error:\s*/, ''))}</p>`;
  }
}

function mount() {
  app.innerHTML = `
    <div class="shell">
      <div class="topbar" style="--wails-draggable:drag">
        <div class="brand-mark">L</div>
        <div class="brand-text"><b>LuxView Cloud</b><span>Games</span></div>
        <div class="spacer"></div>
        <div class="status ${online ? '' : 'off'}"><span class="dot"></span>${online ? 'Conectado' : 'Offline'}</div>
        <button class="acct" id="acctBtn" style="--wails-draggable:no-drag">${player ? esc(player.username) : 'Conta'}</button>
        ${version ? `<span class="ver">${esc(version)}</span>` : ''}
        <div class="wctrls" style="--wails-draggable:no-drag">
          <button class="wbtn" id="winMin" title="Minimizar" aria-label="Minimizar"></button>
          <button class="wbtn" id="winMax" title="Maximizar" aria-label="Maximizar"></button>
          <button class="wbtn close" id="winClose" title="Fechar" aria-label="Fechar"></button>
        </div>
      </div>
      <div class="update-bar" id="updatebar"></div>
      <div class="carousel">
        <button class="car-btn" id="carPrev" title="Anterior" aria-label="Anterior">‹</button>
        <div class="strip" id="strip"></div>
        <button class="car-btn" id="carNext" title="Próximo" aria-label="Próximo">›</button>
      </div>
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
  document.getElementById('carPrev')?.addEventListener('click', () => scrollCarousel(-1));
  document.getElementById('carNext')?.addEventListener('click', () => scrollCarousel(1));
  document.getElementById('acctBtn')?.addEventListener('click', openPlayerAccount);
  document.getElementById('strip')?.addEventListener('scroll', updateCarousel, { passive: true });
  window.removeEventListener('resize', updateCarousel);
  window.addEventListener('resize', updateCarousel);
}

function paintUpdate() {
  const bar = document.getElementById('updatebar');
  if (!bar) return;
  if (!update) { bar.className = 'update-bar'; bar.innerHTML = ''; return; }
  bar.className = 'update-bar show';
  bar.innerHTML = updating
    ? `<span class="spinner"></span><span class="ut">Atualizando o launcher…</span>`
    : `<span class="ub">●</span>
       <span class="ut">Nova versão <b>${esc(update.version)}</b> disponível.</span>
       <button class="ubtn" id="doUpdate">Atualizar agora</button>`;
  if (!updating) {
    document.getElementById('doUpdate')?.addEventListener('click', applyUpdate);
  }
}

let lastUpdateCheck = 0;

async function checkUpdate(force = false) {
  if (updating) return;
  // Debounce: no máx. uma checagem a cada 60s (evita spam em foco/intervalo),
  // a não ser que seja forçada.
  const now = Date.now();
  if (!force && now - lastUpdateCheck < 60_000) return;
  lastUpdateCheck = now;
  try {
    const info = await CheckForUpdate();
    if (info && info.available) {
      update = { version: info.version, url: info.url, notes: info.notes };
      paintUpdate();
    }
  } catch { /* offline ou sem release — ignora */ }
}

// Não depende só do startup: re-checa periodicamente e quando a janela volta ao
// foco, pra que uma release nova seja notada sem precisar reabrir o launcher.
let updateWatcherStarted = false;
function startUpdateWatcher() {
  if (updateWatcherStarted) return;
  updateWatcherStarted = true;
  // Catálogo (client_hash / ATUALIZAR) a cada 60s; release do launcher a cada 10min.
  setInterval(() => { void refreshCatalog(); }, 60_000);
  setInterval(() => { void checkUpdate(); }, 10 * 60_000);
  window.addEventListener('focus', () => { void checkUpdate(true); void refreshCatalog(); });
}

async function applyUpdate() {
  if (!update || updating) return;
  updating = true;
  paintUpdate();
  try {
    await ApplyUpdate(update.url); // o backend reinicia o app ao concluir
  } catch (e) {
    updating = false;
    paintUpdate();
    toast(String(e), true);
  }
}

function paintChips() {
  const strip = document.getElementById('strip');
  if (!strip) return;
  strip.innerHTML = games.map((g, i) => chip(g, i)).join('') + communityChip();
  games.forEach((_, i) => {
    const el = document.getElementById('chip-' + i);
    if (!el) return;
    el.style.animationDelay = `${i * 70}ms`;
    el.addEventListener('click', () => {
      if (installing || (i === selected && !communityOpen)) return;
      communityOpen = false;
      stopCommunity();
      selected = i;
      document.querySelectorAll('.chip').forEach((c, j) => c.classList.toggle('selected', j === selected));
      paintHero();
      paintFooter();
    });
  });
  const comm = document.getElementById('chip-community');
  if (comm) {
    comm.style.animationDelay = `${games.length * 70}ms`;
    comm.addEventListener('click', () => {
      if (installing || communityOpen) return;
      openCommunity();
    });
  }
  requestAnimationFrame(() => requestAnimationFrame(updateCarousel));
}

// scrollCarousel rola o strip por uma largura de chip (com o gap).
function scrollCarousel(dir: number) {
  const stripEl = document.getElementById('strip');
  if (!stripEl) return;
  const chipEl = stripEl.querySelector<HTMLElement>('.chip');
  const step = chipEl ? chipEl.offsetWidth + 10 : 160;
  stripEl.scrollBy({ left: dir * step, behavior: 'smooth' });
}

// updateCarousel só mostra as setas quando os chips transbordam o strip.
// Mede sem as setas, senão elas mesmas criam o overflow que depois "justificam".
function updateCarousel() {
  const carousel = document.querySelector('.carousel');
  const stripEl = document.getElementById('strip');
  const prev = document.getElementById('carPrev') as HTMLButtonElement | null;
  const next = document.getElementById('carNext') as HTMLButtonElement | null;
  if (!carousel || !stripEl) return;
  carousel.classList.remove('overflow');
  void stripEl.offsetWidth;
  const overflowing = stripEl.scrollWidth > stripEl.clientWidth + 1;
  carousel.classList.toggle('overflow', overflowing);
  if (!overflowing) return;
  const maxScroll = stripEl.scrollWidth - stripEl.clientWidth - 1;
  if (prev) prev.disabled = stripEl.scrollLeft <= 1;
  if (next) next.disabled = stripEl.scrollLeft >= maxScroll;
}

function chip(g: Card, i: number): string {
  const id = cardGame(g);
  const t = theme(id);
  const cls = ['chip'];
  if (i === selected && !communityOpen) cls.push('selected');
  if (!g.enabled) cls.push('disabled');
  const pill = !g.enabled
    ? `<span class="pill soon">em breve</span>`
    : g.update_available
      ? `<span class="pill upd">atualização</span>`
      : `<span class="pill on">online</span>`;
  const img = IMAGES[id];
  const ico = img
    ? `<div class="ico img" style="background-image:url('${img}');box-shadow:0 0 16px ${t.accent}66"></div>`
    : `<div class="ico" style="background:${t.grad};box-shadow:0 0 16px ${t.accent}66">${t.initials}</div>`;
  return `
    <div class="${cls.join(' ')}" id="chip-${i}">
      ${pill}
      ${ico}
      <div class="meta">
        <div class="nm">${esc(niceName(g))}</div>
        <div class="tg">${esc(t.tag)}</div>
      </div>
    </div>`;
}

function communityChip(): string {
  const cls = ['chip', 'community-chip'];
  if (communityOpen) cls.push('selected');
  return `
    <div class="${cls.join(' ')}" id="chip-community">
      <span class="pill on">live</span>
      <div class="ico" style="background:linear-gradient(135deg,#b45309 0%,#1c1917 100%);box-shadow:0 0 16px #f59e0b66">⌂</div>
      <div class="meta">
        <div class="nm">Comunidade</div>
        <div class="tg">Eventos & chat</div>
      </div>
    </div>`;
}

function paintHero() {
  const host = document.getElementById('hero')!;
  if (communityOpen) {
    paintCommunity(host);
    return;
  }
  const g = games[selected];
  if (!g) { host.innerHTML = '<div class="hero hero-empty">Conectando à LuxView Cloud…</div>'; return; }
  const id = cardGame(g);
  const t = theme(id);
  const status = g.enabled
    ? `<div class="server"><span class="live">Online</span></div>`
    : `<div class="server"><span>Em breve</span></div>`;
  const img = IMAGES[id];
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

function openCommunity() {
  communityOpen = true;
  document.querySelectorAll('.chip').forEach((c) => c.classList.remove('selected'));
  document.getElementById('chip-community')?.classList.add('selected');
  paintHero();
  paintFooter();
  startCommunity();
}

function startCommunity() {
  stopCommunity();
  void refreshCommunity();
  communityTimer = window.setInterval(() => { void refreshCommunity(); }, 3000);
}

function stopCommunity() {
  if (communityTimer) {
    clearInterval(communityTimer);
    communityTimer = undefined;
  }
}

async function refreshCommunity() {
  if (!communityOpen) return;
  try {
    community = await CommunitySnapshot() as unknown as Snapshot;
    if (player) {
      try { await CommunityHere(); } catch { /* sessão expirada — o chat pede login */ }
    }
  } catch (e) {
    if (!community) toast(String(e), true);
  }
  if (communityOpen) {
    paintHero();
    paintFooter();
  }
}

let chatDraft = '';

function gameLabel(game: string, fallback: string): string {
  return NAMES[gameId(game)] || fallback || game;
}

function ago(raw: string): string {
  const then = new Date(raw).getTime();
  if (!Number.isFinite(then)) return '';
  const min = Math.max(0, Math.floor((Date.now() - then) / 60_000));
  if (min < 1) return 'agora';
  if (min < 60) return `${min} min`;
  const hours = Math.floor(min / 60);
  if (hours < 24) return `${hours} h`;
  return `${Math.floor(hours / 24)} d`;
}

function paintCommunity(host: HTMLElement) {
  const inputEl = document.getElementById('commInput') as HTMLInputElement | null;
  if (inputEl) chatDraft = inputEl.value;
  const keepFocus = document.activeElement === inputEl;
  const chatEl = document.getElementById('commChat');
  const stickBottom = chatEl ? chatEl.scrollHeight - chatEl.scrollTop - chatEl.clientHeight < 48 : true;

  const snap = community;
  const players = snap?.players_online ?? 0;
  const chatting = snap?.chat_online ?? 0;
  const gamesOnline = snap?.games || [];
  const posts = snap?.posts || [];
  const msgs = snap?.chat || [];

  const perGame = gamesOnline.length
    ? gamesOnline.map((g) => {
        const cap = g.max_players > 0 ? `/${g.max_players}` : '';
        return `<span class="comm-pill">${esc(gameLabel(g.game, g.display_name))} <b>${g.players}${cap}</b></span>`;
      }).join('')
    : `<span class="comm-muted">Nenhum servidor listado ainda.</span>`;

  const feed = posts.length
    ? posts.map((p) => `
        <article class="comm-post">
          <div class="comm-post-head">
            <span class="comm-game">${esc(gameLabel(p.game, p.display_name || p.game_name))}</span>
            <span class="comm-time">${esc(ago(p.created_at))}</span>
          </div>
          <h3>${esc(p.title)}</h3>
          <p>${esc(p.body)}</p>
        </article>`).join('')
    : `<div class="comm-empty">Nenhum evento publicado ainda. Os donos dos servidores postam da aplicação do game no painel.</div>`;

  const chat = msgs.length
    ? msgs.map((m) => `
        <div class="comm-msg ${player && m.username === player.username ? 'mine' : ''}">
          <b>${esc(m.username)}</b>
          <span>${esc(m.body)}</span>
        </div>`).join('')
    : `<div class="comm-empty">Seja o primeiro a falar.</div>`;

  host.innerHTML = `
    <div class="community">
      <div class="comm-feed">
        <div class="comm-stats">
          <div class="comm-stat"><b>${players}</b> jogadores nos games</div>
          <div class="comm-stat"><b>${chatting}</b> no chat</div>
        </div>
        <div class="comm-games">${perGame}</div>
        <div class="comm-posts">${feed}</div>
      </div>
      <div class="comm-side">
        <div class="comm-side-head">Chat global</div>
        <div class="comm-chat" id="commChat">${chat}</div>
        <form class="comm-compose" id="commForm">
          <input id="commInput" type="text" maxlength="280" placeholder="${player ? 'Escreva uma mensagem…' : 'Entre na conta para conversar'}" ${player ? '' : 'disabled'} value="${esc(chatDraft)}">
          <button type="submit" class="btn primary" ${player && !communitySending ? '' : 'disabled'}>Enviar</button>
        </form>
      </div>
    </div>`;

  const form = document.getElementById('commForm') as HTMLFormElement | null;
  form?.addEventListener('submit', (e) => {
    e.preventDefault();
    void sendCommunity();
  });
  const nextChat = document.getElementById('commChat');
  if (nextChat && stickBottom) nextChat.scrollTop = nextChat.scrollHeight;
  const nextInput = document.getElementById('commInput') as HTMLInputElement | null;
  if (keepFocus) nextInput?.focus();
}

async function sendCommunity() {
  if (!player) {
    openPlayerAccount();
    return;
  }
  const input = document.getElementById('commInput') as HTMLInputElement | null;
  const text = (input?.value || '').trim();
  if (!text || communitySending) return;
  communitySending = true;
  paintFooter();
  try {
    const msg = await CommunitySend(text) as unknown as CommunityMessage;
    chatDraft = '';
    if (community) community.chat = [...(community.chat || []), msg];
    if (input) input.value = '';
    paintHero();
  } catch (e) {
    const message = String(e).replace(/^Error:\s*/, '');
    if (needsLuxViewLogin(message)) openPlayerAccount();
    toast(needsLuxViewLogin(message) ? 'Entre ou crie uma conta LuxView para conversar.' : message, true);
  } finally {
    communitySending = false;
    paintFooter();
  }
}

function paintFooter() {
  const g = games[selected];
  const id = g ? cardGame(g) : '';
  const line = document.getElementById('pline');
  const bar = document.getElementById('pbarwrap');
  const fill = document.getElementById('pbar') as HTMLElement | null;
  const actions = document.getElementById('actions');
  if (!line || !bar || !fill || !actions) return;
  if (communityOpen) {
    line.textContent = player
      ? 'Chat aberto com quem estiver online no launcher.'
      : 'Entre na conta LuxView para conversar com a comunidade.';
    if (!installing) { bar.classList.remove('active', 'indet'); fill.style.width = '0%'; }
    actions.innerHTML = player
      ? `<button class="btn" id="acctFromComm">Conta: ${esc(player.username)}</button>`
      : `<button class="btn primary" id="acctFromComm">Entrar para conversar</button>`;
    document.getElementById('acctFromComm')?.addEventListener('click', openPlayerAccount);
    return;
  }
  line.textContent = footerLine(g);
  if (!installing) { bar.classList.remove('active', 'indet'); fill.style.width = '0%'; }
  actions.innerHTML = `
    ${g && g.installed && (id === 'rakion' || id === 'metin2') ? `<button class="btn icon" id="optionsBtn" title="Configurações do jogo">⚙</button>` : ''}
    <button class="btn icon" id="folderBtn" title="Abrir pasta de instalação" ${g && g.installed ? '' : 'disabled'}>📁</button>
    ${actionBtn(g)}`;
  document.getElementById('actionBtn')?.addEventListener('click', doAction);
  document.getElementById('optionsBtn')?.addEventListener('click', () => { if (g) openOptions(g); });
  document.getElementById('folderBtn')?.addEventListener('click', () => { if (g && g.app_id) OpenInstallFolder(g.app_id); });
}

function actionBtn(g?: Card): string {
  if (!g) return '';
  const id = cardGame(g);
  if (!g.enabled) return `<button class="btn" disabled>Indisponível</button>`;
  if (id === loadingGame) return `<button class="btn primary" disabled><span class="spinner"></span> Carregando ${esc(niceName(g))}…</button>`;
  if (id === runningGame) return `<button class="btn primary" disabled>● Em execução</button>`;
  if (installing) return `<button class="btn primary" disabled><span class="spinner"></span> ${g.update_available ? 'Atualizando…' : 'Instalando…'}</button>`;
  if (g.update_available) return `<button class="btn primary" id="actionBtn">⬇ ATUALIZAR</button>`;
  if (g.installed) return `<button class="btn primary" id="actionBtn">▶ JOGAR</button>`;
  return `<button class="btn primary" id="actionBtn">⬇ INSTALAR</button>`;
}

function footerLine(g?: Card): string {
  if (!g) return '';
  const id = cardGame(g);
  if (!g.enabled) return 'Este jogo ainda não está disponível.';
  if (id === loadingGame) return `Carregando ${niceName(g)}… (verificando arquivos e iniciando o jogo).`;
  if (id === runningGame) return 'Jogo em execução — Alt+Tab liberado (ou Ctrl+Alt+M para minimizar).';
  if (g.installed) return g.update_available
    ? 'Nova versão do client disponível — clique em ATUALIZAR.'
    : 'Instalado — pronto para jogar.';
  return 'Clique em INSTALAR para baixar o client.';
}

async function doAction() {
  const g = games[selected];
  if (!g || !g.enabled || installing) return;
  const id = cardGame(g);

  if (g.installed && !g.update_available) {
    g.game = id;
    return launchInstalled(g);
  }

  installing = true;
  paintFooter();
  document.getElementById('pbarwrap')!.classList.add('active');
  const updatingClient = !!g.update_available;
  g.game = id;
  try {
    await InstallGame(g as any);
    if (!(await IsInstalled(g.app_id, g.game))) {
      throw new Error('client extraído incompleto — tente instalar novamente');
    }
    g.installed = true;
    g.update_available = false;
    toast(updatingClient ? 'Client atualizado!' : 'Instalado com sucesso!');
    await refreshCatalog();
  } catch (e) {
    toast(String(e), true);
  } finally {
    installing = false;
    paintFooter();
    paintChips();
  }
}

// ---------- modais (login + opções) ----------
function showModal(inner: string, persist = false): HTMLElement {
  closeModal();
  const ov = document.createElement('div');
  ov.id = 'modal';
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal">${inner}</div>`;
  if (!persist) {
    ov.addEventListener('mousedown', (e) => { if (e.target === ov) closeModal(); });
  }
  document.body.appendChild(ov);
  requestAnimationFrame(() => ov.classList.add('show'));
  return ov;
}
function closeModal() { document.getElementById('modal')?.remove(); }

function needsLuxViewLogin(message: string): boolean {
  const m = message.toLowerCase();
  return m.includes('entre na conta luxview')
    || m.includes('unauthorized')
    || m.includes('não autorizado')
    || m.includes('nao autorizado')
    || m.includes('sessão expirada')
    || m.includes('sessao expirada');
}

async function launchInstalled(g: Card) {
  if (!player) {
    toast('Entre ou crie uma conta LuxView para jogar.', true);
    openPlayerAccount();
    return;
  }
  try {
    if (cardGame(g) === 'tibia') {
      await launchTibia(g);
      return;
    }
    if (cardGame(g) === 'muemu') {
      await launchMu(g);
      return;
    }
    await Play(g as any, '', '');
    toast('Iniciando o jogo…');
    monitorGame(g);
  } catch (e) {
    await handleLaunchError(g, e);
  }
}

type MuServer = {
  id: number; server: number; channel: number; name: string;
  difficulty?: string; mode?: string; load: number; players?: number;
};

function muServerKey(appId: string) {
  return 'luxview:mu-server:' + appId;
}

function muPopLabel(s: MuServer): string {
  if (s.players && s.players > 0) return s.players + ' online';
  if (s.load >= 100) return 'lotado';
  if (s.load >= 70) return 'cheio';
  if (s.load >= 30) return 'médio';
  if (s.load > 0) return 'online';
  return 'vazio';
}

function muServerTitle(name: string): string {
  const n = (name || '').trim();
  if (!n) return 'Servidor';
  if (/^servidor\b/i.test(n)) return n;
  return 'Servidor ' + n;
}

function applyMuCardCatalog(servers: MuServer[], groups?: MuServerGroupMeta[]): MuServer[] {
  if (!groups || groups.length === 0) return servers;
  const byGroup = new Map<number, MuServerGroupMeta>();
  const byID = new Map<number, string>();
  for (const g of groups) {
    byGroup.set(g.group, g);
    for (const ch of g.channels || []) {
      if (ch.mode) byID.set(ch.id, ch.mode);
    }
  }
  return servers.map((s) => {
    const g = byGroup.get(s.server - 1);
    const mode = byID.get(s.id);
    return {
      ...s,
      name: g?.name?.trim() || s.name,
      difficulty: g?.difficulty?.trim() || s.difficulty,
      mode: mode || s.mode,
    };
  });
}

// Always render a group header (name + difficulty) and indented channel rows.
// A single-channel season used to collapse into a flat radio labeled with the
// season name — that made Season 2 / Season 6 look nested under the 99d header.
function renderMuServerGroup(entries: MuServer[], last: number, isFirstGroup: boolean): string {
  const head = entries[0];
  const diff = (head.difficulty || '').trim();
  const diffClass = diff ? ' mu-diff-' + diff.toLowerCase() : '';
  const header = '<div class="mu-srv-head">' +
    '<span class="mu-srv-name">' + esc(muServerTitle(head.name)) + '</span>' +
    (diff ? '<span class="mu-diff' + diffClass + '">' + esc(diff) + '</span>' : '') +
    '</div>';
  const rows = entries.map((s, i) => renderMuServerRow(s, last, isFirstGroup && i === 0)).join('');
  return '<div class="mu-srv-group">' + header + '<div class="mu-srv-channels">' + rows + '</div></div>';
}

function renderMuServerRow(s: MuServer, last: number, preferChecked: boolean): string {
  const full = s.load >= 100;
  const hasLast = Number.isFinite(last);
  const checked = (hasLast && s.id === last) || (!hasLast && preferChecked);
  const mode = (s.mode || '').trim();
  const modeClass = mode ? ' mu-mode-' + mode.toLowerCase() : '';
  return '<label class="mu-srv-row' + (full ? ' is-full' : '') + '">' +
    '<input type="radio" name="muSrv" value="' + s.id + '" ' + (checked && !full ? 'checked' : '') + ' ' + (full ? 'disabled' : '') + '>' +
    '<strong>Canal ' + s.channel + '</strong>' +
    (mode ? '<span class="mu-mode' + modeClass + '">' + esc(mode) + '</span>' : '') +
    '<span class="mu-pop">' + esc(muPopLabel(s)) + '</span>' +
  '</label>';
}

function muLastServerId(appId: string): number {
  const raw = localStorage.getItem(muServerKey(appId));
  if (raw === null || raw === '') return Number.NaN;
  const n = Number(raw);
  return Number.isFinite(n) ? n : Number.NaN;
}

async function playMuNow(g: Card, serverID: number) {
  await PlayMu(g as any, serverID);
  if (serverID >= 0) localStorage.setItem(muServerKey(g.app_id), String(serverID));
  toast('Iniciando o jogo…');
  monitorGame(g);
}

// JOGAR no MU: sempre abre o seletor de servidor/canal. A última escolha
// fica pré-marcada no rádio; o usuário confirma com Jogar. Clique fora do
// modal não fecha — isso fazia o JOGAR parecer morto.
async function launchMu(g: Card) {
  let servers: MuServer[] = [];
  let fetchError = '';
  try {
    servers = (await GetMuServers(g as any)) as unknown as MuServer[];
  } catch (e) {
    fetchError = String(e).replace(/^Error:\s*/, '');
    servers = [];
  }
  servers = applyMuCardCatalog(servers, g.server_groups);
  const last = muLastServerId(g.app_id);
  const available = servers.filter((s) => s.load < 100);

  let pickerError = '';
  if (fetchError) {
    pickerError = fetchError;
  } else if (available.length === 0) {
    pickerError = servers.length > 0
      ? 'Nenhum canal disponível no momento (todos lotados).'
      : 'Nenhum servidor disponível no momento.';
  }

  const displayServers = available.length > 0 ? available : servers;
  await openMuServerPicker(g, displayServers, last, pickerError);
}

async function openMuServerPicker(g: Card, servers: MuServer[], last: number, initialError = '') {
  const ov = showModal(`
    <h3>Escolha o servidor</h3>
    <div class="modal-err" id="muSrvErr"></div>
    <div id="muSrvList" class="mu-srv-list"></div>
    <div class="modal-actions">
      <button class="btn" id="muSrvCancel">Cancelar</button>
      <button class="btn primary" id="muSrvGo">▶ Jogar</button>
    </div>
    <p class="modal-hint">O jogo abre direto na seleção de personagem.</p>
  `, true);
  document.getElementById('muSrvCancel')!.onclick = closeModal;

  if (initialError) {
    document.getElementById('muSrvErr')!.textContent = initialError;
  }

  const list = document.getElementById('muSrvList')!;
  const groupOrder: number[] = [];
  const groups = new Map<number, MuServer[]>();
  for (const s of servers) {
    if (!groups.has(s.server)) { groups.set(s.server, []); groupOrder.push(s.server); }
    groups.get(s.server)!.push(s);
  }
  list.innerHTML = groupOrder.map((server, i) => renderMuServerGroup(groups.get(server)!, last, i === 0)).join('');

  const go = document.getElementById('muSrvGo') as HTMLButtonElement;
  go.onclick = async () => {
    const sel = document.querySelector<HTMLInputElement>('input[name="muSrv"]:checked');
    const fallback = servers.find((s) => s.load < 100);
    const id = sel ? Number(sel.value) : fallback?.id;
    if (id === undefined || !Number.isFinite(id)) {
      document.getElementById('muSrvErr')!.textContent = 'Escolha um servidor.';
      return;
    }
    go.disabled = true;
    try {
      await playMuNow(g, id);
      closeModal();
    } catch (e) {
      go.disabled = false;
      const message = String(e).replace(/^Error:\s*/, '');
      if (needsLuxViewLogin(message)) {
        ov.remove();
        openPlayerAccount();
        return;
      }
      document.getElementById('muSrvErr')!.textContent = message;
    }
  };
}

async function handleLaunchError(g: Card, e: unknown) {
  const message = String(e).replace(/^Error:\s*/, '');
  if (needsLuxViewLogin(message)) {
    player = null;
    paintStatus();
    openPlayerAccount();
    toast('Entre ou crie uma conta LuxView para jogar.', true);
    return;
  }
  if (message.includes('jogo não encontrado')) {
    g.installed = await IsInstalled(g.app_id, g.game);
    paintFooter();
    paintChips();
  }
  toast(message, true);
}

function tibiaCharKey(appId: string) {
  return `luxview:tibia-char:${appId}`;
}

function tibiaNameHint(): string {
  const letters = (player?.username || '').replace(/[^A-Za-z]/g, '');
  if (letters.length < 2) return '';
  return letters[0].toUpperCase() + letters.slice(1, 29).toLowerCase();
}

async function launchTibia(g: Card) {
  const raw = localStorage.getItem(tibiaCharKey(g.app_id));
  if (raw) {
    try {
      const saved = JSON.parse(raw) as { name: string; vocation: string };
      if (saved.name && saved.vocation) {
        await playTibia(g, saved.name, saved.vocation);
        return;
      }
    } catch { /* pede de novo */ }
  }
  openTibiaCharacter(g);
}

async function playTibia(g: Card, name: string, vocation: string) {
  try {
    await Play(g as any, name, vocation);
    localStorage.setItem(tibiaCharKey(g.app_id), JSON.stringify({ name, vocation }));
    closeModal();
    toast('Iniciando o jogo…');
    monitorGame(g);
  } catch (e) {
    await handleLaunchError(g, e);
  }
}

function openTibiaCharacter(g: Card) {
  const hint = tibiaNameHint();
  showModal(`
    <h3>Criar personagem</h3>
    <div class="field"><label>Nome</label><input id="tbName" type="text" maxlength="29" value="${esc(hint)}"></div>
    <div class="field"><label>Vocação</label>
      <select id="tbVoc">
        <option value="knight">Cavaleiro</option>
        <option value="paladin">Paladino</option>
        <option value="sorcerer">Mago</option>
        <option value="druid">Druida</option>
        <option value="monk">Monge</option>
      </select>
    </div>
    <div class="modal-err" id="tbErr"></div>
    <div class="modal-actions">
      <button class="btn" id="tbCancel">Cancelar</button>
      <button class="btn primary" id="tbGo">Criar e jogar</button>
    </div>
    <p class="modal-hint">Só na primeira vez — depois o JOGAR entra direto.</p>
  `);
  const nameEl = document.getElementById('tbName') as HTMLInputElement;
  const vocEl = document.getElementById('tbVoc') as HTMLSelectElement;
  const errEl = document.getElementById('tbErr')!;
  document.getElementById('tbCancel')!.onclick = closeModal;
  document.getElementById('tbGo')!.onclick = async () => {
    const name = nameEl.value.trim();
    const vocation = vocEl.value;
    errEl.textContent = '';
    if (name.replace(/[^A-Za-z ]/g, '').replace(/\s+/g, ' ').trim().length < 2) {
      errEl.textContent = 'Nome: 2 a 29 letras.';
      return;
    }
    try {
      await Play(g as any, name, vocation);
      localStorage.setItem(tibiaCharKey(g.app_id), JSON.stringify({ name, vocation }));
      closeModal();
      toast('Iniciando o jogo…');
      monitorGame(g);
    } catch (e) {
      const message = String(e).replace(/^Error:\s*/, '');
      if (message.includes('entre na conta LuxView')) {
        closeModal();
        openPlayerAccount();
      }
      errEl.textContent = message;
    }
  };
}

const RESOLUTIONS = [[1280,720],[1366,768],[1600,900],[1920,1080],[2560,1440],[3840,2160]];

async function openOptions(g: Card) {
  if (g.game === 'metin2') {
    return openMetin2Options(g);
  }
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
        <option value="fullscreen" ${s.display_mode==='fullscreen'?'selected':''}>Tela cheia (recomendado)</option>
        <option value="borderless" ${s.display_mode==='borderless'?'selected':''}>Janela em tela cheia</option>
        <option value="windowed" ${s.display_mode==='windowed'?'selected':''}>Janela</option>
      </select>
      <small class="opt-hint"><b>Tela cheia</b>: melhor desempenho e única que funciona com overlay (Discord/NVIDIA). <b>Janela em tela cheia</b>: janela do Windows preenchendo a tela toda (resolução automática), com Alt+Tab. <b>Janela</b>: janela movível. Nos modos em janela o overlay cobre o jogo com uma camada preta (limitação do overlay com este jogo).</small>
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

  // No modo "Janela em tela cheia" a resolução é automática (preenche a tela), então
  // o seletor de resolução fica desabilitado.
  const dispEl = document.getElementById('optDisp') as HTMLSelectElement;
  const resEl = document.getElementById('optRes') as HTMLSelectElement;
  const syncRes = () => {
    const auto = dispEl.value === 'borderless';
    resEl.disabled = auto;
    resEl.title = auto ? 'Resolução automática neste modo (preenche a tela).' : '';
  };
  dispEl.onchange = syncRes;
  syncRes();

  document.getElementById('optCancel')!.onclick = closeModal;
  document.getElementById('optSave')!.onclick = async () => {
    const val = (id: string) => (document.getElementById(id) as HTMLInputElement);
    const [w, h] = val('optRes').value.split('x').map(Number);
    const out: Settings = {
      screen_width: w, screen_height: h,
      display_mode: (document.getElementById('optDisp') as HTMLSelectElement).value,
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

async function openMetin2Options(g: Card) {
  let s: Metin2Settings;
  try { s = (await GetMetin2Settings(g as any)) as unknown as Metin2Settings; }
  catch (e) { toast(String(e), true); return; }

  const resolutions = [[1280, 720], [1366, 708], [1600, 900], [1920, 1080], [2560, 1440]];
  const selectOptions = (items: Array<[number, string]>, selected: number) =>
    items.map(([value, label]) => `<option value="${value}" ${value === selected ? 'selected' : ''}>${label}</option>`).join('');
  const toggle = (id: string, label: string, checked: boolean) => `
    <label class="metin-toggle"><span>${label}</span><span class="sw"><input type="checkbox" id="${id}" ${checked ? 'checked' : ''}><span></span></span></label>`;

  showModal(`
    <h3>Configurações — ${esc(niceName(g))}</h3>
    <div class="metin-options">
      <div class="metin-heading">Vídeo</div>
      <label>Resolução</label>
      <select id="m2Resolution">
        ${resolutions.map(([w, h]) => `<option value="${w}x${h}" ${w === s.screen_width && h === s.screen_height ? 'selected' : ''}>${w} × ${h}</option>`).join('')}
        ${resolutions.some(([w, h]) => w === s.screen_width && h === s.screen_height) ? '' : `<option value="${s.screen_width}x${s.screen_height}" selected>${s.screen_width} × ${s.screen_height} (atual)</option>`}
      </select>
      <label>Frequência</label>
      <select id="m2Frequency">${selectOptions([[60, '60 Hz'], [75, '75 Hz'], [120, '120 Hz'], [144, '144 Hz'], [165, '165 Hz']], s.frequency)}</select>
      <label>Modo de exibição</label>
      <select id="m2Windowed"><option value="0" ${!s.windowed ? 'selected' : ''}>Tela cheia</option><option value="1" ${s.windowed ? 'selected' : ''}>Janela</option></select>
      <label>Distância de visão</label>
      <select id="m2Visibility">${selectOptions([[0, 'Muito baixa'], [1, 'Baixa'], [2, 'Média'], [3, 'Alta']], s.visibility)}</select>
      <label>Sombras</label>
      <select id="m2Shadow">${selectOptions([[0, 'Desligadas'], [1, 'Baixas'], [2, 'Médias'], [3, 'Altas']], s.shadow_level)}</select>
      <label>Nível de efeitos</label>
      <select id="m2Effects">${selectOptions([[0, 'Todos'], [1, 'Meus e monstros'], [2, 'Meus e outros'], [3, 'Somente meus'], [4, 'Nenhum']], s.effect_level)}</select>
      <label>Lojas privadas</label>
      <select id="m2Shops">${selectOptions([[0, 'Todas'], [1, 'Próximas'], [2, 'Médio alcance'], [3, 'Mais próximas'], [4, 'Ao redor']], s.private_shop_level)}</select>
      <label>Itens no chão</label>
      <select id="m2Drops">${selectOptions([[0, 'Efeito e nome'], [1, 'Ocultar efeito'], [2, 'Ocultar nome'], [3, 'Somente ao passar o mouse'], [4, 'Não mostrar']], s.drop_item_level)}</select>
      ${toggle('m2SoftwareCursor', 'Cursor de software', s.software_cursor)}
      ${toggle('m2ObjectCulling', 'Otimizar objetos distantes', s.object_culling)}
      ${toggle('m2Decompress', 'Descomprimir texturas', s.decompressed_texture)}

      <div class="metin-heading">Áudio e interface</div>
      <label>Volume da música <b id="m2MusicLabel">${Math.round(s.music_volume * 100)}%</b></label>
      <input type="range" id="m2Music" min="0" max="1" step="0.01" value="${s.music_volume}">
      <label>Volume dos efeitos <b id="m2SoundLabel">${s.sound_volume}</b></label>
      <input type="range" id="m2Sound" min="0" max="5" step="1" value="${s.sound_volume}">
      <label>Gamma <b id="m2GammaLabel">${s.gamma}</b></label>
      <input type="range" id="m2Gamma" min="0" max="3" step="1" value="${s.gamma}">
      ${toggle('m2AlwaysName', 'Mostrar nomes sempre', s.always_view_name)}
      ${toggle('m2MobLevel', 'Mostrar nível dos monstros', s.show_mob_level)}
      ${toggle('m2MobAI', 'Mostrar indicador dos monstros', s.show_mob_ai_flag)}
      ${toggle('m2NPCName', 'Mostrar nomes dos NPCs', s.npc_name_status)}
      ${toggle('m2Pets', 'Mostrar pets', s.pet_status)}
      ${toggle('m2AutoPickup', 'Coleta automática', s.auto_pickup)}

      <div class="metin-heading">Ambiente e avançado</div>
      ${toggle('m2Fog', 'Névoa', s.fog_mode)}
      ${toggle('m2Night', 'Modo noturno', s.night_mode)}
      ${toggle('m2Snow', 'Neve', s.snow_mode)}
      ${toggle('m2SnowTexture', 'Textura de neve', s.snow_texture)}
      ${toggle('m2Refine', 'Confirmar refinamento', s.show_refine_dialog)}
      ${toggle('m2FOV', 'Campo de visão estendido', s.extended_fov)}
      ${toggle('m2IME', 'IME padrão do Windows', s.use_default_ime)}
      <label>Modo de renderização</label>
      <select id="m2Tiling">${selectOptions([[0, 'Automático'], [1, 'CPU'], [2, 'GPU']], s.software_tiling)}</select>
      <label>Tempo de pré-carregamento <b id="m2PreloadLabel">${s.pre_loading_delay}s</b></label>
      <input type="range" id="m2Preload" min="0" max="60" step="1" value="${s.pre_loading_delay}">
    </div>
    <div class="modal-err" id="m2Err"></div>
    <div class="modal-actions"><button class="btn" id="m2Cancel">Cancelar</button><button class="btn primary" id="m2Save">Salvar</button></div>
    <div class="modal-hint">As alterações entram em vigor na próxima abertura do jogo.</div>
  `);

  const input = (id: string) => document.getElementById(id) as HTMLInputElement;
  const select = (id: string) => document.getElementById(id) as HTMLSelectElement;
  const bool = (id: string) => input(id).checked;
  input('m2Music').oninput = () => { document.getElementById('m2MusicLabel')!.textContent = `${Math.round(parseFloat(input('m2Music').value) * 100)}%`; };
  input('m2Sound').oninput = () => { document.getElementById('m2SoundLabel')!.textContent = input('m2Sound').value; };
  input('m2Gamma').oninput = () => { document.getElementById('m2GammaLabel')!.textContent = input('m2Gamma').value; };
  input('m2Preload').oninput = () => { document.getElementById('m2PreloadLabel')!.textContent = `${input('m2Preload').value}s`; };
  document.getElementById('m2Cancel')!.onclick = closeModal;
  document.getElementById('m2Save')!.onclick = async () => {
    const [width, height] = select('m2Resolution').value.split('x').map(Number);
    const out: Metin2Settings = {
      ...s, screen_width: width, screen_height: height, frequency: Number(select('m2Frequency').value),
      windowed: select('m2Windowed').value === '1', visibility: Number(select('m2Visibility').value),
      shadow_level: Number(select('m2Shadow').value), effect_level: Number(select('m2Effects').value),
      private_shop_level: Number(select('m2Shops').value), drop_item_level: Number(select('m2Drops').value),
      software_cursor: bool('m2SoftwareCursor'), object_culling: bool('m2ObjectCulling'), decompressed_texture: bool('m2Decompress'),
      music_volume: Number(input('m2Music').value), sound_volume: Number(input('m2Sound').value), gamma: Number(input('m2Gamma').value),
      always_view_name: bool('m2AlwaysName'), show_mob_level: bool('m2MobLevel'), show_mob_ai_flag: bool('m2MobAI'),
      npc_name_status: bool('m2NPCName'), pet_status: bool('m2Pets'), auto_pickup: bool('m2AutoPickup'), fog_mode: bool('m2Fog'),
      night_mode: bool('m2Night'), snow_mode: bool('m2Snow'), snow_texture: bool('m2SnowTexture'), show_refine_dialog: bool('m2Refine'),
      extended_fov: bool('m2FOV'), use_default_ime: bool('m2IME'), software_tiling: Number(select('m2Tiling').value),
      pre_loading_delay: Number(input('m2Preload').value),
    };
    try { await SaveMetin2Settings(g as any, out as any); closeModal(); toast('Configurações do Metin2 salvas!'); }
    catch (e) { document.getElementById('m2Err')!.textContent = String(e).replace(/^Error:\s*/, ''); }
  };
}

try {
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
} catch { /* runtime ainda não injetado */ }

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

void load().catch((e) => {
  if (app) app.innerHTML = `<div class="shell"><div class="hero hero-empty">${esc(String(e))}</div></div>`;
});
