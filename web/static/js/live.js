// Live tail over WebSocket (/api/v1/tail), reconnecting with backoff.
import { qs, redirectToLogin } from './api.js';

// openLive streams entries matching params. onEntries receives batches oldest
// first; onState gets 'connecting' | 'open' | 'retrying'.
export function openLive(params, { onEntries, onState }) {
  let ws = null;
  let closed = false;
  let attempt = 0;
  let timer = null;
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';

  function connect() {
    onState(attempt ? 'retrying' : 'connecting');
    ws = new WebSocket(`${proto}://${location.host}/api/v1/tail${qs(params)}`);
    ws.onopen = () => {
      attempt = 0;
      onState('open');
    };
    ws.onmessage = (ev) => {
      try {
        const m = JSON.parse(ev.data);
        if (m.type === 'entries' && m.entries?.length) onEntries(m.entries);
      } catch { /* ignore malformed frames */ }
    };
    ws.onclose = async () => {
      if (closed) return;
      onState('retrying');
      // A failed upgrade gives no status code; ask whether the session ended.
      try {
        const st = await (await fetch('/api/v1/auth/status')).json();
        if (st.auth_enabled && !st.authenticated) return redirectToLogin();
      } catch { /* server down: keep retrying */ }
      timer = setTimeout(connect, Math.min(15000, 500 * 2 ** attempt++));
    };
  }

  connect();
  return {
    close() {
      closed = true;
      clearTimeout(timer);
      ws?.close();
    },
  };
}
