// Log explorer: filters, volume histogram, time-ordered list, live tail over
// WebSocket. Also embedded in the workload page with namespace/workload locked.
import { api } from '../api.js';
import { h, icon, fmtTime, fmtNum, localDate, tzLabel, nanoOf, msOf, podColor, copy, toast, menu, emptyState, skeletonRows, debounce } from '../ui.js';
import { renderAnsi, stripAnsi } from '../ansi.js';
import { store, on, patchRoute, LEVELS, namespaces, workloadNames, podsOf, containersOf } from '../state.js';
import { queryParams, rangeControls, searchTerms, highlight, toLocalInput, iso } from '../filters.js';
import { volumeChart } from '../chart.js';
import { openLive } from '../live.js';

const PAGE = 500;
const MAX_ROWS = 5000;

const keyOf = (x) => JSON.stringify([x.ns, x.wl, x.pod, x.c, x.lv, x.q, x.range, x.date, x.from, x.to]);
const podSuffix = (e) => (e.pod.startsWith(e.workload + '-') ? e.pod.slice(e.workload.length + 1) : e.pod);

// opts.lock = {ns, wl} pins the scope (workload page).
export function mount(root, opts = {}) {
  const lock = opts.lock || null;
  let p = {};
  let entries = [];
  let cursor = '';
  let result = null;
  let reqId = 0;
  let loadingMore = false;
  let live = null;
  let liveState = '';
  let chart = null;
  let terms = [];
  const eff = () => (lock ? { ...p, ns: lock.ns, wl: lock.wl } : p);
  const patch = (c) => patchRoute(c);

  // A fixed or past window cannot be tailed, so picking one stops live mode.
  const range = rangeControls((c) => patch(c.range === 'custom' || c.from || c.date ? { ...c, live: '' } : c));
  const select = (label, key, clears) => h('select', {
    class: 'select', 'aria-label': label,
    onchange: (e) => patch({ [key]: e.target.value, ...Object.fromEntries(clears.map((k) => [k, ''])) }),
  });
  const nsSel = select('Namespace', 'ns', ['wl', 'pod', 'c']);
  const wlSel = select('Workload', 'wl', ['pod', 'c']);
  const podSel = select('Pod', 'pod', ['c']);
  const cSel = select('Container', 'c', []);
  const clearBtn = h('button', {
    class: 'btn btn-sm', type: 'button',
    onclick: () => patch({ ...(lock ? {} : { ns: '', wl: '' }), pod: '', c: '', lv: '', q: '' }),
  }, icon('x'), 'Clear');
  const searchIn = h('input', {
    type: 'search', autocomplete: 'off', spellcheck: 'false', 'aria-label': 'Search logs', 'data-log-search': '',
    placeholder: 'Search logs  (press /)',
    title: 'Words are ANDed. "exact phrase", -exclude, a OR b, /regex/, and field filters ns: workload: pod: c: level:',
  });
  const pushQuery = debounce(() => patch({ q: searchIn.value.trim() }), 400);
  searchIn.addEventListener('input', pushQuery);
  searchIn.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') patch({ q: searchIn.value.trim() });
    else if (e.key === 'Escape') searchIn.blur();
  });
  const searchBox = h('label', { class: 'tool-search' }, icon('search'), searchIn);
  const chips = LEVELS.map((l) => h('button', { class: `chip lv-${l}`, type: 'button', title: `Show or hide ${l}`, onclick: () => toggleLevel(l) }, h('i', { class: 'dot' }), l));
  const liveBtn = h('button', { class: 'chip live', type: 'button', onclick: toggleLive });
  const exportBtn = h('button', {
    class: 'btn btn-sm', type: 'button',
    onclick: () => menu(exportBtn, [
      { label: 'Export matching logs' },
      { icon: 'download', text: 'CSV', onClick: () => doExport('csv') },
      { icon: 'download', text: 'JSON', onClick: () => doExport('json') },
    ]),
  }, icon('download'), 'Export');
  const chartBox = h('div', { class: 'histogram' });
  const body = h('div', { class: 'log-body' });
  const status = h('div', { class: 'statusbar' });

  root.append(h('div', { class: 'page-fill' },
    h('div', { class: 'toolbar' },
      searchBox, range.el, h('span', { class: 'sep' }), lock ? null : nsSel, lock ? null : wlSel, podSel, cSel, clearBtn,
      h('span', { class: 'spacer' }), h('span', { class: 'chips' }, chips), liveBtn, exportBtn),
    chartBox,
    h('div', { class: 'log-table' },
      h('div', { class: 'log-head' }, h('span', { title: 'Cluster time zone' }, `Time (${tzLabel()})`), h('span', null, 'Level'), h('span', { class: 'c-src' }, lock ? 'Pod' : 'Source'), h('span', null, 'Message')),
      body),
    status));

  body.addEventListener('scroll', () => {
    if (cursor && !p.live && body.scrollTop + body.clientHeight > body.scrollHeight - 300) loadMore();
  }, { passive: true });

  const offs = [on('catalog', syncControls), on('agents', () => renderStatus())];

  // ── Filters ──
  const levelSet = () => new Set(p.lv ? p.lv.split(',') : LEVELS);
  function toggleLevel(l) {
    const set = levelSet();
    if (set.has(l)) set.delete(l);
    else set.add(l);
    if (!set.size) return;
    patch({ lv: set.size === LEVELS.length ? '' : LEVELS.filter((x) => set.has(x)).join(',') });
  }

  function toggleLive() {
    if (p.live) return patch({ live: '' });
    const pastDay = (!p.range || p.range === 'day') && p.date && p.date !== localDate();
    patch({ live: '1', ...(pastDay || p.range === 'custom' ? { range: '15m', date: '', from: '', to: '' } : {}) });
  }

  function fill(sel, all, options, value) {
    const opts = value && !options.includes(value) ? [value, ...options] : options;
    sel.replaceChildren(h('option', { value: '' }, all), ...opts.map((o) => h('option', { value: o }, o)));
    sel.value = value || '';
    sel.classList.toggle('has-value', !!value);
  }

  function syncControls() {
    const e = eff();
    range.sync(p);
    if (document.activeElement !== searchIn) searchIn.value = p.q || '';
    if (!lock) {
      fill(nsSel, 'All namespaces', namespaces().map((n) => n.name), e.ns);
      fill(wlSel, 'All workloads', workloadNames(e.ns), e.wl);
    }
    fill(podSel, 'All pods', podsOf(e.ns, e.wl), e.pod);
    const containers = containersOf(e.ns, e.wl, e.pod);
    fill(cSel, 'All containers', containers, e.c);
    cSel.hidden = containers.length < 2 && !e.c;
    const set = levelSet();
    chips.forEach((c, i) => c.classList.toggle('active', set.has(LEVELS[i])));
    clearBtn.hidden = !((!lock && (e.ns || e.wl)) || e.pod || e.c || e.lv || e.q);
    liveBtn.classList.toggle('active', !!p.live);
    liveBtn.title = p.live ? 'Stop following new lines' : 'Follow new lines as they arrive (WebSocket)';
    liveBtn.replaceChildren(icon(p.live ? 'pause' : 'play'), 'Live',
      ...(p.live ? [h('i', { class: `live-dot${liveState === 'open' ? '' : ' wait'}`, title: liveState === 'open' ? 'Connected' : 'Connecting' })] : []));
  }

  // ── Loading ──
  async function reload() {
    const id = ++reqId;
    entries = [];
    cursor = '';
    result = null;
    terms = searchTerms(p.q);
    body.replaceChildren(...skeletonRows(14));
    chart?.destroy();
    chart = null;
    chartBox.replaceChildren(h('div', { class: 'skel', style: { height: '94px' } }));
    renderStatus(true);

    const q = queryParams(eff());
    const [logs, vol] = await Promise.allSettled([
      api.get('/logs', { ...q, limit: PAGE }),
      api.get('/stats/volume', { ...q, to: q.to || iso(Date.now()), buckets: 60 }),
    ]);
    if (id !== reqId) return;

    if (logs.status === 'fulfilled') {
      result = logs.value;
      entries = result.entries || [];
      cursor = result.next_cursor || '';
      renderRows();
    } else {
      body.replaceChildren(emptyState('alert', 'Could not load logs', logs.reason.message));
    }
    renderChart(vol);
    renderStatus();
  }

  async function loadMore() {
    if (!cursor || loadingMore) return;
    loadingMore = true;
    renderStatus();
    const id = reqId;
    try {
      const res = await api.get('/logs', { ...queryParams(eff()), limit: PAGE, cursor });
      if (id !== reqId) return;
      const fresh = res.entries || [];
      cursor = res.next_cursor || '';
      result = res;
      if (!entries.length) body.replaceChildren();
      entries = entries.concat(fresh);
      const frag = document.createDocumentFragment();
      fresh.forEach((e) => frag.append(rowEl(e)));
      body.append(frag);
      if (!entries.length) body.replaceChildren(emptyView());
    } catch (e) {
      toast(e.message, 'error');
    } finally {
      loadingMore = false;
      renderStatus();
    }
  }

  // ── Live tail ──
  function startLive() {
    if (live) return;
    const q = { ...queryParams(eff()), from: '', to: '' };
    live = openLive(q, {
      onEntries: addFresh,
      onState: (s) => {
        liveState = s;
        syncControls();
        renderStatus();
        if (s === 'open') catchUp(); // lines written while (re)connecting
      },
    });
  }

  function stopLive() {
    live?.close();
    live = null;
    liveState = '';
  }

  async function catchUp() {
    if (!result) return;
    const params = { ...queryParams(eff()), to: '', sort: 'asc', limit: PAGE };
    if (entries.length) params.cursor = nanoOf(entries[0]).toString();
    try {
      addFresh((await api.get('/logs', params)).entries || []);
    } catch { /* the stream still delivers new lines */ }
  }

  // Prepends lines (given oldest first) that are newer than the newest shown.
  function addFresh(list) {
    if (!result || !p.live) return;
    const newest = entries.length ? nanoOf(entries[0]) : -1n;
    const fresh = list.filter((e) => nanoOf(e) > newest).reverse();
    if (!fresh.length) return;
    if (!entries.length) body.replaceChildren();
    const before = body.scrollHeight;
    const top = body.scrollTop;
    const frag = document.createDocumentFragment();
    fresh.forEach((e) => {
      const r = rowEl(e);
      r.classList.add('fresh');
      frag.append(r);
    });
    body.prepend(frag);
    entries = fresh.concat(entries);
    if (entries.length > MAX_ROWS) {
      const excess = entries.length - MAX_ROWS;
      entries.length = MAX_ROWS;
      for (let i = 0; i < excess; i++) body.lastElementChild?.remove();
      cursor = nanoOf(entries[entries.length - 1]).toString();
    }
    if (top > 0) body.scrollTop = top + (body.scrollHeight - before); // keep the reader's place
    renderStatus();
  }

  async function doExport(format) {
    try {
      toast('Preparing export');
      await api.download('/logs/export', { ...queryParams(eff()), format });
    } catch (e) {
      toast(e.message, 'error');
    }
  }

  // ── Rendering ──
  function renderChart(vol) {
    if (vol.status !== 'fulfilled') {
      chartBox.replaceChildren(h('div', { class: 'vchart-empty error-text' }, vol.reason.message));
      return;
    }
    chart = volumeChart(vol.value, {
      height: 74,
      onSelect: (from, to) => patch({ range: 'custom', date: '', live: '', from: toLocalInput(from), to: toLocalInput(Math.max(to - 1, from)) }),
    });
    chartBox.replaceChildren(chart.el);
  }

  function renderRows() {
    if (!entries.length) {
      body.replaceChildren(emptyView());
      return;
    }
    const frag = document.createDocumentFragment();
    entries.forEach((e) => frag.append(rowEl(e)));
    body.replaceChildren(frag);
    body.scrollTop = 0;
  }

  function emptyView() {
    const errs = result?.errors || [];
    if (errs.includes('no agents discovered')) {
      return emptyState('server', 'No agents discovered',
        'The aggregator has not found any Kapture agent. Check that the agent DaemonSet is running and the kapture-agents headless service exists.');
    }
    if (!store.catalog.length) {
      return emptyState('inbox', 'No logs collected yet', errs.length
        ? `Agents could not be queried: ${errs.join('; ')}`
        : 'Agents are running but have not stored any container logs yet. Check that /var/log is mounted into the agent pods.');
    }
    if (result?.partial && cursor) {
      return emptyState('search', 'Still searching', 'The scan limit was reached before anything matched. Continue scanning older logs.',
        h('button', { class: 'btn btn-sm', onclick: loadMore }, 'Continue'));
    }
    if (p.live) return emptyState('activity', 'Waiting for new lines', 'Nothing matched yet. New lines appear here as soon as they are written.');
    return emptyState('inbox', 'No logs match', 'Nothing in this time range matches the current filters.',
      h('span', { class: 'input-group' },
        h('button', { class: 'btn btn-sm', onclick: () => patch({ range: '24h', date: '', from: '', to: '' }) }, 'Last 24 hours'),
        h('button', { class: 'btn btn-sm', onclick: () => patch({ range: '', date: '', from: '', to: '', pod: '', c: '', lv: '', q: '' }) }, 'Reset filters')));
  }

  function rowEl(e) {
    const ms = msOf(e);
    const [first, ...rest] = e.message.split('\n');
    const row = h('div', { class: `row lv-${e.level}`, onclick: () => toggleDetail(row, e) },
      h('span', { class: 'c-time', title: e.timestamp }, fmtTime(ms)),
      h('span', null, h('span', { class: 'lv' }, e.level)),
      h('span', { class: 'c-src', title: `${e.namespace}/${e.pod} (${e.container}) on ${e.node}` },
        h('i', { class: 'pod-dot', style: { background: podColor(e.pod) } }),
        lock ? null : h('span', { class: 'src-wl' }, e.workload),
        h('span', { class: 'src-pod' }, podSuffix(e))),
      h('span', { class: 'c-msg' }, renderAnsi(first, (t) => highlight(t, terms)), rest.length ? h('span', { class: 'more' }, `+${rest.length} lines`) : null));
    return row;
  }

  function toggleDetail(row, e) {
    if (getSelection()?.toString()) return; // selecting text, not opening
    const open = row.nextElementSibling?.classList.contains('detail');
    body.querySelectorAll('.detail').forEach((d) => {
      d.previousElementSibling?.classList.remove('open');
      d.remove();
    });
    if (open) return;
    row.classList.add('open');
    row.after(detailEl(e));
  }

  function detailEl(e) {
    const ms = msOf(e);
    const meta = [
      ['Timestamp', e.timestamp], ['Namespace', e.namespace], ['Workload', `${e.workload} (${e.workload_type})`],
      ['Pod', e.pod], ['Container', e.container], ['Node', e.node], ['Stream', e.stream], ['Level', e.level],
    ];
    const action = (ic, text, fn) => h('button', { class: 'btn btn-sm', type: 'button', onclick: fn }, icon(ic), text);
    return h('div', { class: 'detail' },
      h('div', { class: 'detail-meta' }, meta.map(([k, v]) => h('div', null, h('span', { class: 'k' }, k), h('span', { class: 'v', title: v }, v || '-')))),
      h('pre', { class: 'detail-msg' }, renderAnsi(e.message, (t) => highlight(t, terms))),
      h('div', { class: 'detail-actions' },
        action('copy', 'Copy message', () => copy(stripAnsi(e.message))),
        action('copy', 'Copy JSON', () => copy(JSON.stringify(e, null, 2))),
        action('box', 'Only this pod', () => patch({ ...(lock ? {} : { ns: e.namespace, wl: e.workload }), pod: e.pod, c: '' })),
        action('crosshair', 'Surrounding lines', () => patch({
          ...(lock ? {} : { ns: e.namespace, wl: e.workload }), pod: e.pod, c: '', q: '', lv: '', live: '', date: '',
          range: 'custom', from: toLocalInput(ms - 60e3), to: toLocalInput(ms + 60e3),
        }))));
  }

  function renderStatus(loading) {
    const ok = store.agents.filter((a) => a.status === 'ok').length;
    const errs = result?.errors || [];
    const liveText = { open: 'Live: streaming', connecting: 'Live: connecting', retrying: 'Live: reconnecting' }[liveState];
    status.replaceChildren(...[
      h('span', null, loading ? 'Loading' : `${fmtNum(entries.length)} lines, newest first`),
      h('span', null, `${ok}/${store.agents.length} agents`),
      p.live && liveText ? h('span', { class: liveState === 'open' ? '' : 'warn' }, liveText) : null,
      errs.length ? h('span', { class: 'warn ellipsis', title: errs.join('\n') }, icon('alert'), errs.length === 1 ? errs[0] : `${errs.length} agent errors`) : null,
      result?.partial ? h('span', { class: 'warn' }, icon('alert'), 'Scan limit reached') : null,
      h('span', { class: 'spacer' }),
      cursor && !loading ? h('button', { class: 'btn btn-sm', type: 'button', disabled: loadingMore, onclick: loadMore }, loadingMore ? 'Loading' : 'Load older') : null,
    ].filter(Boolean));
  }

  return {
    update(params) {
      const prev = keyOf(eff());
      p = params;
      syncControls();
      const changed = keyOf(eff()) !== prev || !result;
      if (!p.live) stopLive();
      if (changed) {
        stopLive();
        const loading = reload();
        if (p.live) loading.then(startLive);
      } else if (p.live) {
        startLive();
      }
    },
    refresh: () => reload().then(() => { if (p.live) { stopLive(); startLive(); } }),
    destroy() {
      stopLive();
      chart?.destroy();
      offs.forEach((off) => off());
    },
  };
}
