// Shared app state: catalog, agents, namespace scope, pins, recap cache, router.
import { api } from './api.js';
import { localDate, zonedMs, addDays } from './ui.js';

export const LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL'];
export const ACTIVE_MS = 10 * 60e3; // a pod that logged in the last 10 min counts as running

export const TYPE_GROUPS = [
  { key: 'deployment', label: 'Deployments', icon: 'rocket', types: ['deployment', 'replicaset'] },
  { key: 'statefulset', label: 'StatefulSets', icon: 'layers', types: ['statefulset'] },
  { key: 'daemonset', label: 'DaemonSets', icon: 'grid', types: ['daemonset'] },
  { key: 'cronjob', label: 'CronJobs', icon: 'clock', types: ['cronjob'] },
  { key: 'job', label: 'Jobs', icon: 'play', types: ['job'] },
  { key: 'other', label: 'Pods & others', icon: 'box', types: [] },
];
export const groupOf = (type) => TYPE_GROUPS.find((g) => g.types.includes(type)) || TYPE_GROUPS[TYPE_GROUPS.length - 1];

const read = (k, d) => { try { return JSON.parse(localStorage.getItem(k)) ?? d; } catch { return d; } };
export const save = (k, v) => { try { localStorage.setItem(k, JSON.stringify(v)); } catch { /* private mode */ } };
export const load = read;

export const store = {
  catalog: [],
  catalogErrors: [],
  agents: [],
  version: '',
  commit: '',
  userId: '',
  user: '', // username
  displayName: '',
  role: '', // admin | operator
  timezone: '',
  authEnabled: true,
  scope: read('kapture_scope', ''),
  pins: read('kapture_pins', []),
};

// setUser stores the signed-in identity ({id, username, display_name, role}).
export function setUser(u) {
  store.userId = u.id || '';
  store.user = u.username || '';
  store.displayName = u.display_name || u.username || '';
  store.role = u.role || '';
  emit('user');
}

const listeners = {};
export function on(evt, fn) {
  (listeners[evt] ||= new Set()).add(fn);
  return () => listeners[evt].delete(fn);
}
export function emit(evt, data) {
  listeners[evt]?.forEach((fn) => fn(data));
}

export async function refreshCatalog() {
  try {
    const r = await api.get('/catalog');
    store.catalog = r.items || [];
    store.catalogErrors = r.errors || [];
    emit('catalog');
  } catch (e) {
    if (e.status !== 401) console.warn('catalog:', e.message);
  }
}

export async function refreshAgents() {
  try {
    store.agents = (await api.get('/nodes')) || [];
    emit('agents');
  } catch (e) {
    if (e.status !== 401) console.warn('agents:', e.message);
  }
}

export function setScope(ns) {
  store.scope = ns;
  save('kapture_scope', ns);
  clearRecap();
  emit('scope');
}

export const isPinned = (ns, wl) => store.pins.includes(`${ns}/${wl}`);
export function togglePin(ns, wl) {
  const k = `${ns}/${wl}`;
  store.pins = isPinned(ns, wl) ? store.pins.filter((x) => x !== k) : [...store.pins, k];
  save('kapture_pins', store.pins);
  emit('pins');
}

// ── Daily rollups (days in the cluster time zone, like the agents' index) ──
export const dayOf = (ms = Date.now()) => localDate(ms);
const recapCache = new Map();
export async function recap(days, ns = '') {
  const from = addDays(dayOf(), -(days - 1));
  const key = `${from}|${ns}`;
  const hit = recapCache.get(key);
  if (hit && Date.now() - hit.at < 30e3) return hit.data;
  const data = await api.get('/stats/recap', { from, namespace: ns });
  recapCache.set(key, { at: Date.now(), data });
  return data;
}
export function clearRecap() {
  recapCache.clear();
}

export const linesOf = (lv = {}) => Object.values(lv).reduce((a, b) => a + b, 0);
export const errorsOf = (lv = {}) => (lv.ERROR || 0) + (lv.FATAL || 0);

// Sums rollups per workload (optionally only from a date on).
export function rollupDays(days, since = '') {
  const m = new Map();
  for (const d of days || []) {
    if (since && d.date < since) continue;
    const k = `${d.namespace}/${d.workload}`;
    let r = m.get(k);
    if (!r) m.set(k, (r = { ns: d.namespace, name: d.workload, type: d.workload_type, lines: 0, errors: 0, bytes: 0, pods: new Set(), last: 0 }));
    r.lines += linesOf(d.levels);
    r.errors += errorsOf(d.levels);
    r.bytes += d.bytes;
    (d.pods || []).forEach((p) => r.pods.add(p));
    r.last = Math.max(r.last, d.last / 1e6);
  }
  return m;
}

// Per-day totals for the n days ending today (oldest first), from rollups
// matching filter: lines, errors, level counts, pods and workloads seen.
export function perDay(days, n, filter = () => true) {
  const today = dayOf();
  const out = Array.from({ length: n }, (_, i) => {
    const date = addDays(today, i - (n - 1));
    return { date, ms: zonedMs(date), lines: 0, errors: 0, levels: {}, pods: new Set(), workloads: new Set() };
  });
  const byDate = new Map(out.map((d) => [d.date, d]));
  for (const d of days || []) {
    const t = byDate.get(d.date);
    if (!t || !filter(d)) continue;
    t.lines += linesOf(d.levels);
    t.errors += errorsOf(d.levels);
    for (const [l, c] of Object.entries(d.levels || {})) t.levels[l] = (t.levels[l] || 0) + c;
    (d.pods || []).forEach((p) => t.pods.add(`${d.namespace}/${p}`));
    t.workloads.add(`${d.namespace}/${d.workload}`);
  }
  return out;
}

