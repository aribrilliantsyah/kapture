// The signed-in user's own account: name, password, authenticator, recovery.
import { api } from '../api.js';
import { h, icon, toast, copy, formDialog, emptyState, fmtAgo, passwordProblem, passwordRules } from '../ui.js';
import { store, setUser } from '../state.js';

export function mount(root) {
  let alive = true;
  let hideTimer = null;
  const el = h('div', { class: 'page' });
  root.append(el);
  const head = h('div', { class: 'page-head' },
    h('div', null, h('h1', null, 'Profile'), h('p', null, 'Your account, password and two-factor sign-in')));

  const card = (title, sub, ...content) => h('div', { class: 'card' },
    h('div', { class: 'card-head' }, h('div', null, h('h2', null, title), h('p', { class: 'card-sub' }, sub))), ...content);
  const input = (attrs) => h('input', { class: 'input', spellcheck: 'false', ...attrs });
  const labeled = (label, control, cls = '') => h('label', { class: `field ${cls}` }, h('span', null, label), control);
  const staticField = (label, value) => h('div', { class: 'field' }, h('span', null, label), h('div', { class: 'static' }, value));
  const actions = (...btns) => h('div', { class: 'form-actions' }, ...btns);

  async function save(btn, fn) {
    btn.disabled = true;
    try { await fn(); } catch (e) { toast(e.message, 'error'); } finally { btn.disabled = false; }
  }

  async function load() {
    if (!store.authEnabled) {
      el.replaceChildren(head, emptyState('lock', 'Authentication is disabled', 'Set KAPTURE_AUTH_ENABLED=true to use accounts and two-factor sign-in.'));
      return;
    }
    let data;
    try {
      data = await api.get('/profile');
    } catch (e) {
      if (alive) el.replaceChildren(head, emptyState('alert', 'Could not load your profile', e.message));
      return;
    }
    if (alive) render(data.user, data.questions || []);
  }

  function accountCard(u) {
    const name = input({ value: u.display_name || '', placeholder: u.username, autocomplete: 'name', maxlength: 64 });
    const uname = input({ value: u.username, autocomplete: 'username', required: true });
    const btn = h('button', { class: 'btn btn-primary', type: 'submit' }, 'Save');
    const form = h('form', null,
      h('div', { class: 'form-grid' },
        labeled('Display name', name), labeled('Username', uname),
        staticField('Role', u.role === 'admin' ? 'Administrator' : 'Operator'),
        staticField('Last sign-in', u.last_login_at ? fmtAgo(Date.parse(u.last_login_at)) : '-')),
      actions(btn));
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      save(btn, async () => {
        const nu = await api.patch('/profile', { display_name: name.value, username: uname.value });
        setUser(nu);
        toast('Profile saved');
      });
    });
    return card('Account', 'How you appear and sign in', form);
  }

  function passwordCard() {
    const cur = input({ type: 'password', autocomplete: 'current-password', required: true });
    const p1 = input({ type: 'password', autocomplete: 'new-password', minlength: 8, required: true, placeholder: 'e.g. Qawsed#1477' });
    const p2 = input({ type: 'password', autocomplete: 'new-password', minlength: 8, required: true });
    const btn = h('button', { class: 'btn btn-primary', type: 'submit' }, 'Change password');
    const form = h('form', null,
      h('div', { class: 'form-grid' }, labeled('Current password', cur, 'full'), labeled('New password', p1), labeled('Confirm new password', p2),
        h('div', { class: 'full' }, passwordRules(p1))),
      actions(btn));
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      const problem = passwordProblem(p1.value);
      if (problem) return toast(problem, 'error');
      if (p1.value !== p2.value) return toast('The new passwords do not match', 'error');
      save(btn, async () => {
        await api.post('/profile/password', { current: cur.value, password: p1.value });
        form.reset();
        toast('Password changed, your other sessions were signed out');
      });
    });
    return card('Password', 'Changing it signs out your other sessions', form);
  }

  function twoFactorCard(u) {
    const body = h('div');
    const showBtn = h('button', { class: 'btn', type: 'button' }, icon('shield'), 'Show QR code');
    const hide = () => {
      clearTimeout(hideTimer);
      body.replaceChildren(h('div', { class: 'tf-row' },
        u.has_2fa ? h('span', { class: 'badge ok' }, 'Enabled') : h('span', { class: 'badge err' }, 'Not set up'),
        h('span', { class: 'muted' }, 'Codes come from your authenticator app'),
        h('span', { class: 'spacer' }), u.has_2fa ? showBtn : null));
    };
    const show = (res) => {
      body.replaceChildren(
        h('div', { class: 'qr' },
          h('img', { src: res.qr_data_url, alt: 'QR code for your authenticator app' }),
          h('div', null,
            h('div', { class: 'muted' }, 'Scan it on your new phone, or enter the key:'),
            h('code', null, res.secret),
            h('span', { class: 'input-group' },
              h('button', { class: 'btn btn-sm', type: 'button', onclick: () => copy(res.secret) }, icon('copy'), 'Copy key'),
              h('button', { class: 'btn btn-sm', type: 'button', onclick: hide }, icon('x'), 'Hide')))),
        h('p', { class: 'note', style: { marginTop: '10px' } }, icon('help'),
          'Both phones show the same codes. Remove Kapture from the old phone once the new one works. Hidden again in 2 minutes.'));
      clearTimeout(hideTimer);
      hideTimer = setTimeout(hide, 120e3);
    };
    showBtn.addEventListener('click', async () => {
      const res = await formDialog({
        title: 'Show your authenticator key',
        body: 'Enter your password to show the QR code for moving two-factor sign-in to another phone.',
        fields: [{ name: 'password', label: 'Password', type: 'password', autocomplete: 'current-password' }],
        submitText: 'Show QR code',
        onSubmit: (v) => api.post('/profile/2fa', v),
      });
      if (res && res.secret) show(res);
    });
    hide();
    return card('Two-factor sign-in', 'Show the QR code again to move it to a new phone', body);
  }

  function recoveryCard(u, questions) {
    if (u.role !== 'admin') {
      return card('Account recovery', 'Lost your password or phone?',
        h('p', { class: 'note' }, icon('users'), 'An administrator can reset your password or two-factor sign-in from the Users page.'));
    }
    const q = h('select', { class: 'select wide', required: true },
      h('option', { value: '' }, 'Choose a question'), questions.map((x) => h('option', { value: x.id }, x.text)));
    q.value = u.recovery_question || '';
    const ans = input({ autocomplete: 'off', required: true, placeholder: u.has_recovery ? 'Type the answer again to change it' : 'Not case sensitive' });
    const pw = input({ type: 'password', autocomplete: 'current-password', required: true });
    const btn = h('button', { class: 'btn btn-primary', type: 'submit' }, 'Save recovery question');
    const form = h('form', null,
      u.has_recovery
        ? h('p', { class: 'note' }, icon('check'), 'Set. On the sign-in page, "Forgot your password" asks it plus your 2FA code or your password.')
        : h('p', { class: 'note warn' }, icon('alert'), 'Not set. Without it only another administrator can reset a lost password or phone.'),
      h('div', { class: 'form-grid mt-sm' }, labeled('Question', q, 'full'), labeled('Answer', ans), labeled('Current password', pw)),
      actions(btn));
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      save(btn, async () => {
        await api.put('/profile/recovery', { question: q.value, answer: ans.value, password: pw.value });
        toast('Recovery question saved');
        load();
      });
    });
    return card('Account recovery', 'Recover a lost password or phone yourself', form);
  }

  function render(u, questions) {
    el.replaceChildren(head,
      h('div', { class: 'grid grid-2e' }, accountCard(u), passwordCard()),
      h('div', { class: 'grid grid-2e mt' }, twoFactorCard(u), recoveryCard(u, questions)));
  }

  load();
  return { refresh: load, destroy() { alive = false; clearTimeout(hideTimer); } };
}
