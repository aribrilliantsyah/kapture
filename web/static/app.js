// k8s-log-catcher Dashboard — Vanilla JS, no dependencies
(function() {
  'use strict';

  // ── State ──
  const state = {
    namespace: '',
    workload: '',
    pod: '',
    date: '',
    search: '',
    levels: ['DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL'],
    logs: [],
    cursor: '',
    loading: false,
    autoScroll: true,
  };

  // ── Init ──
  document.addEventListener('DOMContentLoaded', async () => {
    // Set date to today
    const today = new Date().toISOString().slice(0, 10);
    document.getElementById('f-date').value = today;
    state.date = today;

    const authed = await checkAuth();
    if (authed) {
      loadNamespaces();
      loadWorkloads();
      applyFilters();
    }

    // Auto-refresh every 5 seconds
    setInterval(() => {
      const isModalOpen = document.getElementById('login-modal').style.display === 'flex' ||
                          document.getElementById('setup-modal').style.display === 'flex';
      if (state.autoScroll && !isModalOpen) {
        applyFilters();
      }
    }, 5000);
  });

  // ── Auth & 2FA Management ──
  let currentSetupSecret = '';
  let tempLoginToken = '';
  let tempLoginUsername = '';

  function getAuthHeader() {
    const token = localStorage.getItem('kapture_token');
    if (token) {
      return { 'Authorization': 'Bearer ' + token };
    }
    return {};
  }

  async function checkAuth() {
    try {
      const resp = await fetch('/api/v1/auth/status', {
        headers: getAuthHeader()
      });
      const data = await resp.json();
      if (!data.auth_enabled) {
        document.getElementById('setup-modal').style.display = 'none';
        document.getElementById('login-modal').style.display = 'none';
        return true;
      }
      if (data.setup_needed) {
        document.getElementById('login-modal').style.display = 'none';
        await initSetup();
        return false;
      }
      if (data.authenticated) {
        document.getElementById('setup-modal').style.display = 'none';
        document.getElementById('login-modal').style.display = 'none';
        document.getElementById('user-badge').textContent = '👤 ' + (data.username || 'admin');
        document.getElementById('logout-btn').style.display = 'inline-flex';
        return true;
      }
      // Setup is done, but user is not logged in
      document.getElementById('setup-modal').style.display = 'none';
      document.getElementById('login-modal').style.display = 'flex';
      return false;
    } catch(e) {
      console.error('Auth check error:', e);
      return false;
    }
  }

  async function initSetup() {
    try {
      const resp = await fetch('/api/v1/auth/setup/init', { method: 'POST' });
      const data = await resp.json();
      if (data.secret) {
        currentSetupSecret = data.secret;
        document.getElementById('setup-qr-img').src = data.qr_data_url;
        document.getElementById('setup-secret-text').textContent = data.secret;
        document.getElementById('setup-modal').style.display = 'flex';
      }
    } catch(e) {
      console.error('Setup init error:', e);
    }
  }

  window.copySetupSecret = function() {
    if (currentSetupSecret) {
      copyText(currentSetupSecret);
      setStatus('Kunci 2FA berhasil disalin');
    }
  };

  window.handleSetupComplete = async function(e) {
    e.preventDefault();
    const u = document.getElementById('setup-username').value.trim();
    const p = document.getElementById('setup-password').value;
    const pConfirm = document.getElementById('setup-password-confirm').value;
    const code = document.getElementById('setup-otp').value.trim();
    const errorEl = document.getElementById('setup-error');
    errorEl.style.display = 'none';

    if (p !== pConfirm) {
      errorEl.textContent = 'Password dan konfirmasi password tidak cocok!';
      errorEl.style.display = 'block';
      return;
    }

    try {
      const resp = await fetch('/api/v1/auth/setup/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: u,
          password: p,
          secret: currentSetupSecret,
          code: code
        })
      });
      const data = await resp.json();
      if (data.success && data.token) {
        localStorage.setItem('kapture_token', data.token);
        document.getElementById('setup-modal').style.display = 'none';
        document.getElementById('user-badge').textContent = '👤 ' + data.username;
        document.getElementById('logout-btn').style.display = 'inline-flex';
        loadNamespaces();
        loadWorkloads();
        applyFilters();
      } else {
        errorEl.textContent = data.error || 'Verifikasi 2FA gagal. Periksa kode di aplikasi authenticator!';
        errorEl.style.display = 'block';
      }
    } catch(err) {
      errorEl.textContent = 'Gagal menghubungi server';
      errorEl.style.display = 'block';
    }
  };

  // Langkah 1 Login: Cek Username dan Password terlebih dahulu
  window.handleLoginStep1 = async function(e) {
    e.preventDefault();
    const u = document.getElementById('login-username').value.trim();
    const p = document.getElementById('login-password').value;
    const errorEl = document.getElementById('login-error-step1');
    const btn = document.getElementById('login-btn-step1');
    errorEl.style.display = 'none';

    btn.disabled = true;
    btn.textContent = 'Memeriksa...';

    try {
      const resp = await fetch('/api/v1/auth/login/credentials', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: u, password: p })
      });
      const data = await resp.json();
      btn.disabled = false;
      btn.textContent = 'Lanjutkan →';

      if (data.require_2fa && data.temp_token) {
        tempLoginToken = data.temp_token;
        tempLoginUsername = data.username || u;

        // Tampilkan Langkah 2 (2FA OTP)
        document.getElementById('login-form-step1').style.display = 'none';
        document.getElementById('login-form-step2').style.display = 'flex';
        document.getElementById('login-title').textContent = 'Verifikasi 2FA';
        document.getElementById('login-subtitle').textContent = 'Masukkan 6-digit kode dari Google Authenticator';
        document.getElementById('login-user-confirmed').textContent = tempLoginUsername;

        const otpInput = document.getElementById('login-otp');
        otpInput.value = '';
        setTimeout(() => otpInput.focus(), 100);
      } else if (data.token) {
        // Auth is disabled or token granted directly
        finishLogin(data.token, data.username || u);
      } else {
        errorEl.textContent = data.error || 'Username atau password salah!';
        errorEl.style.display = 'block';
      }
    } catch(err) {
      btn.disabled = false;
      btn.textContent = 'Lanjutkan →';
      errorEl.textContent = 'Gagal menghubungi server';
      errorEl.style.display = 'block';
    }
  };

  // Langkah 2 Login: Setelah username/password lolos, masukkan kode 2FA
  window.handleLoginStep2 = async function(e) {
    e.preventDefault();
    const code = document.getElementById('login-otp').value.trim();
    const errorEl = document.getElementById('login-error-step2');
    const btn = document.getElementById('login-btn-step2');
    errorEl.style.display = 'none';

    if (!tempLoginToken) {
      backToLoginStep1();
      return;
    }

    btn.disabled = true;
    btn.textContent = 'Memverifikasi 2FA...';

    try {
      const resp = await fetch('/api/v1/auth/login/2fa', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ temp_token: tempLoginToken, code: code })
      });
      const data = await resp.json();
      btn.disabled = false;
      btn.textContent = 'Verifikasi & Masuk';

      if (data.success && data.token) {
        finishLogin(data.token, tempLoginUsername);
      } else {
        errorEl.textContent = data.error || 'Kode 2FA salah atau kedaluwarsa!';
        errorEl.style.display = 'block';
      }
    } catch(err) {
      btn.disabled = false;
      btn.textContent = 'Verifikasi & Masuk';
      errorEl.textContent = 'Gagal memverifikasi 2FA ke server';
      errorEl.style.display = 'block';
    }
  };

  window.backToLoginStep1 = function() {
    tempLoginToken = '';
    document.getElementById('login-form-step2').style.display = 'none';
    document.getElementById('login-form-step1').style.display = 'flex';
    document.getElementById('login-title').textContent = 'Masuk ke Kapture';
    document.getElementById('login-subtitle').textContent = 'Kubernetes Log Catcher Dashboard';
    document.getElementById('login-error-step1').style.display = 'none';
    document.getElementById('login-error-step2').style.display = 'none';
    document.getElementById('login-password').value = '';
    document.getElementById('login-password').focus();
  };

  function finishLogin(token, username) {
    localStorage.setItem('kapture_token', token);
    document.getElementById('login-modal').style.display = 'none';
    document.getElementById('setup-modal').style.display = 'none';
    document.getElementById('user-badge').textContent = '👤 ' + username;
    document.getElementById('logout-btn').style.display = 'inline-flex';
    backToLoginStep1();
    loadNamespaces();
    loadWorkloads();
    applyFilters();
  }

  window.logout = async function() {
    try {
      await fetch('/api/v1/auth/logout', {
        method: 'POST',
        headers: getAuthHeader()
      });
    } catch(e) {}
    localStorage.removeItem('kapture_token');
    document.getElementById('user-badge').textContent = '';
    document.getElementById('logout-btn').style.display = 'none';
    backToLoginStep1();
    document.getElementById('login-modal').style.display = 'flex';
  };

  // ── API Calls ──
  async function api(path) {
    try {
      const resp = await fetch('/api/v1' + path, {
        headers: getAuthHeader()
      });
      if (resp.status === 401) {
        await checkAuth();
        return null;
      }
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      return await resp.json();
    } catch(e) {
      console.error('API error:', path, e);
      return null;
    }
  }

  async function apiDelete(path) {
    try {
      const resp = await fetch('/api/v1' + path, {
        method: 'DELETE',
        headers: getAuthHeader()
      });
      if (resp.status === 401) {
        await checkAuth();
        return null;
      }
      return await resp.json();
    } catch(e) {
      console.error('API delete error:', path, e);
      return null;
    }
  }

  // ── Namespaces ──
  async function loadNamespaces() {
    const ns = await api('/namespaces');
    if (!ns) return;
    const list = document.getElementById('ns-list');
    list.innerHTML = '<div class="sidebar-item active" data-ns="" onclick="filterNS(\'\')">● all</div>';
    ns.forEach(n => {
      list.innerHTML += `<div class="sidebar-item" data-ns="${n}" onclick="filterNS('${n}')">○ ${n}</div>`;
    });
  }

  // ── Workloads ──
  async function loadWorkloads() {
    const path = state.namespace ? `/workloads?namespace=${encodeURIComponent(state.namespace)}` : '/workloads';
    const wl = await api(path);
    if (!wl) return;
    const list = document.getElementById('wl-list');
    list.innerHTML = '';
    wl.forEach(w => {
      list.innerHTML += `<div class="sidebar-item sidebar-workload" onclick="filterWorkload('${w}')">▶ ${w}</div>`;
    });
  }

  // ── Pods dropdown ──
  async function loadPods() {
    let path = '/pods?';
    if (state.namespace) path += `namespace=${encodeURIComponent(state.namespace)}&`;
    if (state.workload) path += `workload=${encodeURIComponent(state.workload)}&`;
    const pods = await api(path);
    const sel = document.getElementById('f-pod');
    sel.innerHTML = '<option value="">All</option>';
    if (pods) {
      pods.forEach(p => {
        sel.innerHTML += `<option value="${p}">${p}</option>`;
      });
    }
  }

  // ── Filters ──
  window.filterNS = function(ns) {
    state.namespace = ns;
    // Update sidebar active state
    document.querySelectorAll('#ns-list .sidebar-item').forEach(el => {
      el.classList.toggle('active', el.dataset.ns === ns);
      el.textContent = el.textContent.replace(/^[●○]/, el.dataset.ns === ns ? '●' : '○');
    });
    loadWorkloads();
    loadPods();
    applyFilters();
  };

  window.filterWorkload = function(wl) {
    state.workload = state.workload === wl ? '' : wl;
    document.querySelectorAll('#wl-list .sidebar-item').forEach(el => {
      const name = el.textContent.replace(/^[▶▼]\s*/, '');
      const isActive = name === state.workload;
      el.classList.toggle('active', isActive);
      el.textContent = (isActive ? '▼ ' : '▶ ') + name;
    });
    loadPods();
    applyFilters();
  };

  window.toggleLevel = function(btn) {
    btn.classList.toggle('active');
    state.levels = [];
    document.querySelectorAll('.level-btn.active').forEach(b => {
      state.levels.push(b.dataset.level);
    });
    applyFilters();
  };

  let searchTimer;
  window.debounceSearch = function() {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      state.search = document.getElementById('f-search').value;
      applyFilters();
    }, 300);
  };

  window.applyFilters = async function() {
    state.date = document.getElementById('f-date').value;
    state.pod = document.getElementById('f-pod').value;

    let path = '/logs?limit=500&sort=desc';
    if (state.date) path += `&date=${state.date}`;
    if (state.namespace) path += `&namespace=${encodeURIComponent(state.namespace)}`;
    if (state.workload) path += `&workload=${encodeURIComponent(state.workload)}`;
    if (state.pod) path += `&pod=${encodeURIComponent(state.pod)}`;
    if (state.search) path += `&search=${encodeURIComponent(state.search)}`;
    if (state.levels.length > 0 && state.levels.length < 5) {
      path += `&level=${state.levels.join(',')}`;
    }

    setStatus('Loading...');
    const result = await api(path);
    if (!result) {
      setStatus('Error loading logs');
      return;
    }

    state.logs = result.entries || [];
    state.cursor = result.next_cursor || '';
    renderLogs();
    setStatus(`Loaded ${state.logs.length} logs`);
    document.getElementById('log-count').textContent = state.logs.length;
  };

  // ── Render Logs ──
  function renderLogs() {
    const viewer = document.getElementById('log-viewer');
    const empty = document.getElementById('empty-state');

    if (state.logs.length === 0) {
      viewer.innerHTML = '';
      viewer.appendChild(empty);
      empty.style.display = 'flex';
      return;
    }

    empty.style.display = 'none';
    const frag = document.createDocumentFragment();

    state.logs.forEach((log, idx) => {
      const line = document.createElement('div');
      line.className = 'log-line';
      line.onclick = () => showDetail(log);

      const ts = new Date(log.timestamp).toLocaleTimeString('en-GB', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });

      line.innerHTML = `
        <span class="log-time">${ts}</span>
        <span class="log-level ${log.level}">${pad(log.level, 5)}</span>
        <span class="log-source"><span class="log-workload">${log.workload}</span> <span class="log-pod">[${shortPod(log.pod)}]</span></span>
        <span class="log-msg">${escapeHtml(log.message)}</span>
      `;

      frag.appendChild(line);
    });

    viewer.innerHTML = '';
    viewer.appendChild(frag);
  }

  // ── Log Detail ──
  let currentDetailLog = null;

  window.showDetail = function(log) {
    currentDetailLog = log;
    const content = document.getElementById('detail-content');
    const ts = new Date(log.timestamp);

    content.innerHTML = `
      <div class="detail-row"><span class="detail-label">Timestamp</span><span class="detail-value">${ts.toISOString()}</span></div>
      <div class="detail-row"><span class="detail-label">Date</span><span class="detail-value">${log.date}</span></div>
      <div class="detail-row"><span class="detail-label">Namespace</span><span class="detail-value">${log.namespace}</span></div>
      <div class="detail-row"><span class="detail-label">Workload</span><span class="detail-value">${log.workload} (${log.workload_type})</span></div>
      <div class="detail-row"><span class="detail-label">Pod</span><span class="detail-value">${log.pod}</span></div>
      <div class="detail-row"><span class="detail-label">Container</span><span class="detail-value">${log.container}</span></div>
      <div class="detail-row"><span class="detail-label">Node</span><span class="detail-value">${log.node}</span></div>
      <div class="detail-row"><span class="detail-label">Stream</span><span class="detail-value">${log.stream}</span></div>
      <div class="detail-row"><span class="detail-label">Level</span><span class="detail-value log-level ${log.level}">${log.level}</span></div>
      <div class="detail-message">${escapeHtml(log.message)}</div>
      <div style="margin-top:12px;display:flex;gap:8px">
        <button class="btn btn-sm" onclick="copyDetailMessage()">📋 Copy Message</button>
        <button class="btn btn-sm" onclick="copyDetailJSON()">📋 Copy JSON</button>
      </div>
    `;
    document.getElementById('detail-modal').classList.add('active');
  };

  window.copyDetailMessage = function() {
    if (currentDetailLog && currentDetailLog.message) {
      copyText(currentDetailLog.message);
    }
  };

  window.copyDetailJSON = function() {
    if (currentDetailLog) {
      copyText(JSON.stringify(currentDetailLog, null, 2));
    }
  };

  window.closeModal = function() {
    document.getElementById('detail-modal').classList.remove('active');
  };

  // Close modal on escape
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') closeModal();
  });

  // ── Tabs ──
  window.showTab = function(tab) {
    document.getElementById('tab-logs').style.display = tab === 'logs' ? '' : 'none';
    document.getElementById('tab-storage').style.display = tab === 'storage' ? '' : 'none';
    if (tab === 'storage') loadStorage();
  };

  // ── Storage ──
  async function loadStorage() {
    const info = await api('/storage');
    if (!info) return;

    const pct = info.max_bytes > 0 ? (info.used_bytes / info.max_bytes * 100) : 0;
    document.getElementById('storage-bar-fill').style.width = pct.toFixed(1) + '%';
    document.getElementById('st-used').textContent = formatBytes(info.used_bytes);
    document.getElementById('st-limit').textContent = formatBytes(info.max_bytes);
    document.getElementById('st-entries').textContent = formatNumber(info.entry_count);
    document.getElementById('st-range').textContent = info.oldest_date && info.newest_date
      ? `${info.oldest_date} → ${info.newest_date}` : '-';

    const dateList = document.getElementById('date-list');
    dateList.innerHTML = '';
    if (info.dates) {
      info.dates.forEach(d => {
        dateList.innerHTML += `
          <div class="date-item">
            <div class="date-info">
              <span class="date-name">📅 ${d.date}</span>
              <span class="date-count">${formatNumber(d.entry_count)} entries</span>
              <span class="date-size">${formatBytes(d.size_bytes)}</span>
            </div>
            <button class="btn btn-danger btn-sm" onclick="deleteDate('${d.date}')">🗑</button>
          </div>
        `;
      });
    }
  }

  window.deleteBeforeDate = async function() {
    const date = document.getElementById('delete-before-date').value;
    if (!date) return alert('Select a date first');
    if (!confirm(`Delete all logs before ${date}?`)) return;
    const result = await apiDelete(`/logs?before=${date}`);
    if (result) alert(`Deleted ${result.deleted || 0} entries`);
    loadStorage();
  };

  window.deleteDate = async function(date) {
    if (!confirm(`Delete all logs for ${date}?`)) return;
    // Delete before next day to delete the specific date
    const next = nextDay(date);
    const result = await apiDelete(`/logs?before=${next}`);
    if (result) alert(`Deleted entries`);
    loadStorage();
  };

  window.resetAll = async function() {
    if (!confirm('⚠️ This will DELETE ALL logs from all nodes. Are you sure?')) return;
    if (!confirm('⚠️ FINAL WARNING: This cannot be undone. Proceed?')) return;
    const result = await apiDelete('/logs/all');
    if (result) alert('All logs deleted');
    loadStorage();
    applyFilters();
  };

  // ── Export ──
  window.exportLogs = function(format) {
    if (state.logs.length === 0) return alert('No logs to export');

    let content, filename, mime;

    if (format === 'csv') {
      const headers = 'timestamp,date,namespace,workload,pod,container,node,stream,level,message\n';
      const rows = state.logs.map(l =>
        `"${l.timestamp}","${l.date}","${l.namespace}","${l.workload}","${l.pod}","${l.container}","${l.node}","${l.stream}","${l.level}","${csvEscape(l.message)}"`
      ).join('\n');
      content = headers + rows;
      filename = `logs-${state.date || 'export'}.csv`;
      mime = 'text/csv';
    } else {
      content = JSON.stringify(state.logs, null, 2);
      filename = `logs-${state.date || 'export'}.json`;
      mime = 'application/json';
    }

    const blob = new Blob([content], { type: mime });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  };

  // ── Helpers ──
  window.refreshAll = function() {
    loadNamespaces();
    loadWorkloads();
    loadPods();
    applyFilters();
  };

  window.copyText = function(text) {
    navigator.clipboard.writeText(text).then(
      () => setStatus('Copied to clipboard'),
      () => setStatus('Copy failed')
    );
  };

  function setStatus(text) {
    document.getElementById('status-text').textContent = text;
  }

  function shortPod(pod) {
    if (pod.length <= 20) return pod;
    return pod.slice(0, 17) + '...';
  }

  function pad(s, n) {
    s = s || '';
    while (s.length < n) s += ' ';
    return s;
  }

  function escapeHtml(s) {
    return (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function escapeAttr(s) {
    return (s || '').replace(/'/g, "\\'").replace(/\n/g, '\\n');
  }

  function csvEscape(s) {
    return (s || '').replace(/"/g, '""');
  }

  function formatBytes(b) {
    if (b === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(b) / Math.log(k));
    return parseFloat((b / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatNumber(n) {
    return (n || 0).toLocaleString();
  }

  function nextDay(date) {
    const d = new Date(date + 'T00:00:00Z');
    d.setUTCDate(d.getUTCDate() + 1);
    return d.toISOString().slice(0, 10);
  }
})();
