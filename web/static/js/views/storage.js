// Disk usage per day and per node, backup and restore, delete and reset.
import { api, qs, redirectToLogin } from '../api.js';
import { h, icon, fmtNum, fmtBytes, localDate, tzLabel, toast, confirmDialog, emptyState } from '../ui.js';
import { store, refreshCatalog, clearRecap } from '../state.js';

function fmtRetention(r) {
  if (r === 'unlimited') return 'Keep forever';
  const hours = Number(/(\d+)h/.exec(r || '')?.[1] || 0);
  if (!hours) return r || '-';
  return hours % 24 === 0 ? `${hours / 24} days` : `${hours} hours`;
}

export function mount(root) {
  let alive = true;
  const el = h('div', { class: 'page' });
  root.append(el);
  const head = h('div', { class: 'page-head' },
    h('div', null, h('h1', null, 'Storage'), h('p', null, 'Each agent keeps the logs of its own node on local disk')));

  const card = (title, sub, ...content) => h('div', { class: 'card' },
    h('div', { class: 'card-head' }, h('div', null, h('h2', null, title), h('p', { class: 'card-sub' }, sub))), ...content);
  const statCard = (label, value, sub) => h('div', { class: 'card' },
    h('div', { class: 'stat-label' }, label), h('div', { class: 'stat-value' }, value), h('div', { class: 'muted' }, sub));

  async function load() {
    el.replaceChildren(head, h('div', { class: 'grid grid-4' },
      [0, 1, 2, 3].map(() => h('div', { class: 'card' }, h('div', { class: 'skel', style: { height: '62px' } })))));
    let info;
    try {
      info = await api.get('/storage');
    } catch (e) {
      if (alive) el.replaceChildren(head, emptyState('alert', 'Could not load storage info', e.message));
      return;
    }
    if (alive) render(info);
  }

  // Built once, so an upload in progress survives the page re-rendering.
  const backupEl = backupCard();

  // Download a .tar.gz of every node's logs, or load one (from this or another
  // cluster, e.g. production logs into a Kapture on a laptop).
  function backupCard() {
    const today = localDate();
    const from = h('input', { type: 'date', class: 'input', max: today, 'aria-label': 'First day' });
    const to = h('input', { type: 'date', class: 'input', max: today, 'aria-label': 'Last day' });
    const file = h('input', { type: 'file', accept: '.gz,.tgz,application/gzip', hidden: true });
    const bar = h('div', { style: { width: '0%' } });
    const meter = h('div', { class: 'meter', hidden: true }, bar);
    const status = h('p', { class: 'muted', hidden: true });
    const results = h('div');
    const downloadBtn = h('button', { class: 'btn btn-primary', type: 'button' }, icon('download'), 'Download');
    const restoreBtn = h('button', { class: 'btn', type: 'button', onclick: () => file.click() }, icon('upload'), 'Choose backup file');

    downloadBtn.addEventListener('click', () => {
      if (from.value && to.value && from.value > to.value) return toast('The first day is after the last day', 'error');
      if (!store.agents.some((a) => a.status === 'ok')) return toast('No reachable agent to back up', 'error');
      const a = h('a', { href: '/api/v1/storage/backup' + qs({ from: from.value, to: to.value }), download: '' });
      document.body.append(a);
      a.click();
      a.remove();
      toast('Backup started, the download grows while the agents send their logs');
    });

    file.addEventListener('change', async () => {
      const f = file.files[0];
      file.value = '';
      if (!f) return;
      const ok = await confirmDialog({
        title: `Restore ${f.name}?`,
        body: `${fmtBytes(f.size)} of logs are added to what is stored. Nothing is deleted and lines already stored are not duplicated. `
          + 'Each node\'s logs go to the agent of the same node, or to the first agent when that node is not in this cluster.',
        confirmText: 'Restore',
      });
      if (ok) upload(f);
    });

    function upload(f) {
      downloadBtn.disabled = restoreBtn.disabled = true;
      meter.hidden = status.hidden = false;
      bar.style.width = '0%';
      status.className = 'muted';
      status.textContent = 'Uploading...';
      results.replaceChildren();
      const finish = () => {
        downloadBtn.disabled = restoreBtn.disabled = false;
        meter.hidden = true;
      };
      const xhr = new XMLHttpRequest(); // fetch() cannot report upload progress
      xhr.open('POST', '/api/v1/storage/restore');
      xhr.setRequestHeader('Content-Type', 'application/gzip');
      xhr.upload.onprogress = (e) => {
        if (!e.lengthComputable) return;
        const pct = Math.round((e.loaded / e.total) * 100);
        bar.style.width = `${pct}%`;
        status.textContent = pct < 100 ? `Uploading ${pct}% of ${fmtBytes(e.total)}` : 'Restoring on the agents, this can take a while...';
      };
      xhr.onload = () => {
        finish();
        if (xhr.status === 401) return redirectToLogin();
        let data = {};
        try { data = JSON.parse(xhr.responseText); } catch { /* not JSON */ }
        const failed = xhr.status >= 400;
        status.className = failed ? 'error-text' : 'muted';
        status.textContent = failed ? (data.error || `Restore failed (HTTP ${xhr.status})`) : `Restored ${fmtNum(data.restored || 0)} lines from ${f.name}`;
        toast(status.textContent, failed ? 'error' : 'ok');
        showResults(data.results || []);
        clearRecap();
        refreshCatalog();
        load();
      };
      xhr.onerror = () => {
        finish();
        status.className = 'error-text';
        status.textContent = 'Upload failed: cannot reach the Kapture server';
        toast(status.textContent, 'error');
      };
      xhr.send(f);
    }

    function showResults(list) {
      if (!list.length) return results.replaceChildren();
      results.replaceChildren(h('div', { class: 'table-wrap mt-sm' }, h('table', { class: 'table' },
        h('thead', null, h('tr', null, h('th', null, 'Node in the backup'), h('th', null, 'Restored into'), h('th', { class: 'num' }, 'Lines'), h('th', null, 'Result'))),
        h('tbody', null, list.map((r) => h('tr', null,
          h('td', { class: 'strong' }, r.node),
          h('td', { class: 'muted' }, r.target || '-'),
          h('td', { class: 'num' }, fmtNum(r.entries)),
          h('td', null, r.error ? h('span', { class: 'badge err', title: r.error }, r.error) : h('span', { class: 'badge ok' }, 'OK'))))))));
    }

    return h('div', { class: 'card', style: { marginTop: '14px' } },
      h('div', { class: 'card-head' }, h('div', null, h('h2', null, 'Backup and restore'),
        h('p', { class: 'card-sub' }, 'One .tar.gz with the logs of every node. Restore it here or into another Kapture, for example on your laptop.'))),
      h('div', { class: 'danger-row' },
        h('div', null, h('b', null, 'Download a backup'), h('p', null, 'Every stored day, or only the days between two dates (both optional).')),
        h('span', { class: 'input-group' }, from, h('span', { class: 'muted' }, 'to'), to, downloadBtn)),
      h('div', { class: 'danger-row' },
        h('div', null, h('b', null, 'Restore a backup'), h('p', null, 'Adds the lines of a Kapture backup file. Nothing is deleted.')),
        h('span', { class: 'input-group' }, restoreBtn, file)),
      meter, status, results);
  }

  async function run(req, what) {
    try {
      const res = req.all ? await api.del('/logs/all') : await api.del('/logs', req);
      toast(`Deleted ${fmtNum(res.deleted)} lines ${what}`);
      if (res.errors?.length) toast(`Some agents failed: ${res.errors.join('; ')}`, 'error');
    } catch (e) {
      toast(e.message, 'error');
    }
    refreshCatalog();
    load();
  }

  function render(info) {
    const pct = info.max_bytes ? (info.used_bytes / info.max_bytes) * 100 : 0;
    const dates = info.dates || [];
    const nodes = info.nodes || [];
    const maxLines = Math.max(1, ...dates.map((d) => d.entry_count));
    const today = localDate();

    const datesTable = dates.length
      ? h('div', { class: 'table-wrap' }, h('table', { class: 'table' },
        h('thead', null, h('tr', null, h('th', null, `Day (${tzLabel()})`), h('th', { class: 'num' }, 'Lines'), h('th', { class: 'num' }, 'Size'), h('th', { style: { width: '28%' } }), h('th'))),
        h('tbody', null, dates.map((d) => h('tr', null,
          h('td', null, d.date, d.date === today ? h('span', { class: 'muted' }, '  today') : null),
          h('td', { class: 'num' }, fmtNum(d.entry_count)),
          h('td', { class: 'num' }, fmtBytes(d.size_bytes)),
          h('td', null, h('span', { class: 'inline-bar', style: { width: `${(d.entry_count / maxLines) * 100}%` } })),
          h('td', { class: 'num' }, h('button', {
            class: 'icon-btn sm', type: 'button', title: `Delete ${d.date}`,
            onclick: async () => {
              const ok = await confirmDialog({
                title: `Delete logs of ${d.date}?`,
                body: `This removes ${fmtNum(d.entry_count)} lines stored for ${d.date} on every node. It cannot be undone.`,
                confirmText: 'Delete day', danger: true,
              });
              if (ok) run({ date: d.date }, `from ${d.date}`);
            },
          }, icon('trash'))))))))
      : emptyState('inbox', 'No stored logs', 'Agents have not written any logs yet.');

    const nodesTable = nodes.length
      ? h('div', { class: 'table-wrap' }, h('table', { class: 'table' },
        h('thead', null, h('tr', null, h('th', null, 'Node'), h('th', { class: 'num' }, 'Used'), h('th', { class: 'num' }, 'Lines'), h('th', { class: 'num' }, 'Days'))),
        h('tbody', null, nodes.map((n) => h('tr', null,
          h('td', { class: n.max_bytes ? '' : 'error-text' }, n.node || '-'),
          h('td', { class: 'num' }, n.max_bytes ? `${fmtBytes(n.used_bytes)} / ${fmtBytes(n.max_bytes)}` : '-'),
          h('td', { class: 'num' }, fmtNum(n.entry_count)),
          h('td', { class: 'num' }, n.oldest_date ? `${n.oldest_date.slice(5)} to ${n.newest_date.slice(5)}` : '-'))))))
      : emptyState('server', 'No agents discovered', 'The aggregator cannot see any agent.');

    const before = h('input', { type: 'date', class: 'input', max: localDate(), 'aria-label': 'Delete before date' });
    const danger = h('div', { class: 'card danger-zone', style: { marginTop: '14px' } },
      h('div', { class: 'card-head' }, h('div', null, h('h2', null, 'Clean up'), h('p', { class: 'card-sub' }, 'Deletes run on every node and cannot be undone'))),
      h('div', { class: 'danger-row' },
        h('div', null, h('b', null, 'Delete older logs'), h('p', null, 'Remove every day before the chosen date.')),
        h('span', { class: 'input-group' }, before, h('button', {
          class: 'btn btn-danger', type: 'button',
          onclick: async () => {
            if (!before.value) return toast('Pick a date first', 'error');
            const ok = await confirmDialog({
              title: `Delete logs before ${before.value}?`,
              body: 'Every stored day before this date is removed from all nodes.',
              confirmText: 'Delete', danger: true,
            });
            if (ok) run({ before: before.value }, `before ${before.value}`);
          },
        }, icon('trash'), 'Delete'))),
      h('div', { class: 'danger-row' },
        h('div', null, h('b', null, 'Reset all logs'), h('p', null, 'Empty the log database on every node. Collection keeps running.')),
        h('button', {
          class: 'btn btn-danger', type: 'button',
          onclick: async () => {
            const ok = await confirmDialog({
              title: 'Reset all logs?',
              body: `All ${fmtNum(info.entry_count)} stored lines on ${nodes.length} nodes will be deleted.`,
              confirmText: 'Reset everything', danger: true, typeToConfirm: 'RESET',
            });
            if (ok) run({ all: true }, 'in total');
          },
        }, 'Reset all logs')));

    el.replaceChildren(head,
      h('div', { class: 'grid grid-4' },
        h('div', { class: 'card' },
          h('div', { class: 'stat-label' }, 'Disk used'),
          h('div', { class: 'stat-value' }, fmtBytes(info.used_bytes)),
          h('div', { class: `meter${pct > 90 ? ' bad' : pct > 70 ? ' warn' : ''}` }, h('div', { style: { width: `${Math.min(100, pct).toFixed(1)}%` } })),
          h('div', { class: 'muted' }, `${pct.toFixed(1)}% of ${fmtBytes(info.max_bytes)}`)),
        statCard('Stored lines', fmtNum(info.entry_count), `${dates.length} days`),
        statCard('Date range', info.oldest_date ? `${info.oldest_date.slice(5)} to ${info.newest_date.slice(5)}` : '-', info.oldest_date ? `${info.oldest_date} to ${info.newest_date}` : 'No data yet'),
        statCard('Retention', fmtRetention(info.retention), info.retention === 'unlimited' ? 'Only max disk or a manual delete removes logs' : 'Older days are dropped automatically')),
      h('div', { class: 'grid grid-2e', style: { marginTop: '14px' } },
        card('Per day', 'Summed over all nodes', datesTable),
        card('Per node', 'Usage of each agent', nodesTable)),
      backupEl,
      danger);
  }

  load();
  return { refresh: load, destroy() { alive = false; } };
}
