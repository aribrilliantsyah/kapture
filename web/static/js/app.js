// App shell: router, grouped sidebar, command palette (Ctrl+K), topbar.
import { api, redirectToLogin } from './api.js';
import { h, icon, menu, emptyState, setTimeZone, tzLabel, toast, TYPE_TAGS } from './ui.js';
import {
  store, on, save, load, refreshCatalog, refreshAgents, getRoute, setRoute, patchRoute, href, setUser,
  namespaces, workloadsOf, findWorkload, setScope, togglePin, TYPE_GROUPS, groupOf, clearRecap,
} from './state.js';
import { openPalette } from './palette.js';
import * as dashboard from './views/dashboard.js';
import * as logs from './views/logs.js';
import * as compare from './views/compare.js';
import * as workloads from './views/workloads.js';
import * as workload from './views/workload.js';
import * as nsView from './views/namespaces.js';
import * as agents from './views/agents.js';
import * as storage from './views/storage.js';
import * as profile from './views/profile.js';
import * as users from './views/users.js';
import * as about from './views/about.js';

const VIEWS = { dashboard, logs, compare, workloads, workload, namespaces: nsView, agents, storage, profile, users, about };
const TITLES = {
  dashboard: 'Dashboard', logs: 'Explorer', compare: 'Compare', workloads: 'Workloads', workload: 'Workload',
  namespaces: 'Namespaces', agents: 'Agents', storage: 'Storage', profile: 'Profile', users: 'Users', about: 'About',
};
const $ = (id) => document.getElementById(id);
const rootEl = document.documentElement;
const viewRoot = $('view');
const collapsed = new Set(load('kapture_groups', []));
const isAdmin = () => store.authEnabled && store.role === 'admin';
let current = null;

// ── Routing ──
function renderRoute() {
  const r = getRoute();
  if (!VIEWS[r.view] || (r.view === 'users' && !isAdmin())) return setRoute('dashboard', {}, { replace: true });
  if (current?.view !== r.view) {
    current?.inst.destroy?.();
    viewRoot.replaceChildren();
    viewRoot.scrollTop = 0;
    current = { view: r.view, inst: VIEWS[r.view].mount(viewRoot) };
  }
  current.inst.update?.(r.params);
  const now = getRoute();
  document.title = `${TITLES[now.view]} - Kapture`;
  rootEl.classList.remove('sidebar-open');
  renderCrumbs(now);
  renderSidebar(now);
}

function renderCrumbs(r) {
  const parts = [];
  const link = (text, to) => h('a', { href: to }, text);
  if (r.view === 'workload') {
    const w = findWorkload(r.params.ns, r.params.wl);
    const g = groupOf(w?.type);
    parts.push(link(g.label, href('workloads', { type: g.key })), icon('chevron-right'), h('span', null, r.params.ns), icon('chevron-right'), h('b', null, r.params.wl));
  } else if (r.view === 'workloads') {
    const g = TYPE_GROUPS.find((x) => x.key === r.params.type);
    parts.push(h('span', null, 'Workloads'), icon('chevron-right'), h('b', null, g ? g.label : 'All'));
  } else {
    parts.push(h('b', null, r.view === 'logs' && r.params.live ? 'Live tail' : TITLES[r.view]));
    if (r.view === 'logs' || r.view === 'compare') {
      for (const v of [r.params.ns, r.params.wl, r.params.pod]) if (v) parts.push(icon('chevron-right'), h('span', null, v));
    }
  }
  $('crumbs').replaceChildren(...parts);
  renderScopeChip();
}

// The namespace scope (set from the Namespaces page or Ctrl+K) shows as a
// removable chip in the topbar instead of a sidebar control.
function renderScopeChip() {
  const el = $('scope-chip');
  el.hidden = !store.scope;
  if (!store.scope) return;
  el.title = `Only namespace ${store.scope} is shown. Click to show all namespaces.`;
  el.replaceChildren(icon('folder'), h('span', null, store.scope), icon('x'));
}
$('scope-chip').addEventListener('click', () => setScope(''));

