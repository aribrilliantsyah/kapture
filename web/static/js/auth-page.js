// /setup (first run), /login and account recovery. Sign-in is a chain of
// steps decided by the server (code, new password, authenticator setup); the
// server sets an HttpOnly session cookie once the chain is done.
import { h, icon, copy, passwordProblem, passwordRules } from './ui.js';

const wrap = document.getElementById('card-wrap');
const card = document.getElementById('card');
const params = new URLSearchParams(location.search);

document.getElementById('copyright').textContent = `© ${new Date().getFullYear()} Kapture`;
document.getElementById('theme').addEventListener('click', () => {
  const t = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
  document.documentElement.dataset.theme = t;
  try { localStorage.setItem('kapture_theme', t); } catch { /* private mode */ }
});

// Only local paths; the #fragment survives the server redirect and is kept.
function nextURL() {
  const n = params.get('next') || '/';
  const path = n.startsWith('/') && !n.startsWith('//') ? n : '/';
  return path + location.hash;
}

async function post(path, body) {
  let resp;
  try {
    resp = await fetch('/api/v1' + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    });
  } catch {
    throw new Error('Cannot reach the Kapture server');
  }
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) throw Object.assign(new Error(data.error || `Request failed (HTTP ${resp.status})`), { expired: !!data.expired });
  return data;
}

function field(label, attrs) {
  const input = h('input', { class: 'input', required: true, spellcheck: 'false', ...attrs });
  return { input, el: h('label', { class: 'field' }, h('span', null, label), input) };
}

const title = (text, sub) => h('div', { class: 'auth-title' }, h('h1', null, text), h('p', null, sub));
const submit = (text) => h('button', { class: 'btn btn-primary btn-block', type: 'submit' }, text);
const note = (kind, text) => h('p', { class: `auth-note ${kind}` }, icon(kind === 'warn' ? 'alert' : 'help'), h('span', null, text));
const hint30 = () => h('p', { class: 'auth-hint' }, icon('lock'), ' You stay signed in on this browser for 30 days.');

async function busy(btn, text, fn) {
  const old = btn.textContent;
  btn.disabled = true;
  btn.textContent = text;
  try { await fn(); } finally { btn.disabled = false; btn.textContent = old; }
}

// Six single-digit boxes; typing, pasting and one-time-code autofill all work.
function otpInput(onComplete) {
  const boxes = Array.from({ length: 6 }, (_, i) => h('input', {
    class: 'otp-box', inputmode: 'numeric', autocomplete: i === 0 ? 'one-time-code' : 'off', 'aria-label': `Digit ${i + 1}`,
  }));
  const value = () => boxes.map((b) => b.value).join('');
  const fill = (digits, from) => {
    digits.split('').forEach((c, j) => { if (boxes[from + j]) boxes[from + j].value = c; });
    boxes[Math.min(from + digits.length, 5)].focus();
    if (value().length === 6) onComplete(value());
  };
  boxes.forEach((b, i) => {
    b.addEventListener('input', () => {
      const digits = b.value.replace(/\D/g, '');
      b.value = '';
      if (digits) fill(digits.slice(0, 6 - i), i);
    });
    b.addEventListener('keydown', (e) => {
      if (e.key === 'Backspace' && !b.value && i > 0) boxes[i - 1].focus();
      if (e.key === 'ArrowLeft' && i > 0) boxes[i - 1].focus();
      if (e.key === 'ArrowRight' && i < 5) boxes[i + 1].focus();
    });
    b.addEventListener('paste', (e) => {
      const digits = (e.clipboardData.getData('text') || '').replace(/\D/g, '').slice(0, 6);
      if (!digits) return;
      e.preventDefault();
      boxes.forEach((x) => { x.value = ''; });
      fill(digits, 0);
    });
  });
  return {
    el: h('div', { class: 'otp-row' }, boxes),
    value,
    clear() { boxes.forEach((b) => { b.value = ''; }); boxes[0].focus(); },
    focus() { boxes[0].focus(); },
  };
}

const otpField = (label, otp) => h('div', { class: 'field center' }, h('span', null, label), otp.el);

function qrBlock(qrURL, secret) {
  return h('div', { class: 'qr' },
    qrURL ? h('img', { src: qrURL, alt: 'QR code for your authenticator app' }) : h('div', { class: 'skel', style: { width: '140px', height: '140px' } }),
    h('div', null,
      h('div', { class: 'muted' }, 'Cannot scan? Enter this key:'),
      h('code', null, secret || '...'),
      secret ? h('button', { class: 'btn btn-sm', type: 'button', onclick: () => copy(secret) }, icon('copy'), 'Copy key') : null));
}