// Sums level-count maps.
export function sumCounts(list) {
  const out = {};
  for (const lv of list) for (const [l, c] of Object.entries(lv || {})) out[l] = (out[l] || 0) + c;
  return out;
}

// Turns rollups into the histogram shape used by volumeChart: one bucket per day.
export function dailySeries(days, n, filter = () => true) {
  const today = dayOf();
  const dates = Array.from({ length: n }, (_, i) => addDays(today, i - (n - 1)));
  const byDate = new Map(dates.map((d) => [d, {}]));
  const totals = {};
  for (const d of days || []) {
    const counts = byDate.get(d.date);
    if (!counts || !filter(d)) continue;
    for (const [l, c] of Object.entries(d.levels || {})) {
      counts[l] = (counts[l] || 0) + c;
      totals[l] = (totals[l] || 0) + c;
    }
  }
  const start = (d) => zonedMs(d) * 1e6;
  return {
    buckets: dates.map((d) => ({ start: start(d), counts: byDate.get(d) })),
    totals,
    from: start(dates[0]),
    to: start(addDays(dates[n - 1], 1)),
    bucket_nanos: 86400e9,
  };
}

// ── Router: #/<view>?<params> ──
export function getRoute() {
  const raw = location.hash.replace(/^#\/?/, '');
  const [view, query = ''] = raw.split('?');
  return { view: view || 'dashboard', params: Object.fromEntries(new URLSearchParams(query)) };
}

export function href(view, params = {}) {
  const q = new URLSearchParams(Object.entries(params).filter(([, v]) => v !== '' && v != null)).toString();
  return `#/${view}${q ? '?' + q : ''}`;
}

export function setRoute(view, params = {}, { replace = false } = {}) {
  const hash = href(view, params);
  if (hash === location.hash) return;
  if (replace) {
    history.replaceState(null, '', hash);
    emit('route');
  } else {
    location.hash = hash;
  }
}

export function patchRoute(changes, opts) {
  const r = getRoute();
  setRoute(r.view, { ...r.params, ...changes }, opts);
}

// ── Catalog views ──
const byName = (a, b) => a.name.localeCompare(b.name);
const uniq = (xs) => [...new Set(xs)].sort();

export function namespaces() {
  const m = new Map();
  for (const it of store.catalog) {
    let n = m.get(it.namespace);
    if (!n) m.set(it.namespace, (n = { name: it.namespace, pods: new Set(), workloads: new Set(), last: 0 }));
    n.pods.add(it.pod);
    n.workloads.add(it.workload);
    n.last = Math.max(n.last, it.last_seen / 1e6);
  }
  return [...m.values()].sort(byName).map((n) => ({ name: n.name, pods: n.pods.size, workloads: n.workloads.size, last: n.last }));
}

// Workloads with every pod ever seen, newest pods first.
export function workloadsOf(ns) {
  const m = new Map();
  for (const it of store.catalog) {
    if (ns && it.namespace !== ns) continue;
    const key = it.namespace + '/' + it.workload;
    let w = m.get(key);
    if (!w) m.set(key, (w = { ns: it.namespace, name: it.workload, type: it.workload_type, pods: new Map(), containers: new Set(), first: Infinity, last: 0 }));
    let p = w.pods.get(it.pod);
    if (!p) w.pods.set(it.pod, (p = { name: it.pod, node: it.node, containers: new Set(), first: Infinity, last: 0 }));
    p.containers.add(it.container);
    w.containers.add(it.container);
    const first = it.first_seen / 1e6;
    const last = it.last_seen / 1e6;
    p.first = Math.min(p.first, first);
    p.last = Math.max(p.last, last);
    w.first = Math.min(w.first, first);
    w.last = Math.max(w.last, last);
    if (it.workload_type) w.type = it.workload_type;
  }
  const now = Date.now();
  return [...m.values()].sort(byName).map((w) => {
    const pods = [...w.pods.values()]
      .map((p) => ({ ...p, containers: [...p.containers].sort(), active: now - p.last < ACTIVE_MS }))
      .sort((a, b) => b.last - a.last);
    return { ...w, pods, containers: [...w.containers].sort(), active: pods.filter((p) => p.active).length };
  });
}

export const findWorkload = (ns, wl) => workloadsOf(ns).find((w) => w.name === wl);

const match = (it, ns, wl, pod) => (!ns || it.namespace === ns) && (!wl || it.workload === wl) && (!pod || it.pod === pod);
export const workloadNames = (ns) => uniq(store.catalog.filter((it) => match(it, ns)).map((it) => it.workload));
export const podsOf = (ns, wl) => uniq(store.catalog.filter((it) => match(it, ns, wl)).map((it) => it.pod));
export const containersOf = (ns, wl, pod) => uniq(store.catalog.filter((it) => match(it, ns, wl, pod)).map((it) => it.container));
