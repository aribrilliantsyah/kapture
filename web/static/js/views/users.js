// User management (administrators): accounts, roles, password and 2FA resets.
import { api } from '../api.js';
import { h, icon, fmtAgo, toast, menu, confirmDialog, formDialog, randomPassword, copy, emptyState, passwordProblem, PASSWORD_HINT } from '../ui.js';
import { store, setUser } from '../state.js';

const ROLES = [['operator', 'Operator: everything except user management'], ['admin', 'Administrator: also manages users']];
const roleBadge = (r) => h('span', { class: `badge${r === 'admin' ? ' role-admin' : ''}` }, r === 'admin' ? 'Admin' : 'Operator');

// Shows sign-in details to hand over to a user.
function shareDetails(heading, username, password) {
  const text = `Kapture sign-in\nAddress: ${location.origin}/login\nUsername: ${username}\nTemporary password: ${password}`;
  return formDialog({
    title: heading,
    body: 'Share these privately. At the first sign-in the user chooses a new password and sets up two-factor sign-in.',
    fields: [{
      node: h('div', { class: 'cred' },
        h('div', null, h('span', { class: 'muted' }, 'Username  '), h('b', null, username)),
        h('div', null, h('span', { class: 'muted' }, 'Password  '), h('code', null, password)),
        h('div', null, h('button', { class: 'btn btn-sm', type: 'button', onclick: () => copy(text) }, icon('copy'), 'Copy details'))),
    }],
    submitText: 'Done',
    cancel: false,
  });
}