function questionSelect(questions) {
  return h('select', { class: 'select wide input', required: true },
    h('option', { value: '' }, 'Choose a question'), questions.map((q) => h('option', { value: q.id }, q.text)));
}

function step(iconName, heading, sub) {
  const ic = h('div', { class: 'step-icon' }, icon(iconName));
  const body = h('div', { class: 'step-body' });
  const el = h('section', { class: 'step' }, ic, h('div', { class: 'step-head' }, h('h2', null, heading), h('p', null, sub)), body);
  return {
    el, body,
    state(s) {
      el.className = `step ${s}`;
      ic.replaceChildren(icon(s === 'done' ? 'checkmark' : iconName));
      body.hidden = s !== 'active';
    },
  };
}

// ── First-run setup: account, recovery question, authenticator ──
async function renderSetup() {
  wrap.classList.add('wide');
  let questions = [];
  try { questions = await (await fetch('/api/v1/auth/questions')).json(); } catch { /* the select stays empty */ }

  const user = field('Username', { value: 'admin', autocomplete: 'username' });
  const pass = field('Password', { type: 'password', minlength: 8, autocomplete: 'new-password', placeholder: 'e.g. Qawsed#1477' });
  const pass2 = field('Confirm password', { type: 'password', minlength: 8, autocomplete: 'new-password' });
  const err1 = h('p', { class: 'form-error' });
  const next1 = submit('Continue');
  const s1 = step('user', 'Administrator account', 'Administrators manage users; you can add operators later');
  const form1 = h('form', { class: 'auth-form' }, user.el, pass.el, passwordRules(pass.input), pass2.el, next1, err1);
  s1.body.append(form1);

  const question = questionSelect(questions);
  const answer = field('Answer', { autocomplete: 'off', placeholder: 'Not case sensitive' });
  const err2 = h('p', { class: 'form-error' });
  const next2 = submit('Continue');
  const back2 = h('button', { class: 'link-btn', type: 'button' }, 'Back');
  const s2 = step('help', 'Recovery question', 'Lets you reset a lost password or phone from the sign-in page');
  const form2 = h('form', { class: 'auth-form' }, h('label', { class: 'field' }, h('span', null, 'Question'), question), answer.el, next2, err2,
    h('div', { class: 'auth-links' }, back2));
  s2.body.append(form2);

  const s3 = step('shield', 'Two-factor sign-in', 'Scan the QR code with Google Authenticator, Authy or 1Password');
  const err3 = h('p', { class: 'form-error' });
  const finish = submit('Enable 2FA and sign in');
  const back3 = h('button', { class: 'link-btn', type: 'button' }, 'Back');
  let secret = '';
  const complete = () => busy(finish, 'Verifying...', async () => {
    err3.textContent = '';
    try {
      await post('/auth/setup/complete', {
        username: user.input.value.trim(), password: pass.input.value, secret, code: otp.value(),
        question: question.value, answer: answer.input.value,
      });
      location.replace(nextURL());
    } catch (e) {
      err3.textContent = e.message;
      otp.clear();
    }
  });
  const otp = otpInput(complete);
  const qrBox = h('div', null, qrBlock('', ''));
  const form3 = h('form', { class: 'auth-form' }, qrBox, otpField('Enter the 6-digit code shown in the app', otp), finish, err3,
    h('div', { class: 'auth-links' }, back3));
  s3.body.append(form3);

  card.replaceChildren(title('Initial setup', 'Create the administrator account and turn on two-factor sign-in'),
    h('div', { class: 'steps' }, s1.el, s2.el, s3.el));
  const go = (n) => {
    [s1, s2, s3].forEach((s, i) => s.state(i + 1 < n ? 'done' : i + 1 === n ? 'active' : 'pending'));
    ({ 1: () => pass.input.focus(), 2: () => question.focus(), 3: () => otp.focus() })[n]();
  };
  go(1);
  user.input.focus();

  form1.addEventListener('submit', (e) => {
    e.preventDefault();
    err1.textContent = '';
    const problem = passwordProblem(pass.input.value);
    if (problem) return void (err1.textContent = problem);
    if (pass.input.value !== pass2.input.value) return void (err1.textContent = 'Passwords do not match.');
    go(2);
  });
  form2.addEventListener('submit', (e) => {
    e.preventDefault();
    err2.textContent = '';
    if (!question.value) return void (err2.textContent = 'Choose a question.');
    if (answer.input.value.trim().length < 3) return void (err2.textContent = 'The answer must be at least 3 characters.');
    busy(next2, 'Preparing...', async () => {
      try {
        if (!secret) {
          const init = await post('/auth/setup/init');
          secret = init.secret;
          qrBox.replaceChildren(qrBlock(init.qr_data_url, secret));
        }
        go(3);
      } catch (e2) {
        err2.textContent = e2.message;
      }
    });
  });
  form3.addEventListener('submit', (e) => { e.preventDefault(); complete(); });
  back2.addEventListener('click', () => go(1));
  back3.addEventListener('click', () => go(2));
}

