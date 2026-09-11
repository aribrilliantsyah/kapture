// Namespaces with their workloads and recent activity; a row sets the scope.
import { h, fmtNum, fmtAgo, emptyState } from '../ui.js';
import { store, on, setRoute, setScope, recap, rollupDays, namespaces, workloadsOf, dayOf } from '../state.js';

export function mount(root) {
  let alive = true;
  let perNs = new Map();
  const box = h('div', { class: 'card table-card' });
  root.append(h('div', { class: 'page' },
    h('div', { class: 'page-head' }, h('div', null, h('h1', null, 'Namespaces'), h('p', null, 'Pick one to scope the dashboard and workload lists to it'))),
    box));

  function render() {
    const list = namespaces();
    if (!list.length) {
      box.replaceChildren(emptyState('inbox', 'No logs collected yet', 'Namespaces appear once agents store their first lines.'));
      return;
    }
    const active = new Map();
    for (const w of workloadsOf('')) active.set(w.ns, (active.get(w.ns) || 0) + w.active);
    box.replaceChildren(h('div', { class: 'table-wrap' }, h('table', { class: 'table clickable' },
      h('thead', null, h('tr', null,
        h('th', null, 'Namespace'), h('th', { class: 'num' }, 'Workloads'), h('th', { class: 'num', title: 'Running now / seen in total' }, 'Pods'),
        h('th', { class: 'num' }, 'Lines (7d)'), h('th', { class: 'num' }, 'Errors (7d)'), h('th', { class: 'num' }, 'Last log'))),
      h('tbody', null, list.map((n) => {
        const r = perNs.get(n.name) || { lines: 0, errors: 0 };
        return h('tr', { onclick: () => { setScope(n.name); setRoute('workloads', {}); } },
          h('td', null, h('span', { class: 'strong' }, n.name), store.scope === n.name ? h('span', { class: 'badge ok', style: { marginLeft: '8px' } }, 'current scope') : null),
          h('td', { class: 'num' }, fmtNum(n.workloads)),
          h('td', { class: 'num' }, `${active.get(n.name) || 0} / ${n.pods}`),
          h('td', { class: 'num' }, fmtNum(r.lines)),
          h('td', { class: `num${r.errors ? ' err' : ''}` }, fmtNum(r.errors)),
          h('td', { class: 'num muted' }, fmtAgo(n.last)));
      })))));
  }

  async function load() {
    try {
      perNs = new Map();
      for (const r of rollupDays((await recap(7, '')).days, dayOf(Date.now() - 6 * 86400e3)).values()) {
        const t = perNs.get(r.ns) || { lines: 0, errors: 0 };
        t.lines += r.lines;
        t.errors += r.errors;
        perNs.set(r.ns, t);
      }
    } catch { /* counts stay empty */ }
    if (alive) render();
  }

  const offs = [on('catalog', render), on('scope', render)];
  render();
  load();
  return { refresh: load, destroy() { alive = false; offs.forEach((off) => off()); } };
}
