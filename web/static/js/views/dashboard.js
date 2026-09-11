// Dashboard: today against yesterday, then the chosen period (7, 14 or 30
// days) against the period before: volume, level mix, error rate, error
// sources, the last 24 hours, busiest workloads and system health.
import { api } from '../api.js';
import { h, fmtNum, fmtBytes, fmtTime, msOf, emptyState, TYPE_TAGS } from '../ui.js';
import { store, on, setRoute, patchRoute, recap, workloadsOf, rollupDays, dailySeries, perDay, sumCounts } from '../state.js';
import { volumeChart, lineChart, shareBar, levelParts, statTile, sparkline, fmtPct } from '../chart.js';
import { iso, toLocalInput } from '../filters.js';
import { stripAnsi } from '../ansi.js';

const PERIODS = [7, 14, 30];
const DEFAULT_DAYS = 14;
const HOUR = 3600e3;

const rateOf = (d) => (d.lines ? (d.errors / d.lines) * 100 : 0);
const pctChange = (now, before) => (before ? ((now - before) / before) * 100 : null);
const ratePct = (v) => fmtPct(v, v > 0 && v < 1 ? 2 : 1);

export function mount(root) {
  let alive = true;
  let days = 0;
  let reqId = 0;
  let charts = [];
  let data = null;

  const sub = h('p');
  const period = h('div', { class: 'period', role: 'group', 'aria-label': 'Period' });
  const kpis = h('div', { class: 'grid grid-4' });
  const cards = {};
  for (const k of ['vol', 'mix', 'rate', 'top', 'hour', 'recent', 'busy', 'sys']) cards[k] = h('div', { class: 'card' });
  const page = h('div', { class: 'page' },
    h('div', { class: 'page-head' }, h('div', null, h('h1', null, 'Dashboard'), sub), h('div', { class: 'page-actions' }, period)),
    kpis,
    h('div', { class: 'grid grid-2 mt' }, cards.vol, cards.mix),
    h('div', { class: 'grid grid-2 mt' }, cards.rate, cards.top),
    h('div', { class: 'grid grid-2 mt' }, cards.hour, cards.recent),
    h('div', { class: 'grid grid-2 mt' }, cards.busy, cards.sys));
  root.append(page);

  const head = (title, subtitle, action) => h('div', { class: 'card-head' },
    h('div', null, h('h2', null, title), h('p', { class: 'card-sub' }, subtitle)), action);
  const skel = (hgt) => h('div', { class: 'skel', style: { height: hgt } });
  const failed = (card, title, reason) => card.replaceChildren(head(title, ''), h('p', { class: 'error-text' }, reason.message));
  const keep = (c) => { charts.push(c); return c.el; };
  const logsRoute = (extra) => setRoute('logs', { ns: store.scope, ...extra });
  const openRange = (from, to) => logsRoute({ range: 'custom', from: toLocalInput(from), to: toLocalInput(Math.max(from, to - 60e3)) });

  function renderPeriod() {
    period.replaceChildren(...PERIODS.map((n) => h('button', {
      type: 'button', class: n === days ? 'active' : '', 'aria-pressed': String(n === days),
      onclick: () => patchRoute({ days: n === DEFAULT_DAYS ? '' : String(n) }),
    }, `${n} days`)));
  }

  function skeletons() {
    kpis.replaceChildren(...[0, 1, 2, 3].map(() => h('div', { class: 'card' }, skel('74px'))));
    const titles = { vol: 'Log volume', mix: 'Level mix', rate: 'Error rate', top: 'Top error sources', hour: 'Last 24 hours', recent: 'Recent errors', busy: 'Busiest workloads', sys: 'System' };
    for (const [k, t] of Object.entries(titles)) cards[k].replaceChildren(head(t, ''), skel(k === 'sys' || k === 'recent' ? '150px' : '210px'));
  }

  // ── KPI row: today against yesterday ──
  function renderKpis() {
    const { series } = data;
    const today = series[series.length - 1];
    const yday = series[series.length - 2];
    const total = series.reduce((a, d) => a + d.lines, 0);
    const avgRate = total ? (series.reduce((a, d) => a + d.errors, 0) / total) * 100 : 0;
    kpis.replaceChildren(
      statTile({
        icon: 'logs', label: 'Lines today', value: fmtNum(today.lines),
        delta: pctChange(today.lines, yday.lines), deltaText: 'vs yesterday', sub: 'No lines yesterday',
        spark: series.map((d) => d.lines), sparkLabel: `Lines per day, last ${days} days`,
      }),
      statTile({
        icon: 'alert', label: 'Errors today', value: fmtNum(today.errors), upGood: false,
        delta: pctChange(today.errors, yday.errors), deltaText: 'vs yesterday',
        sub: today.errors ? 'None yesterday' : 'No ERROR or FATAL lines',
        spark: series.map((d) => d.errors), sparkLabel: `Errors per day, last ${days} days`,
      }),
      statTile({
        icon: 'percent', label: 'Error rate today', value: ratePct(rateOf(today)), upGood: false,
        delta: today.lines && avgRate ? pctChange(rateOf(today), avgRate) : null, deltaText: `vs ${ratePct(avgRate)} average`,
        sub: `${ratePct(avgRate)} average over ${days} days`,
        spark: series.map(rateOf), sparkLabel: `Error rate per day, last ${days} days`,
      }),
      statTile({
        icon: 'layers', label: 'Active workloads', value: fmtNum(today.workloads.size),
        sub: `of ${fmtNum(workloadsOf(store.scope).length)} known, ${fmtNum(today.pods.size)} pods logged today`,
        spark: series.map((d) => d.workloads.size), sparkLabel: `Workloads logging per day, last ${days} days`,
      }));
  }

  function renderCharts(all) {
    const { series, prev } = data;
    const total = series.reduce((a, d) => a + d.lines, 0);
    const errors = series.reduce((a, d) => a + d.errors, 0);
    const prevTotal = prev.reduce((a, d) => a + d.lines, 0);
    const prevRate = prevTotal ? (prev.reduce((a, d) => a + d.errors, 0) / prevTotal) * 100 : 0;
    const avgRate = total ? (errors / total) * 100 : 0;
    const since = series[0].date;
    const prevSince = prev[0].date;
    const cur = rollupDays(all, since);
    const before = rollupDays(all.filter((d) => d.date < since), prevSince);
    const wlDays = (r) => perDay(all, days, (d) => d.namespace === r.ns && d.workload === r.name);

    cards.vol.replaceChildren(
      head('Log volume', `${fmtNum(total)} lines in ${days} days, ${pctText(pctChange(total, prevTotal))} · click a day to open it`),
      keep(volumeChart(dailySeries(all, days), { height: 210, daily: true, onSelect: openRange })));

    cards.mix.replaceChildren(
      head('Level mix', `Share of all lines per level, last ${days} days`),
      shareBar(levelParts(sumCounts(series.map((d) => d.levels)), sumCounts(prev.map((d) => d.levels))), { prevName: `previous ${days} days` }));

    cards.rate.replaceChildren(
      head('Error rate', 'ERROR and FATAL lines as a share of all lines, per day · click a day to see its errors'),
      keep(lineChart(series.map((d) => ({
        ms: d.ms, y: rateOf(d), title: d.date, date: d.date,
        rows: [['errors', fmtNum(d.errors)], ['lines', fmtNum(d.lines)]],
      })), {
        height: 190, format: ratePct, color: 'var(--lv-error)', name: 'error rate',
        summary: `Average ${ratePct(avgRate)} · previous ${days} days ${prevTotal ? ratePct(prevRate) : 'no data'}`,
        onSelect: (p) => logsRoute({ date: p.date, lv: 'ERROR,FATAL' }),
      })));

    const top = [...cur.values()].filter((r) => r.errors).sort((a, b) => b.errors - a.errors).slice(0, 8);
    const maxErr = Math.max(1, ...top.map((r) => r.errors));
    cards.top.replaceChildren(head('Top error sources', `ERROR and FATAL lines, last ${days} days, with the daily trend`),
      top.length
        ? h('div', { class: 'list-rows' }, top.map((r) => {
          const was = before.get(`${r.ns}/${r.name}`)?.errors || 0;
          return h('div', {
            class: 'list-row top-err', title: `${fmtNum(r.errors)} errors, ${ratePct((r.errors / r.lines) * 100)} of its lines`,
            onclick: () => setRoute('workload', { ns: r.ns, wl: r.name, tab: 'logs', lv: 'ERROR,FATAL', range: 'custom', from: toLocalInput(series[0].ms), to: toLocalInput(Date.now()) }),
          },
            h('div', { class: 'ellipsis' },
              h('div', { class: 'ellipsis' }, r.name, h('span', { class: 'sub' }, `  ${r.ns}`)),
              h('div', { class: 'bar', style: { width: `${(r.errors / maxErr) * 100}%` } })),
            sparkline(wlDays(r).map((d) => d.errors), { width: 72, height: 24, label: `Errors per day of ${r.name}` }),
            h('div', { class: 'num-pair' }, h('b', null, fmtNum(r.errors)), h('small', null, was ? pctText(pctChange(r.errors, was)) : 'new')));
        }))
        : emptyState('check', 'No errors', `No ERROR or FATAL lines in the last ${days} days.`));

    const busy = [...cur.values()].sort((a, b) => b.lines - a.lines).slice(0, 10);
    cards.busy.replaceChildren(head('Busiest workloads', `Lines logged in the last ${days} days`),
      busy.length
        ? h('div', { class: 'table-wrap' }, h('table', { class: 'table clickable' },
          h('thead', null, h('tr', null, h('th', null, 'Workload'), h('th', { class: 'num' }, 'Lines'), h('th', { class: 'num' }, 'Share'),
            h('th', { class: 'num' }, 'Errors'), h('th', { class: 'num' }, 'Error rate'), h('th', { class: 'num' }, 'Trend'))),
          h('tbody', null, busy.map((r) => h('tr', { onclick: () => setRoute('workload', { ns: r.ns, wl: r.name }) },
            h('td', null, h('span', { class: 'strong' }, r.name), ' ', h('span', { class: 'type-tag' }, TYPE_TAGS[r.type] || r.type), h('div', { class: 'sub' }, r.ns)),
            h('td', { class: 'num' }, fmtNum(r.lines)),
            h('td', { class: 'num' }, fmtPct((r.lines / total) * 100)),
            h('td', { class: `num${r.errors ? ' err' : ''}` }, fmtNum(r.errors)),
            h('td', { class: 'num' }, ratePct((r.errors / r.lines) * 100)),
            h('td', { class: 'num' }, h('div', { class: 'trend-cell' }, sparkline(wlDays(r).map((d) => d.lines), { width: 72, height: 22, label: `Lines per day of ${r.name}` }))))))))
        : emptyState('inbox', 'No logs yet', 'Workloads show up here once agents store their lines.'));
  }

  function renderHourly(res) {
    if (res.status !== 'fulfilled') return failed(cards.hour, 'Last 24 hours', res.reason);
    cards.hour.replaceChildren(head('Last 24 hours', 'Lines per hour by level · click an hour to open it'),
      keep(volumeChart(res.value, { height: 190, onSelect: openRange })));
  }

  function renderRecent(res) {
    if (res.status !== 'fulfilled') return failed(cards.recent, 'Recent errors', res.reason);
    const entries = res.value.entries || [];
    cards.recent.replaceChildren(head('Recent errors', 'Latest ERROR and FATAL lines, last 24 hours'),
      entries.length
        ? h('div', { class: 'list-rows' }, entries.map((e) => {
          const ms = msOf(e);
          return h('div', {
            class: `list-row err-row lv-${e.level}`, title: `${e.namespace}/${e.pod}`,
            onclick: () => setRoute('workload', {
              ns: e.namespace, wl: e.workload, tab: 'logs', pod: e.pod, range: 'custom',
              from: toLocalInput(ms - 60e3), to: toLocalInput(ms + 60e3),
            }),
          },
            h('span', { class: 'muted' }, fmtTime(ms).slice(0, 8)),
            h('span', null, h('span', { class: 'lv' }, e.level)),
            h('span', { class: 'ellipsis' }, h('span', { class: 'muted' }, `${e.workload}  `), stripAnsi(e.message.split('\n')[0])));
        }))
        : emptyState('check', 'Nothing failing', 'No ERROR or FATAL lines in the last 24 hours.'));
  }

  function renderSystem() {
    const info = data?.storage;
    const agents = store.agents;
    const ok = agents.filter((a) => a.status === 'ok').length;
    const down = agents.filter((a) => a.status !== 'ok');
    const pct = info?.max_bytes ? (info.used_bytes / info.max_bytes) * 100 : 0;
    cards.sys.replaceChildren(
      head('System', 'Agents and storage', h('a', { class: 'btn btn-sm', href: '#/storage' }, 'Storage')),
      h('dl', { class: 'kv' },
        h('dt', null, 'Agents'), h('dd', null, h('span', { class: `badge${agents.length && !down.length ? ' ok' : ' err'}` },
          !agents.length ? 'none discovered' : down.length ? `${ok} of ${agents.length} reachable` : `${ok} reachable`)),
        ...down.slice(0, 3).flatMap((a) => [h('dt', { class: 'error-text' }, 'Unreachable'), h('dd', { class: 'ellipsis', title: a.error }, a.node || a.endpoint)])),
      ...(info
        ? [
          h('div', { class: `meter${pct > 90 ? ' bad' : pct > 70 ? ' warn' : ''}`, title: `${pct.toFixed(1)}% of the disk cap` }, h('div', { style: { width: `${Math.min(100, pct).toFixed(1)}%` } })),
          h('dl', { class: 'kv' },
            h('dt', null, 'Disk used'), h('dd', null, `${fmtBytes(info.used_bytes)} of ${fmtBytes(info.max_bytes)} (${pct.toFixed(1)}%)`),
            h('dt', null, 'Stored lines'), h('dd', null, fmtNum(info.entry_count)),
            h('dt', null, 'Days stored'), h('dd', null, info.oldest_date ? `${info.oldest_date} to ${info.newest_date}` : '-'),
            h('dt', null, 'Retention'), h('dd', null, info.retention === 'unlimited' ? 'until the disk cap' : info.retention)),
        ]
        : [h('p', { class: 'error-text' }, data?.storageError || 'Storage info unavailable')]));
  }

  async function load() {
    const id = ++reqId;
    const ns = store.scope;
    sub.textContent = `${ns ? `Namespace ${ns}` : 'All namespaces'} · last ${days} days, compared with the ${days} days before`;
    renderPeriod();
    if (data) page.classList.add('refetching'); // hold the previous render while reloading
    else skeletons();

    const hourEnd = Math.floor(Date.now() / HOUR) * HOUR + HOUR;
    const [rc, st, recent, hourly] = await Promise.allSettled([
      recap(days * 2, ns),
      api.get('/storage'),
      api.get('/logs', { namespace: ns, level: 'ERROR,FATAL', limit: 8, from: iso(Date.now() - 24 * HOUR) }),
      api.get('/stats/volume', { namespace: ns, from: iso(hourEnd - 24 * HOUR), to: iso(hourEnd), buckets: 24 }),
    ]);
    if (!alive || id !== reqId) return;
    page.classList.remove('refetching');
    charts.forEach((c) => c.destroy());
    charts = [];

    const all = rc.status === 'fulfilled' ? rc.value.days || [] : [];
    const both = perDay(all, days * 2);
    data = {
      series: both.slice(days), prev: both.slice(0, days),
      storage: st.status === 'fulfilled' ? st.value : null, storageError: st.reason?.message,
    };
    renderKpis();
    if (rc.status === 'fulfilled') renderCharts(all);
    else for (const [k, t] of [['vol', 'Log volume'], ['mix', 'Level mix'], ['rate', 'Error rate'], ['top', 'Top error sources'], ['busy', 'Busiest workloads']]) failed(cards[k], t, rc.reason);
    renderHourly(hourly);
    renderRecent(recent);
    renderSystem();
  }

  const offs = [on('catalog', () => { if (data) renderKpis(); }), on('agents', () => { if (data) renderSystem(); }), on('scope', load)];
  return {
    update(params) {
      const d = PERIODS.includes(Number(params.days)) ? Number(params.days) : DEFAULT_DAYS;
      if (d !== days) {
        days = d;
        load();
      }
    },
    refresh: load,
    destroy() {
      alive = false;
      charts.forEach((c) => c.destroy());
      offs.forEach((off) => off());
    },
  };
}

// "+12% vs previous period" style text for card subtitles.
function pctText(delta) {
  if (delta == null) return 'no data for the previous period';
  if (Math.abs(delta) < 0.5) return 'same as the previous period';
  return `${delta > 0 ? '+' : ''}${Math.round(delta)}% vs the previous period`;
}