// ── Sign in and recovery ──
function renderLogin() {
  let temp = '';
  let username = '';
  let timer = null;

  const stopTimer = () => clearTimeout(timer);
  // The server forgets a half-done sign-in after a few minutes: go back to the
  // password form and say why there, where the user has to act.
  function startTimer(seconds) {
    stopTimer();
    if (seconds > 0) timer = setTimeout(() => showCredentials('Your sign-in took too long. Enter your password again.'), seconds * 1000);
  }

  function show(wide, ...nodes) {
    wrap.classList.toggle('wide', wide);
    card.replaceChildren(...nodes);
  }

  function fail(e, err, otp) {
    if (e.expired) return showCredentials(e.message);
    err.textContent = e.message;
    otp?.clear();
  }

  function onStep(res) {
    if (res.step === 'done') {
      stopTimer();
      location.replace(nextURL());
      return;
    }
    temp = res.temp_token;
    username = res.username || username;
    startTimer(res.expires_in);
    const screens = { totp: showTOTP, password: showNewPassword, enroll: showEnroll };
    (screens[res.step] || (() => showCredentials('Unexpected sign-in step, start again.')))(res);
  }

  const accountChip = () => h('div', { class: 'account-chip' },
    h('span', null, icon('user'), ' ', h('b', null, username)),
    h('button', { class: 'link-btn', type: 'button', onclick: () => showCredentials() }, 'Change'));

  function showCredentials(notice = '') {
    stopTimer();
    temp = '';
    const user = field('Username', { autocomplete: 'username', value: username });
    const pass = field('Password', { type: 'password', autocomplete: 'current-password' });
    const err = h('p', { class: 'form-error' });
    const btn = submit('Continue');
    const forgot = h('button', { class: 'link-btn', type: 'button', onclick: () => showRecoverStart(user.input.value.trim()) }, 'Forgot your password or lost your phone?');
    const form = h('form', { class: 'auth-form' }, notice ? note('warn', notice) : null, user.el, pass.el, btn, err, h('div', { class: 'auth-links' }, forgot));
    show(false, title('Sign in', 'Welcome back to Kapture'), form, hint30());
    (user.input.value ? pass.input : user.input).focus();
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.textContent = '';
      busy(btn, 'Checking...', async () => {
        try {
          username = user.input.value.trim();
          onStep(await post('/auth/login/credentials', { username, password: pass.input.value }));
        } catch (e2) {
          err.textContent = e2.message;
          pass.input.select();
        }
      });
    });
  }

  function showTOTP() {
    const err = h('p', { class: 'form-error' });
    const btn = submit('Verify and sign in');
    const go = () => busy(btn, 'Verifying...', async () => {
      err.textContent = '';
      try { onStep(await post('/auth/login/2fa', { temp_token: temp, code: otp.value() })); } catch (e) { fail(e, err, otp); }
    });
    const otp = otpInput(go);
    const form = h('form', { class: 'auth-form' }, accountChip(), otpField('Code from your authenticator app', otp), btn, err);
    form.addEventListener('submit', (e) => { e.preventDefault(); go(); });
    show(false, title('Two-factor authentication', 'Enter the 6-digit code to finish signing in'), form);
    otp.focus();
  }

  function showNewPassword() {
    const pass = field('New password', { type: 'password', minlength: 8, autocomplete: 'new-password', placeholder: 'e.g. Qawsed#1477' });
    const pass2 = field('Confirm new password', { type: 'password', minlength: 8, autocomplete: 'new-password' });
    const err = h('p', { class: 'form-error' });
    const btn = submit('Save password and continue');
    const form = h('form', { class: 'auth-form' }, accountChip(), pass.el, passwordRules(pass.input), pass2.el, btn, err);
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.textContent = '';
      const problem = passwordProblem(pass.input.value);
      if (problem) return void (err.textContent = problem);
      if (pass.input.value !== pass2.input.value) return void (err.textContent = 'Passwords do not match.');
      busy(btn, 'Saving...', async () => {
        try { onStep(await post('/auth/login/password', { temp_token: temp, password: pass.input.value })); } catch (e2) { fail(e2, err); }
      });
    });
    show(false, title('Choose a new password', 'Your current password is temporary'), form);
    pass.input.focus();
  }

  function showEnroll(res) {
    const err = h('p', { class: 'form-error' });
    const btn = submit('Enable 2FA and continue');
    const go = () => busy(btn, 'Verifying...', async () => {
      err.textContent = '';
      try { onStep(await post('/auth/login/enroll', { temp_token: temp, code: otp.value() })); } catch (e) { fail(e, err, otp); }
    });
    const otp = otpInput(go);
    const form = h('form', { class: 'auth-form' }, accountChip(), qrBlock(res.qr_data_url, res.secret),
      otpField('Enter the 6-digit code shown in the app', otp), btn, err);
    form.addEventListener('submit', (e) => { e.preventDefault(); go(); });
    show(true, title('Set up two-factor sign-in', 'Scan the QR code with Google Authenticator, Authy or 1Password'), form);
    otp.focus();
  }

  function showRecoverStart(prefill) {
    stopTimer();
    const user = field('Username', { autocomplete: 'username', value: prefill || username });
    const err = h('p', { class: 'form-error' });
    const btn = submit('Continue');
    const back = h('button', { class: 'link-btn', type: 'button', onclick: () => showCredentials() }, 'Back to sign in');
    const form = h('form', { class: 'auth-form' },
      note('info', 'Administrators answer their recovery question. Operators: ask an administrator to reset your account.'),
      user.el, btn, err, h('div', { class: 'auth-links' }, back));
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.textContent = '';
      busy(btn, 'Checking...', async () => {
        try {
          const res = await post('/auth/recover/start', { username: user.input.value.trim() });
          temp = res.temp_token;
          username = res.username;
          startTimer(res.expires_in);
          showRecoverAnswer(res.question);
        } catch (e2) {
          err.textContent = e2.message;
        }
      });
    });
    show(false, title('Recover your account', 'Reset your password or your two-factor sign-in'), form);
    user.input.focus();
  }

  function showRecoverAnswer(question) {
    let mode = 'password';
    const answer = field(question, { autocomplete: 'off', placeholder: 'Your answer (not case sensitive)' });
    const err = h('p', { class: 'form-error' });
    const btn = submit('Verify');
    const go = () => busy(btn, 'Verifying...', async () => {
      err.textContent = '';
      try {
        onStep(await post('/auth/recover/verify', {
          temp_token: temp, answer: answer.input.value, mode,
          code: mode === 'password' ? otp.value() : '', password: mode === '2fa' ? pass.input.value : '',
        }));
      } catch (e) {
        fail(e, err, mode === 'password' ? otp : null);
      }
    });
    const otp = otpInput(() => { if (answer.input.value.trim()) go(); });
    const pass = field('Your password', { type: 'password', autocomplete: 'current-password' });
    const factor = h('div');
    const choice = (value, label, sub) => {
      const radio = h('input', { type: 'radio', name: 'mode', value, checked: value === mode });
      const el = h('label', { class: `choice-item${value === mode ? ' on' : ''}` }, radio, h('span', null, label, h('small', null, sub)));
      radio.addEventListener('change', () => { mode = value; sync(); });
      return el;
    };
    const choices = h('div', { class: 'choice' },
      choice('password', 'I forgot my password', 'Confirm with your 2FA code'),
      choice('2fa', 'I lost my phone', 'Confirm with your password'));
    function sync() {
      choices.querySelectorAll('.choice-item').forEach((c) => c.classList.toggle('on', c.querySelector('input').value === mode));
      factor.replaceChildren(mode === 'password' ? otpField('Current code from your authenticator app', otp) : pass.el);
      pass.input.required = mode === '2fa';
    }
    sync();
    const form = h('form', { class: 'auth-form' }, accountChip(), answer.el, choices, factor, btn, err,
      h('div', { class: 'auth-links' }, h('button', { class: 'link-btn', type: 'button', onclick: () => showCredentials() }, 'Back to sign in')));
    form.addEventListener('submit', (e) => { e.preventDefault(); go(); });
    show(false, title('Answer your recovery question', 'Plus one thing only you still have'), form);
    answer.input.focus();
  }

  showCredentials();
}

async function start() {
  let st;
  try {
    st = await (await fetch('/api/v1/auth/status')).json();
  } catch {
    card.replaceChildren(title('Kapture is unreachable', 'Check that the aggregator is running, then reload this page.'));
    return;
  }
  if (!st.auth_enabled || st.authenticated) return location.replace(nextURL());
  if (st.setup_needed) {
    if (location.pathname !== '/setup') history.replaceState(null, '', '/setup' + location.hash);
    document.title = 'Setup - Kapture';
    renderSetup();
  } else {
    renderLogin();
  }
}

start();
