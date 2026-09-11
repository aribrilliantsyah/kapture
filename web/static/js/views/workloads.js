// Workload list per kind, grouped by owner across pod generations.
import { h, icon, fmtNum, fmtAgo, emptyState, TYPE_TAGS } from '../ui.js';
import { store, on, setRoute, recap, rollupDays, workloadsOf, groupOf, TYPE_GROUPS, isPinned, togglePin, dayOf } from '../state.js';

const COLS = [
  { key: 'name', label: 'Name' },
  { key: 'ns', label: 'Namespace' },
  { key: 'pods', label: 'Pods', num: true, title: 'Running now / seen in total' },
  { key: 'lines', label: 'Lines (7d)', num: true },
  { key: 'errors', label: 'Errors (7d)', num: true },
  { key: 'last', label: 'Last log', num: true },
];

export function pinButton(ns, wl) {
  const on = isPinned(ns, wl);
  return h('button', {
    class: `icon-btn sm pin${on ? ' on' : ''}`, type: 'button', title: on ? 'Unpin from sidebar' : 'Pin to sidebar',
    onclick: (e) => { e.stopPropagation(); togglePin(ns, wl); },
  }, icon('pin'));
}

export function mount(root) {
  let p = {};
  let roll = new Map();
  let sort = { key: 'name', dir: 1 };
  let text = '';
  let alive = true;
  let loaded = false;

  const title = h('h1');
  const filter = h('input', {
    class: 'input', type: 'search', placeholder: 'Filter by name', 'aria-label': 'Filter workloads', style: { width: '240px' },
    oninput: (e) => { text = e.target.value.trim().toLowerCase(); renderTable(); },
  });
  const box = h('div', { class: 'card table-card' });
  root.append(h('div', { class: 'page' },
    h('div', { class: 'page-head' },
      h('div', null, title, h('p', null, 'Grouped by owner: pods from every rollout or restart stay under one workload')),
      h('div', { class: 'page-actions' }, filter)),
    box));

  function rows() {
    const g = TYPE_GROUPS.find((x) => x.key === p.type);
    return workloadsOf(store.scope)
      .filter((w) => (!g || groupOf(w.type).key === g.key) && (!text || w.name.toLowerCase().includes(text) || w.ns.includes(text)))
      .map((w) => {
        const r = roll.get(`${w.ns}/${w.name}`);
        return { w, name: w.name, ns: w.ns, pods: w.active, lines: r?.lines || 0, errors: r?.errors || 0, last: w.last };
      })
      .sort((a, b) => {
        const x = a[sort.key];
        const y = b[sort.key];
        return (typeof x === 'string' ? x.localeCompare(y) : x - y) * sort.dir || a.name.localeCompare(b.name);
      });
  }

  function renderTable() {
    const list = rows();
    if (!list.length) {
      box.replaceChildren(store.catalog.length
        ? emptyState('inbox', 'No workloads here', text ? 'Nothing matches this filter.' : 'Nothing of this kind has logged in the current namespace scope.')
        : emptyState('inbox', 'No logs collected yet', 'Workloads appear as soon as agents store their first lines.'));
      return;
    }
    const th = (c) => h('th', {
      class: `sortable${c.num ? ' num' : ''}${sort.key === c.key ? ' sorted' : ''}`, title: c.title,
      onclick: () => {
        sort = { key: c.key, dir: sort.key === c.key ? -sort.dir : c.num ? -1 : 1 };
        renderTable();
      },
    }, c.label, sort.key === c.key ? (sort.dir > 0 ? ' ↑' : ' ↓') : '');

    box.replaceChildren(
      h('div', { class: 'table-wrap' }, h('table', { class: 'table clickable' },
        h('thead', null, h('tr', null, COLS.map(th), h('th'))),
        h('tbody', null, list.map((r) => h('tr', { onclick: () => setRoute('workload', { ns: r.ns, wl: r.name }) },
          h('td', null, h('span', { class: 'strong' }, r.name), ' ', h('span', { class: 'type-tag' }, TYPE_TAGS[r.w.type] || r.w.type)),
          h('td', { class: 'muted' }, r.ns),
          h('td', { class: 'num' }, h('span', { class: `badge${r.w.active ? ' ok' : ''}`, title: `${r.w.active} running now, ${r.w.pods.length} seen in total` }, `${r.w.active} / ${r.w.pods.length}`)),
          h('td', { class: 'num' }, fmtNum(r.lines)),
          h('td', { class: `num${r.errors ? ' err' : ''}` }, fmtNum(r.errors)),
          h('td', { class: 'num muted' }, fmtAgo(r.last)),
          h('td', { class: 'num' }, pinButton(r.ns, r.name))))))),
      h('div', { class: 'table-foot' }, `${list.length} workloads${store.scope ? ` in ${store.scope}` : ''}`));
  }

  async function load() {
    try {
      roll = rollupDays((await recap(7, store.scope)).days, dayOf(Date.now() - 6 * 86400e3));
    } catch { /* counts stay empty */ }
    if (alive) renderTable();
  }

  const offs = [on('catalog', renderTable), on('pins', renderTable), on('scope', load)];
  return {
    update(params) {
      p = params;
      const g = TYPE_GROUPS.find((x) => x.key === p.type);
      title.textContent = g ? g.label : 'All workloads';
      renderTable();
      if (!loaded) {
        loaded = true;
        load();
      }
    },
    refresh: load,
    destroy() {
      alive = false;
      offs.forEach((off) => off());
    },
  };
}
