// Two replicas side by side, scroll-synced by timestamp.
import { api } from '../api.js';
import { h, icon, fmtTime, fmtNum, msOf, emptyState, skeletonRows, debounce } from '../ui.js';
import { store, on, patchRoute, workloadsOf } from '../state.js';
import { queryParams, rangeControls, searchTerms, highlight } from '../filters.js';
import { renderAnsi } from '../ansi.js';

const LIMIT = 2000;
const keyOf = (x) => JSON.stringify([x.ns, x.wl, x.a, x.b, x.q, x.range, x.date, x.from, x.to]);

export function mount(root) {
  let p = {};
  let eff = { wls: [], pods: [] };
  let reqId = 0;
  let loadedKey = null;
  const skip = [false, false]; // ignore the scroll event we caused ourselves
  const patch = (c) => patchRoute(c);

  const range = rangeControls(patch);
  const wlSel = h('select', {
    class: 'select', 'aria-label': 'Workload',
    onchange: (e) => {
      const [ns, wl] = e.target.value.split('/');
      patch({ ns, wl, a: '', b: '' });
    },
  });
  const aSel = h('select', { class: 'select', 'aria-label': 'Left pod', onchange: (e) => patch({ a: e.target.value }) });
  const bSel = h('select', { class: 'select', 'aria-label': 'Right pod', onchange: (e) => patch({ b: e.target.value }) });
  const syncBtn = h('button', {
    class: 'chip', type: 'button', title: 'Keep both panes at the same point in time',
    onclick: () => patch({ sync: p.sync === 'off' ? '' : 'off' }),
  }, icon('link'), 'Sync scroll');
  const panes = [makePane(0), makePane(1)];
  const searchIn = h('input', {
    type: 'search', autocomplete: 'off', spellcheck: 'false', 'aria-label': 'Search logs', 'data-log-search': '',
    placeholder: 'Search both pods  (press /)',
  });
  const pushQuery = debounce(() => patch({ q: searchIn.value.trim() }), 400);
  searchIn.addEventListener('input', pushQuery);
  searchIn.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') patch({ q: searchIn.value.trim() });
    else if (e.key === 'Escape') searchIn.blur();
  });

  root.append(h('div', { class: 'page-fill' },
    h('div', { class: 'toolbar' },
      h('label', { class: 'tool-search' }, icon('search'), searchIn), range.el, h('span', { class: 'sep' }), wlSel, aSel, h('span', { class: 'muted' }, 'vs'), bSel,
      h('span', { class: 'spacer' }), syncBtn),
    h('div', { class: 'cmp-grid' }, panes[0].el, panes[1].el)));

  function makePane(i) {
    const title = h('span', { class: 'cmp-title' });
    const count = h('span', { class: 'muted' });
    const body = h('div', { class: 'cmp-body' });
    body.addEventListener('scroll', () => follow(i), { passive: true });
    return { el: h('div', { class: 'cmp-pane' }, h('div', { class: 'cmp-head' }, title, count), body), title, count, body, rows: [] };
  }

  // Resolves defaults: a workload with several replicas and its first two pods.
  function effective() {
    const wls = workloadsOf('');
    const ns = p.ns || store.scope;
    const inNs = wls.filter((w) => !ns || w.ns === ns);
    const w = inNs.find((x) => x.name === p.wl) || inNs.find((x) => x.pods.length > 1) || inNs[0] || wls[0];
    const pods = w ? w.pods.map((x) => x.name) : [];
    return { wls, ns: w?.ns || '', wl: w?.name || '', pods, a: p.a || pods[0] || '', b: p.b || pods[1] || pods[0] || '' };
  }

  function fill(sel, options, value) {
    sel.replaceChildren(...options.map(([v, label]) => h('option', { value: v }, label)));
    sel.value = value;
  }

  function syncControls() {
    range.sync(p);
    if (document.activeElement !== searchIn) searchIn.value = p.q || '';
    eff = effective();
    fill(wlSel, eff.wls.map((w) => [`${w.ns}/${w.name}`, `${w.ns} / ${w.name} (${w.pods.length})`]), `${eff.ns}/${eff.wl}`);
    fill(aSel, eff.pods.map((x) => [x, x]), eff.a);
    fill(bSel, eff.pods.map((x) => [x, x]), eff.b);
    syncBtn.classList.toggle('active', p.sync !== 'off');
  }

  async function load() {
    const id = ++reqId;
    loadedKey = keyOf({ ...p, ns: eff.ns, wl: eff.wl, a: eff.a, b: eff.b });
    const terms = searchTerms(p.q);
    const pods = [eff.a, eff.b];
    panes.forEach((pn, i) => {
      pn.title.textContent = pods[i] || 'No pod';
      pn.count.textContent = '';
      pn.rows = [];
      pn.body.replaceChildren(...skeletonRows(10));
    });
    if (!eff.wl) {
      panes.forEach((pn) => pn.body.replaceChildren(emptyState('compare', 'Nothing to compare', 'No workloads have stored logs yet.')));
      return;
    }
    const base = { ...queryParams({ ...p, ns: eff.ns, wl: '', pod: '', c: '', lv: '' }), sort: 'asc', limit: LIMIT };
    const res = await Promise.allSettled(pods.map((pod) => api.get('/logs', { ...base, pod })));
    if (id !== reqId) return;
    res.forEach((r, i) => render(panes[i], r, terms));
  }

  function render(pn, r, terms) {
    if (r.status !== 'fulfilled') {
      pn.body.replaceChildren(emptyState('alert', 'Could not load logs', r.reason.message));
      return;
    }
    const entries = r.value.entries || [];
    pn.count.textContent = `${fmtNum(entries.length)} lines${r.value.next_cursor ? `, first ${fmtNum(LIMIT)}` : ''}`;
    if (!entries.length) {
      pn.body.replaceChildren(emptyState('inbox', 'No logs', 'This pod has no logs in the selected range.'));
      return;
    }
    const frag = document.createDocumentFragment();
    pn.rows = entries.map((e) => {
      const el = h('div', { class: `cmp-row lv-${e.level}`, title: `${e.timestamp}  ${e.container}`, onclick: () => el.classList.toggle('wrap') },
        h('span', { class: 'c-time' }, fmtTime(msOf(e))),
        h('span', null, h('span', { class: 'lv' }, e.level)),
        h('span', { class: 'c-msg' }, renderAnsi(e.message, (t) => highlight(t, terms))));
      frag.append(el);
      return { el, ms: msOf(e) };
    });
    pn.body.replaceChildren(frag);
  }

  // Scroll the other pane to the first line at or after the top line of this one.
  function follow(i) {
    if (skip[i]) {
      skip[i] = false;
      return;
    }
    if (p.sync === 'off') return;
    const src = panes[i];
    const dst = panes[1 - i];
    if (!src.rows.length || !dst.rows.length) return;
    const top = src.body.scrollTop;
    let lo = 0;
    let hi = src.rows.length - 1;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if (src.rows[mid].el.offsetTop < top) lo = mid + 1;
      else hi = mid;
    }
    const ts = src.rows[lo].ms;
    let a = 0;
    let b = dst.rows.length - 1;
    while (a < b) {
      const mid = (a + b) >> 1;
      if (dst.rows[mid].ms < ts) a = mid + 1;
      else b = mid;
    }
    const target = dst.rows[a].el.offsetTop;
    if (Math.abs(dst.body.scrollTop - target) > 1) {
      skip[1 - i] = true;
      dst.body.scrollTop = target;
    }
  }

  const offs = [on('catalog', () => {
    syncControls();
    if (keyOf({ ...p, ns: eff.ns, wl: eff.wl, a: eff.a, b: eff.b }) !== loadedKey) load();
  })];

  return {
    update(params) {
      p = params;
      syncControls();
      if (keyOf({ ...p, ns: eff.ns, wl: eff.wl, a: eff.a, b: eff.b }) !== loadedKey) load();
    },
    refresh: load,
    destroy: () => offs.forEach((off) => off()),
  };
}