// ── Sidebar ──
function sideLink(to, label, ic, active, extra = []) {
  return h('a', { class: `side-link${active ? ' active' : ''}`, href: to, title: label }, icon(ic), h('span', { class: 'name' }, label), ...extra);
}

function sideGroup(id, title, links) {
  const el = h('section', { class: `side-group${collapsed.has(id) ? ' collapsed' : ''}` });
  if (title) {
    el.append(h('button', {
      class: 'side-group-title', type: 'button',
      onclick: () => {
        if (collapsed.has(id)) collapsed.delete(id);
        else collapsed.add(id);
        save('kapture_groups', [...collapsed]);
        el.classList.toggle('collapsed');
      },
    }, title, icon('chevron-down')));
  }
  el.append(h('div', { class: 'side-links' }, links));
  return el;
}

function renderSidebar(r = getRoute()) {
  const scope = store.scope ? { ns: store.scope } : {};
  const wls = workloadsOf(store.scope);
  const counts = Object.fromEntries(TYPE_GROUPS.map((g) => [g.key, 0]));
  wls.forEach((w) => counts[groupOf(w.type).key]++);
  const openType = r.view === 'workloads' ? r.params.type : r.view === 'workload' ? groupOf(findWorkload(r.params.ns, r.params.wl)?.type).key : '';
  const ok = store.agents.filter((a) => a.status === 'ok').length;
  const live = r.view === 'logs' && !!r.params.live;

  const pins = store.pins.map((k) => {
    const [ns, wl] = k.split('/');
    const w = findWorkload(ns, wl);
    return sideLink(href('workload', { ns, wl }), wl, groupOf(w?.type).icon,
      r.view === 'workload' && r.params.ns === ns && r.params.wl === wl,
      [h('button', {
        class: 'unpin', type: 'button', title: `Unpin ${ns}/${wl}`,
        onclick: (e) => { e.preventDefault(); togglePin(ns, wl); },
      }, icon('x'))]);
  });

  $('side-nav').replaceChildren(...[
    sideGroup('main', '', [sideLink('#/dashboard', 'Dashboard', 'overview', r.view === 'dashboard')]),
    pins.length ? sideGroup('pinned', 'Pinned', pins) : null,
    sideGroup('logs', 'Logs', [
      sideLink(href('logs', scope), 'Explorer', 'logs', r.view === 'logs' && !live),
      sideLink(href('logs', { ...scope, live: '1', range: '15m' }), 'Live tail', 'activity', live),
      sideLink(href('compare', scope), 'Compare', 'compare', r.view === 'compare'),
    ]),
    sideGroup('workloads', 'Workloads', TYPE_GROUPS.map((g) => sideLink(href('workloads', { type: g.key }), g.label, g.icon,
      openType === g.key, [h('span', { class: 'count' }, counts[g.key] || '')]))),
    sideGroup('cluster', 'Cluster', [
      sideLink('#/namespaces', 'Namespaces', 'folder', r.view === 'namespaces', [h('span', { class: 'count' }, namespaces().length || '')]),
      sideLink('#/agents', 'Agents', 'server', r.view === 'agents', [
        h('span', { class: 'count' }, store.agents.length ? `${ok}/${store.agents.length}` : '0'),
        h('i', { class: `state${store.agents.length && ok === store.agents.length ? '' : ' bad'}`, title: 'Agent health' })]),
      sideLink('#/storage', 'Storage', 'storage', r.view === 'storage'),
    ]),
    store.authEnabled ? sideGroup('account', 'Account', [
      sideLink('#/profile', 'Profile', 'user', r.view === 'profile'),
      isAdmin() ? sideLink('#/users', 'Users', 'users', r.view === 'users') : null,
    ].filter(Boolean)) : null,
    sideGroup('about', '', [sideLink('#/about', 'About', 'info', r.view === 'about')]),
  ].filter(Boolean));
}

