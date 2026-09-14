// One workload: status, activity, full pod history and its logs.
import { api } from '../api.js';
import { h, icon, fmtNum, fmtDateTime, fmtAgo, fmtTime, msOf, emptyState, TYPE_TAGS } from '../ui.js';
import { on, setRoute, patchRoute, recap, findWorkload, rollupDays, dailySeries, perDay, sumCounts, isPinned, togglePin } from '../state.js';
import { volumeChart, lineChart, shareBar, levelParts, statTile, fmtPct } from '../chart.js';
import { toLocalInput, iso } from '../filters.js';
import { stripAnsi } from '../ansi.js';
import * as logsView from './logs.js';

const TABS = [['overview', 'Overview'], ['logs', 'Logs'], ['pods', 'Pods']];

export function mount(root) {
  let p = {};
  let tab = '';
  let key = '';
  let logs = null;
  let chart = null;
  let alive = true;

  const head = h('div', { class: 'page-head' });
  const tabs = h('div', { class: 'tabs', role: 'tablist' });
  const body = h('div', { class: 'tab-body' });
  const page = h('div', { class: 'page page-detail' }, head, tabs, body);
  root.append(page);

  const wl = () => findWorkload(p.ns, p.wl);
  const cardHead = (title, sub) => h('div', { class: 'card-head' }, h('div', null, h('h2', null, title), h('p', { class: 'card-sub' }, sub)));

  function renderHead() {
    const w = wl();
    head.replaceChildren(
      h('div', null,
        h('h1', null, p.wl || 'Workload'),
        h('p', { class: 'page-sub' }, `Namespace: ${p.ns}`, w ? h('span', { class: 'type-tag' }, TYPE_TAGS[w.type] || w.type) : null)),
      h('div', { class: 'page-actions' },
        h('button', { class: 'btn', type: 'button', onclick: () => togglePin(p.ns, p.wl) }, icon('pin'), isPinned(p.ns, p.wl) ? 'Unpin' : 'Pin'),
        w && w.pods.length > 1 ? h('button', { class: 'btn', type: 'button', onclick: () => setRoute('compare', { ns: p.ns, wl: p.wl }) }, icon('compare'), 'Compare pods') : null,
        h('button', { class: 'btn', type: 'button', onclick: () => setRoute('logs', { ns: p.ns, wl: p.wl }) }, icon('logs'), 'Open in Explorer')));
    tabs.replaceChildren(...TABS.map(([k, label]) => h('button', {
      class: `tab${tab === k ? ' active' : ''}`, type: 'button', role: 'tab',
      onclick: () => patchRoute({ tab: k === 'overview' ? '' : k }),
    }, label, k === 'pods' && w ? h('span', { class: 'badge' }, w.pods.length) : null)));
  }

  function renderTab() {
    logs?.destroy();
    logs = null;
    chart?.destroy();
    chart = null;
    body.replaceChildren();
    page.classList.toggle('fill', tab === 'logs');
    if (tab === 'logs') {
      const box = h('div', { class: 'embedded-logs' });
      body.append(box);
      logs = logsView.mount(box, { lock: { ns: p.ns, wl: p.wl } });
      logs.update(p);
    } else if (!wl()) {
      body.append(emptyState('inbox', 'No logs for this workload', `Nothing is stored for ${p.ns}/${p.wl}. Its logs may have been deleted.`));
    } else if (tab === 'pods') {
      renderPods();
    } else {
      renderOverview();
    }
  }

  async function renderOverview() {
    const myKey = key;
    const w = wl();
    const status = h('div', { class: 'card' });
    const skel = (hgt) => h('div', { class: 'skel', style: { height: hgt } });
    const kpis = h('div', { class: 'grid grid-4 mt' }, [0, 1, 2, 3].map(() => h('div', { class: 'card' }, skel('74px'))));
    const activity = h('div', { class: 'card' }, cardHead('Daily activity', 'Lines per day, last 30 days'), skel('190px'));
    const mix = h('div', { class: 'card' }, cardHead('Level mix', 'Share of lines per level, last 30 days'), skel('190px'));
    const rate = h('div', { class: 'card' }, cardHead('Error rate', 'ERROR and FATAL share of lines per day'), skel('170px'));
    const hourly = h('div', { class: 'card' }, cardHead('Last 24 hours', 'Lines per hour by level'), skel('170px'));
    const podsCard = h('div', { class: 'card' }, cardHead('Pods logging per day', ''), skel('150px'));
    const errors = h('div', { class: 'card' }, cardHead('Recent errors', 'Latest ERROR and FATAL lines'), skel('150px'));
    const list = [];
    chart = { destroy: () => list.forEach((c) => c.destroy()) };
    const keep = (c) => { list.push(c); return c.el; };
    const ratePct = (v) => fmtPct(v, v > 0 && v < 1 ? 2 : 1);
    const rateOf = (d) => (d.lines ? (d.errors / d.lines) * 100 : 0);
    const change = (now, before) => (before ? ((now - before) / before) * 100 : null);
    const openRange = (from, to) => patchRoute({ tab: 'logs', range: 'custom', date: '', from: toLocalInput(from), to: toLocalInput(Math.max(from, to - 60e3)) });
    const kv = (k, v) => h('div', null, h('div', { class: 'k' }, k), h('div', { class: 'v' }, v));
    const fillStatus = (r) => status.replaceChildren(h('h2', null, 'Status overview'), h('div', { class: 'kv-grid mt' },
      kv('Type', w.type),
      kv('Running pods', h('span', { class: `badge${w.active ? ' ok' : ''}` }, w.active ? `${w.active} running` : 'none running')),
      kv('Pods seen', fmtNum(w.pods.length)),
      kv('Containers', w.containers.join(', ')),
      kv('First log', fmtDateTime(w.first)),
      kv('Last log', `${fmtDateTime(w.last)} (${fmtAgo(w.last)})`),
      kv('Lines (30d)', r ? fmtNum(r.lines) : '-'),
      kv('Errors (30d)', r ? h('span', { class: r.errors ? 'err-text' : '' }, fmtNum(r.errors)) : '-')));
    fillStatus(null);
    body.append(status, kpis,
      h('div', { class: 'grid grid-2 mt' }, activity, mix),
      h('div', { class: 'grid grid-2 mt' }, rate, hourly),
      h('div', { class: 'grid grid-2 mt' }, errors, podsCard));

    const hourEnd = Math.floor(Date.now() / 3600e3) * 3600e3 + 3600e3;
    const [rc, er, hv] = await Promise.allSettled([
      recap(60, p.ns),
      api.get('/logs', { namespace: p.ns, workload: p.wl, level: 'ERROR,FATAL', limit: 8 }),
      api.get('/stats/volume', { namespace: p.ns, workload: p.wl, from: iso(hourEnd - 24 * 3600e3), to: iso(hourEnd), buckets: 24 }),
    ]);
    if (!alive || myKey !== key) return;

    if (rc.status === 'fulfilled') {
      const days = (rc.value.days || []).filter((d) => d.workload === p.wl);
      const both = perDay(days, 60);
      const series = both.slice(30);
      const prev = both.slice(0, 30);
      const sum = (xs, f) => xs.reduce((a, d) => a + f(d), 0);
      const lines = sum(series, (d) => d.lines);
      const errs = sum(series, (d) => d.errors);
      const prevLines = sum(prev, (d) => d.lines);
      const prevErrs = sum(prev, (d) => d.errors);
      const avgRate = lines ? (errs / lines) * 100 : 0;
      const prevRate = prevLines ? (prevErrs / prevLines) * 100 : 0;
      const today = series[series.length - 1];
      fillStatus(rollupDays(days, series[0].date).get(`${p.ns}/${p.wl}`) || { lines: 0, errors: 0 });

      kpis.replaceChildren(
        statTile({ icon: 'logs', label: 'Lines (30 days)', value: fmtNum(lines), delta: change(lines, prevLines), deltaText: 'vs previous 30 days', sub: 'No lines in the previous 30 days', spark: series.map((d) => d.lines), sparkLabel: 'Lines per day' }),
        statTile({ icon: 'alert', label: 'Errors (30 days)', value: fmtNum(errs), upGood: false, delta: change(errs, prevErrs), deltaText: 'vs previous 30 days', sub: errs ? 'None in the previous 30 days' : 'No ERROR or FATAL lines', spark: series.map((d) => d.errors), sparkLabel: 'Errors per day' }),
        statTile({ icon: 'percent', label: 'Error rate (30 days)', value: ratePct(avgRate), upGood: false, delta: prevRate && lines ? change(avgRate, prevRate) : null, deltaText: `vs ${ratePct(prevRate)} before`, sub: 'No earlier data to compare', spark: series.map(rateOf), sparkLabel: 'Error rate per day' }),
        statTile({ icon: 'box', label: 'Pods logging today', value: fmtNum(today.pods.size), sub: `${fmtNum(w.pods.length)} pods seen in total`, spark: series.map((d) => d.pods.size), sparkLabel: 'Pods logging per day' }));

      activity.replaceChildren(cardHead('Daily activity', 'Lines per day by level, last 30 days · click a day to open it'),
        keep(volumeChart(dailySeries(days, 30), { height: 190, daily: true, onSelect: openRange })));
      mix.replaceChildren(cardHead('Level mix', 'Share of lines per level, last 30 days'),
        shareBar(levelParts(sumCounts(series.map((d) => d.levels)), sumCounts(prev.map((d) => d.levels))), { prevName: 'previous 30 days' }));
      rate.replaceChildren(cardHead('Error rate', 'ERROR and FATAL share of lines per day · click a day to see its errors'),
        keep(lineChart(series.map((d) => ({ ms: d.ms, y: rateOf(d), title: d.date, rows: [['errors', fmtNum(d.errors)], ['lines', fmtNum(d.lines)]] })), {
          height: 170, format: ratePct, color: 'var(--lv-error)', name: 'error rate',
          summary: `Average ${ratePct(avgRate)} · previous 30 days ${prevLines ? ratePct(prevRate) : 'no data'}`,
          onSelect: (pt) => patchRoute({ tab: 'logs', range: '', from: '', to: '', date: pt.title, lv: 'ERROR,FATAL' }),
        })));
      podsCard.replaceChildren(cardHead('Pods logging per day', 'Distinct pods that wrote logs each day; a jump usually means a rollout or restarts'),
        keep(lineChart(series.map((d) => ({ ms: d.ms, y: d.pods.size, title: d.date, rows: [['lines', fmtNum(d.lines)]] })), {
          height: 150, name: 'pods', summary: `Now ${fmtNum(today.pods.size)} · peak ${fmtNum(Math.max(...series.map((d) => d.pods.size)))}`,
        })));
    } else {
      for (const [c, t] of [[activity, 'Daily activity'], [mix, 'Level mix'], [rate, 'Error rate'], [podsCard, 'Pods logging per day']]) {
        c.replaceChildren(cardHead(t, ''), h('p', { class: 'error-text' }, rc.reason.message));
      }
      kpis.replaceChildren();
    }

    if (hv.status === 'fulfilled') {
      hourly.replaceChildren(cardHead('Last 24 hours', 'Lines per hour by level · click an hour to open it'),
        keep(volumeChart(hv.value, { height: 170, onSelect: openRange })));
    } else {
      hourly.replaceChildren(cardHead('Last 24 hours', ''), h('p', { class: 'error-text' }, hv.reason.message));
    }

    if (er.status === 'fulfilled') {
      const entries = er.value.entries || [];
      errors.replaceChildren(cardHead('Recent errors', 'Latest ERROR and FATAL lines'),
        entries.length
          ? h('div', { class: 'list-rows' }, entries.map((e) => {
            const ms = msOf(e);
            return h('div', {
              class: `list-row err-row lv-${e.level}`, title: e.pod,
              onclick: () => patchRoute({ tab: 'logs', pod: e.pod, range: 'custom', date: '', from: toLocalInput(ms - 60e3), to: toLocalInput(ms + 60e3) }),
            },
              h('span', { class: 'muted' }, fmtTime(ms).slice(0, 8)),
              h('span', null, h('span', { class: 'lv' }, e.level)),
              h('span', { class: 'ellipsis' }, stripAnsi(e.message.split('\n')[0])));
          }))
          : emptyState('check', 'No errors stored', 'This workload has no ERROR or FATAL lines.'));
    } else {
      errors.replaceChildren(cardHead('Recent errors', ''), h('p', { class: 'error-text' }, er.reason.message));
    }
  }

  function renderPods() {
    const w = wl();
    const revision = (pod) => (w.type === 'deployment' && pod.startsWith(w.name + '-') ? pod.slice(w.name.length + 1).replace(/-[a-z0-9]{5}$/, '') : '');
    const showRev = w.type === 'deployment';
    body.replaceChildren(h('div', { class: 'card table-card' },
      h('div', { class: 'card-head', style: { padding: '12px 10px 4px' } },
        h('div', null, h('h2', null, 'Pod history'),
          h('p', { class: 'card-sub' }, 'Every pod that has logged for this workload, newest first. Replaced pods keep their logs. Running means it logged in the last 10 minutes.'))),
      h('div', { class: 'table-wrap' }, h('table', { class: 'table clickable' },
        h('thead', null, h('tr', null,
          h('th', null, 'Pod'), h('th', null, 'Status'), showRev ? h('th', null, 'ReplicaSet') : null,
          h('th', null, 'Node'), h('th', null, 'Containers'), h('th', null, 'First log'), h('th', null, 'Last log'), h('th'))),
        h('tbody', null, w.pods.map((pod) => h('tr', { onclick: () => patchRoute({ tab: 'logs', pod: pod.name, c: '' }) },
          h('td', { class: 'strong' }, pod.name),
          h('td', null, h('span', { class: `badge${pod.active ? ' ok' : ''}` }, pod.active ? 'Running' : 'Stopped')),
          showRev ? h('td', { class: 'muted' }, revision(pod.name) || '-') : null,
          h('td', { class: 'muted' }, pod.node || '-'),
          h('td', null, pod.containers.join(', ')),
          h('td', { class: 'muted' }, fmtDateTime(pod.first)),
          h('td', null, fmtDateTime(pod.last), h('div', { class: 'sub' }, fmtAgo(pod.last))),
          h('td', { class: 'num' }, h('span', { class: 'btn btn-sm' }, 'Logs', icon('arrow-right'))))))))));
  }

  const offs = [
    on('catalog', () => { renderHead(); if (tab === 'pods' && wl()) renderPods(); }),
    on('pins', renderHead),
  ];

  return {
    update(params) {
      p = params;
      const nextTab = TABS.some(([k]) => k === p.tab) ? p.tab : 'overview';
      const nextKey = `${p.ns}/${p.wl}/${nextTab}`;
      if (nextKey !== key) {
        key = nextKey;
        tab = nextTab;
        renderHead();
        renderTab();
      } else {
        renderHead();
        logs?.update(p);
      }
    },
    refresh() {
      if (logs) logs.refresh();
      else renderTab();
    },
    destroy() {
      alive = false;
      logs?.destroy();
      chart?.destroy();
      offs.forEach((off) => off());
    },
  };
}
