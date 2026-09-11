// One agent per node (DaemonSet): health, version and local storage.
import { api } from '../api.js';
import { h, fmtNum, fmtBytes, emptyState } from '../ui.js';
import { store, on, refreshAgents } from '../state.js';

export function mount(root) {
  let alive = true;
  let nodes = [];
  const box = h('div', { class: 'card table-card' });
  root.append(h('div', { class: 'page' },
    h('div', { class: 'page-head' }, h('div', null, h('h1', null, 'Agents'), h('p', null, 'One agent per node reads its container logs and keeps them on the node'))),
    box));

  function render() {
    const agents = store.agents;
    if (!agents.length) {
      box.replaceChildren(emptyState('server', 'No agents discovered',
        'The aggregator has not found any agent. Check the kapture-agent DaemonSet and the kapture-agents headless service.'));
      return;
    }
    const byNode = new Map(nodes.map((n) => [n.node, n]));
    box.replaceChildren(h('div', { class: 'table-wrap' }, h('table', { class: 'table' },
      h('thead', null, h('tr', null,
        h('th', null, 'Node'), h('th', null, 'Status'), h('th', null, 'Endpoint'), h('th', null, 'Version'), h('th', null, 'Time zone'),
        h('th', { class: 'num' }, 'Disk used'), h('th', { class: 'num' }, 'Lines'), h('th', { class: 'num' }, 'Days stored'))),
      h('tbody', null, agents.map((a) => {
        const n = byNode.get(a.node) || {};
        return h('tr', null,
          h('td', { class: 'strong' }, a.node || '-'),
          h('td', null, h('span', { class: `badge${a.status === 'ok' ? ' ok' : ' err'}`, title: a.error || '' }, a.status === 'ok' ? 'Ready' : 'Unreachable')),
          h('td', { class: 'muted' }, a.endpoint),
          h('td', { class: 'muted' }, a.version || '-'),
          h('td', n.timezone && store.timezone && n.timezone !== store.timezone
            ? { class: 'err-text', title: 'Differs from the aggregator: set the same KAPTURE_TIMEZONE on agents and aggregator' }
            : { class: 'muted' }, n.timezone || '-'),
          h('td', { class: 'num' }, n.max_bytes ? `${fmtBytes(n.used_bytes)} / ${fmtBytes(n.max_bytes)}` : '-'),
          h('td', { class: 'num' }, n.entry_count != null ? fmtNum(n.entry_count) : '-'),
          h('td', { class: 'num' }, n.oldest_date ? `${n.oldest_date} to ${n.newest_date}` : '-'));
      })))));
  }

  async function load() {
    try {
      nodes = (await api.get('/storage')).nodes || [];
    } catch { /* keep health only */ }
    await refreshAgents();
    if (alive) render();
  }

  const offs = [on('agents', render)];
  render();
  load();
  return { refresh: load, destroy() { alive = false; offs.forEach((off) => off()); } };
}