on('scope', () => {
  const r = getRoute();
  if (r.view === 'logs' || r.view === 'compare') patchRoute({ ns: store.scope, wl: '', pod: '', c: '', a: '', b: '' });
  renderSidebar();
  renderScopeChip();
});

// ── Command palette: pages, workloads, namespaces, actions ──
function paletteItems(q) {
  const scope = store.scope ? { ns: store.scope } : {};
  const go = (to) => () => { location.hash = to; };
  const page = (label, ic, to, keywords = '') => ({ group: 'Pages', label, icon: ic, keywords, run: go(to) });
  const items = [];
  if (q) items.push({ group: 'Logs', label: `Search logs for "${q}"`, icon: 'search', always: true, run: () => setRoute('logs', { ...scope, q }) });
  items.push(
    page('Dashboard', 'overview', '#/dashboard', 'home overview'),
    page('Log explorer', 'logs', href('logs', scope), 'logs search'),
    page('Live tail', 'activity', href('logs', { ...scope, live: '1', range: '15m' }), 'stream follow realtime'),
    page('Compare pods', 'compare', href('compare', scope), 'replicas side by side'),
    ...TYPE_GROUPS.map((g) => page(`Workloads › ${g.label}`, g.icon, href('workloads', { type: g.key }), 'workloads')),
    page('Namespaces', 'folder', '#/namespaces', 'scope'),
    page('Agents', 'server', '#/agents', 'nodes health'),
    page('Storage', 'storage', '#/storage', 'disk backup restore delete reset'),
  );
  if (store.authEnabled) items.push(page('Profile', 'user', '#/profile', 'account password 2fa qr recovery'));
  if (isAdmin()) items.push(page('Users', 'users', '#/users', 'accounts roles operators admin'));
  items.push(page('About', 'info', '#/about', 'author credits stack license version help'));
  if (q) {
    for (const w of workloadsOf('')) {
      const tag = TYPE_TAGS[w.type] || w.type;
      items.push(
        { group: 'Workloads', label: w.name, hint: `${w.ns} · ${tag}`, icon: groupOf(w.type).icon, keywords: w.ns, run: () => setRoute('workload', { ns: w.ns, wl: w.name }) },
        { group: 'Workloads', label: `${w.name} › Logs`, hint: w.ns, icon: 'logs', keywords: `${w.ns} logs`, run: () => setRoute('workload', { ns: w.ns, wl: w.name, tab: 'logs' }) },
      );
    }
  }
  if (store.scope) items.push({ group: 'Namespace scope', label: 'Show all namespaces', icon: 'folder', keywords: 'scope clear', run: () => setScope('') });
  for (const n of namespaces()) {
    if (n.name !== store.scope) items.push({ group: 'Namespace scope', label: `Only namespace ${n.name}`, hint: `${n.workloads} workloads`, icon: 'folder', keywords: `scope ${n.name}`, run: () => setScope(n.name) });
  }
  items.push(
    { group: 'Actions', label: 'Toggle dark mode', icon: rootEl.dataset.theme === 'dark' ? 'sun' : 'moon', keywords: 'theme light', run: toggleTheme },
    { group: 'Actions', label: 'Refresh data', icon: 'refresh', keywords: 'reload', run: refreshAll },
  );
  if (store.authEnabled) items.push({ group: 'Actions', label: 'Sign out', icon: 'logout', keywords: 'logout', run: signOut });
  return items;
}

const showPalette = () => openPalette(paletteItems);
$('palette').addEventListener('click', showPalette);
$('search-kbd').textContent = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘K' : 'Ctrl K';

// Ctrl+K opens the menu; "/" jumps to the log search of the current page.
document.addEventListener('keydown', (e) => {
  const typing = /INPUT|TEXTAREA|SELECT/.test(document.activeElement?.tagName || '');
  if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    showPalette();
  } else if (e.key === '/' && !typing) {
    e.preventDefault();
    const box = document.querySelector('[data-log-search]');
    if (box) { box.focus(); box.select(); } else showPalette();
  }
});