export function mount(root) {
  let alive = true;
  const box = h('div', { class: 'card table-card' });
  root.append(h('div', { class: 'page' },
    h('div', { class: 'page-head' },
      h('div', null, h('h1', null, 'Users'), h('p', null, 'Administrators manage accounts, operators use everything else')),
      h('div', { class: 'page-actions' }, h('button', { class: 'btn btn-primary', type: 'button', onclick: addUser }, icon('plus'), 'Add user'))),
    box,
    h('p', { class: 'page-foot' }, icon('help'),
      'New and reset accounts sign in with a temporary password, then choose their own and set up two-factor sign-in. ',
      'Administrators can also recover themselves on the sign-in page with their recovery question.')));

  async function load() {
    let list;
    try {
      list = await api.get('/users');
    } catch (e) {
      if (alive) box.replaceChildren(emptyState('alert', 'Could not load users', e.message));
      return;
    }
    if (alive) render(list || []);
  }

  // Sign-in state, plus a button for admins still missing a recovery question.
  function status(u, self) {
    const state = !u.has_2fa ? h('span', { class: 'badge' }, '2FA setup pending')
      : u.must_change_password ? h('span', { class: 'badge' }, 'Temporary password')
        : h('span', { class: 'badge ok' }, 'Active');
    const recovery = u.role === 'admin' && !u.has_recovery
      ? h('button', {
        class: 'badge err badge-btn', type: 'button', title: 'Without it only another administrator can reset this account. Click to set one.',
        onclick: () => setRecovery(u, self),
      }, icon('plus'), 'Set recovery question')
      : null;
    return h('div', { class: 'status-cell' }, state, recovery);
  }

  // Recovery questions: another admin's directly, the own one with the password.
  let questions = null;
  async function setRecovery(u, self) {
    try {
      questions ||= await api.get('/auth/questions');
    } catch (e) {
      return toast(e.message, 'error');
    }
    const fields = [
      { name: 'question', label: 'Question', type: 'select', options: [['', 'Choose a question'], ...questions.map((q) => [q.id, q.text])], value: u.recovery_question || '' },
      { name: 'answer', label: 'Answer', placeholder: 'Not case sensitive', hint: self ? '' : `Tell ${u.username} the answer privately. They can change it later in their profile.` },
    ];
    if (self) fields.push({ name: 'password', label: 'Your current password', type: 'password', autocomplete: 'current-password' });
    const ok = await formDialog({
      title: self ? 'Set your recovery question' : `Recovery question of ${u.username}`,
      body: 'On the sign-in page, "Forgot your password" asks this answer plus the 2FA code (to reset the password) or the password (to reset a lost phone).',
      fields, submitText: 'Save',
      onSubmit: (v) => {
        if (!v.question) throw new Error('Choose a question.');
        return self ? api.put('/profile/recovery', v) : api.put(`/users/${u.id}/recovery`, v);
      },
    });
    if (!ok) return;
    toast('Recovery question saved');
    load();
  }

  function render(list) {
    box.replaceChildren(h('div', { class: 'table-wrap' }, h('table', { class: 'table' },
      h('thead', null, h('tr', null,
        h('th', null, 'User'), h('th', null, 'Role'), h('th', null, 'Status'), h('th', null, 'Last sign-in'), h('th', null, 'Created'), h('th'))),
      h('tbody', null, list.map((u) => {
        const self = u.id === store.userId;
        return h('tr', null,
          h('td', null,
            h('div', { class: 'strong' }, u.display_name || u.username, self ? h('span', { class: 'badge', style: { marginLeft: '6px' } }, 'you') : null),
            h('div', { class: 'sub' }, u.username)),
          h('td', null, roleBadge(u.role)),
          h('td', null, status(u, self)),
          h('td', { class: 'muted' }, u.last_login_at ? fmtAgo(Date.parse(u.last_login_at)) : 'Never'),
          h('td', { class: 'muted' }, (u.created_at || '').slice(0, 10)),
          h('td', { class: 'num' }, actions(u, self)));
      })))));
  }

  function actions(u, self) {
    const recovery = u.role === 'admin'
      ? [{ icon: 'help', text: u.has_recovery ? 'Change recovery question' : 'Set recovery question', onClick: () => setRecovery(u, self) }]
      : [];
    const b = h('button', {
      class: 'icon-btn sm', type: 'button', title: 'Actions',
      onclick: () => menu(b, self
        ? [{ icon: 'pencil', text: 'Edit', onClick: () => editUser(u) }, ...recovery, { icon: 'user', text: 'Open my profile', onClick: () => { location.hash = '#/profile'; } }]
        : [
          { icon: 'pencil', text: 'Edit', onClick: () => editUser(u) },
          ...recovery,
          { icon: 'key', text: 'Reset password', onClick: () => resetPassword(u) },
          { icon: 'shield', text: 'Reset 2FA', onClick: () => reset2FA(u) },
          { icon: 'trash', text: 'Delete', onClick: () => remove(u) },
        ]),
    }, icon('more'));
    return b;
  }

  async function addUser() {
    const res = await formDialog({
      title: 'Add user', wide: true, submitText: 'Create user',
      fields: [
        { name: 'username', label: 'Username', placeholder: 'e.g. budi', autocomplete: 'off' },
        { name: 'display_name', label: 'Display name', required: false, placeholder: 'Optional' },
        { name: 'role', label: 'Role', type: 'select', options: ROLES, value: 'operator' },
        { name: 'password', label: 'Temporary password', value: randomPassword(), hint: `${PASSWORD_HINT}. The user replaces it at the first sign-in.` },
      ],
      onSubmit: async (v) => {
        const problem = passwordProblem(v.password);
        if (problem) throw new Error(problem);
        return { user: await api.post('/users', v), password: v.password };
      },
    });
    if (!res) return;
    toast(`User ${res.user.username} created`);
    load();
    shareDetails('User created', res.user.username, res.password);
  }

  async function editUser(u) {
    const res = await formDialog({
      title: `Edit ${u.username}`,
      fields: [
        { name: 'username', label: 'Username', value: u.username },
        { name: 'display_name', label: 'Display name', value: u.display_name, required: false },
        { name: 'role', label: 'Role', type: 'select', options: ROLES, value: u.role },
      ],
      onSubmit: (v) => api.patch(`/users/${u.id}`, v),
    });
    if (!res) return;
    toast('User saved');
    if (res.id === store.userId) {
      setUser(res);
      if (res.role !== 'admin') return void (location.hash = '#/dashboard');
    }
    load();
  }

  async function resetPassword(u) {
    const password = await formDialog({
      title: `Reset the password of ${u.username}?`, danger: true, submitText: 'Reset password',
      body: `${u.username} is signed out everywhere and chooses a new password at the next sign-in.`,
      fields: [{ name: 'password', label: 'Temporary password', value: randomPassword(), hint: PASSWORD_HINT }],
      onSubmit: async (v) => {
        const problem = passwordProblem(v.password);
        if (problem) throw new Error(problem);
        await api.post(`/users/${u.id}/password`, v);
        return v.password;
      },
    });
    if (!password) return;
    load();
    shareDetails('Password reset', u.username, password);
  }

  async function reset2FA(u) {
    const ok = await confirmDialog({
      title: `Reset two-factor sign-in of ${u.username}?`,
      body: 'Use this when they lost or replaced their phone. They are signed out everywhere and scan a new QR code at the next sign-in.',
      confirmText: 'Reset 2FA', danger: true,
    });
    if (!ok) return;
    try {
      await api.post(`/users/${u.id}/2fa/reset`);
      toast(`2FA of ${u.username} reset`);
    } catch (e) {
      toast(e.message, 'error');
    }
    load();
  }

  async function remove(u) {
    const ok = await confirmDialog({
      title: `Delete ${u.username}?`, body: 'The account and its sessions are removed. This cannot be undone.',
      confirmText: 'Delete user', danger: true, typeToConfirm: u.username,
    });
    if (!ok) return;
    try {
      await api.del(`/users/${u.id}`);
      toast(`User ${u.username} deleted`);
    } catch (e) {
      toast(e.message, 'error');
    }
    load();
  }

  load();
  return { refresh: load, destroy() { alive = false; } };
}