// ── Topbar ──
function toggleTheme() {
  const t = rootEl.dataset.theme === 'dark' ? 'light' : 'dark';
  rootEl.dataset.theme = t;
  try { localStorage.setItem('kapture_theme', t); } catch { /* private mode */ }
}

function refreshAll() {
  clearRecap();
  refreshCatalog();
  refreshAgents();
  current?.inst.refresh?.();
}

$('toggle-sidebar').addEventListener('click', () => {
  if (matchMedia('(max-width: 860px)').matches) {
    rootEl.classList.toggle('sidebar-open');
    return;
  }
  save('kapture_sidebar', rootEl.classList.toggle('sidebar-collapsed') ? 'collapsed' : '');
});
document.querySelector('.main').addEventListener('click', (e) => {
  if (!e.target.closest('#toggle-sidebar')) rootEl.classList.remove('sidebar-open');
});
$('theme').addEventListener('click', toggleTheme);
$('refresh').addEventListener('click', refreshAll);

async function signOut() {
  try { await api.post('/auth/logout'); } catch { /* already gone */ }
  location.href = '/login';
}

function renderAvatar() {
  $('avatar').textContent = (store.displayName || store.user || '?').charAt(0).toUpperCase();
  $('avatar').title = store.authEnabled ? `${store.displayName} (${store.role})` : 'Account';
}

$('avatar').addEventListener('click', () => menu($('avatar'), store.authEnabled
  ? [
    { label: `${store.displayName} · ${store.role === 'admin' ? 'Administrator' : 'Operator'}` },
    { icon: 'user', text: 'Profile', onClick: () => setRoute('profile') },
    ...(isAdmin() ? [{ icon: 'users', text: 'Users', onClick: () => setRoute('users') }] : []),
    { icon: 'info', text: 'About', onClick: () => setRoute('about') },
    { icon: 'logout', text: 'Sign out', onClick: signOut },
  ]
  : [{ label: 'Authentication is disabled' }, { icon: 'info', text: 'About', onClick: () => setRoute('about') }]));

// ── Start ──
async function start() {
  let st;
  try {
    st = await api.get('/auth/status');
  } catch (e) {
    viewRoot.replaceChildren(emptyState('alert', 'Cannot reach Kapture', e.message,
      h('button', { class: 'btn', onclick: () => location.reload() }, 'Retry')));
    return;
  }
  if (st.auth_enabled && !st.authenticated) return redirectToLogin();
  store.authEnabled = st.auth_enabled;
  setUser(st.user || { id: '', username: st.username, display_name: st.username, role: 'admin' });
  renderAvatar();

  // Dates and clock times follow the cluster zone, so set it before any view renders.
  const cfg = await api.get('/config').catch(() => null);
  if (cfg) {
    store.version = cfg.version;
    store.commit = cfg.commit;
    store.timezone = cfg.timezone;
    setTimeZone(cfg.timezone);
    $('brand-ver').textContent = [...new Set([cfg.version, cfg.commit].filter(Boolean)), tzLabel()].join(' · ');
    $('brand-ver').title = `Times are shown in ${cfg.timezone}`;
  }

  await Promise.all([refreshCatalog(), refreshAgents()]);
  on('catalog', () => { renderSidebar(); renderCrumbs(getRoute()); });
  on('agents', () => renderSidebar());
  on('pins', () => renderSidebar());
  on('user', () => { renderAvatar(); renderSidebar(); });
  on('route', renderRoute);
  window.addEventListener('hashchange', renderRoute);
  renderRoute();
  setInterval(() => {
    if (!document.hidden) {
      refreshCatalog();
      refreshAgents();
    }
  }, 30000);

  // An administrator without a recovery question can only be rescued by another admin.
  if (isAdmin()) {
    api.get('/profile').then((p) => {
      if (!p.user.has_recovery) toast('Set a recovery question in your profile, so a lost password or phone can be recovered', 'error');
    }).catch(() => {});
  }
}

start();
