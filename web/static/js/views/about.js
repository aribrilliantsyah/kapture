// About: why Kapture exists, what it runs on, and who built it.
import { h, icon } from '../ui.js';
import { store } from '../state.js';

const AUTHOR = {
  name: 'Ari Ardiansyah',
  github: 'aribrilliantsyah',
  email: 'ariardiansyah.study@gmail.com',
};

// Why the tool exists: what breaks without it, and what the usual fix costs.
const PROBLEMS = [
  ['Restarted pods take their logs with them', 'kubectl logs --previous only reaches one generation back, and once a pod is replaced its output is gone for good.'],
  ['kubelet rotation overwrites yesterday', 'The default 10 MB x 5 files per container is often only a few hours on a busy service.'],
  ['The usual answer is heavy', 'ELK or Loki plus Grafana want gigabytes of RAM per node and a stack to operate, which is a lot for "let me read last night\'s logs".'],
];

// The point of the tool: it does not care what runs in the container.
const AGNOSTIC = [
  ['Any framework, any language', 'Spring Boot, Express, Django, Laravel, Go services, a shell script in a CronJob: all the same to Kapture.'],
  ['No SDK, no sidecar, no code change', 'Nothing is added to your applications. Kapture reads the files kubelet already writes, in CRI and Docker json-file format.'],
  ['Any log style', 'Plain text, logfmt, JSON, klog and multi-line stack traces are stored as written. Levels and ANSI colors are detected, never required.'],
  ['Any workload type', 'Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, init containers and sidecars, grouped by their owner so a rollout keeps its history.'],
];

const STACK = [
  ['Go 1.26', 'One static binary that runs as both agent and aggregator. Nothing to install on the node.'],
  ['BadgerDB v4', 'Embedded store on each node. Keys are time-ordered and date-prefixed, so a day is a cheap range scan or a cheap delete.'],
  ['fsnotify + 1s poll', 'inotify finds new files; /var/log/containers entries are symlinks, so reads are polled, which also follows kubelet rotation.'],
  ['coder/websocket', 'Live tail streamed agent to aggregator to browser.'],
  ['bcrypt + TOTP (RFC 6238)', 'Password hashing, two-factor sign-in with QR enrollment, and HMAC-signed session cookies.'],
  ['Vanilla JS + go:embed', 'The whole dashboard ships inside the binary: no Node build step, no CDN, no external assets.'],
  ['JetBrains Mono Nerd Font', 'Monospace everywhere, so log glyphs and terminal colors read the way they do in a shell.'],
];

const FEATURES = [
  ['Per-node storage', 'Agents keep their own node\'s logs; the aggregator only routes queries and merges the answers in time order.'],
  ['Search and live tail', 'Field filters, phrases, OR, exclusions and regex, plus a WebSocket tail of matching new lines.'],
  ['Backup and restore', 'Every node into one .tar.gz, restorable into another Kapture, for example on a laptop.'],
  ['Accounts and 2FA', 'Admin and operator roles, recovery questions, and per-user authenticator enrollment.'],
];

export function mount(root) {
  const el = h('div', { class: 'page' });
  root.append(el);

  const card = (title, sub, ...content) => h('div', { class: 'card' },
    h('div', { class: 'card-head' }, h('div', null, h('h2', null, title), sub ? h('p', { class: 'card-sub' }, sub) : null)), ...content);
  const defs = (items) => h('dl', { class: 'about-list' },
    items.map(([term, text]) => [h('dt', null, term), h('dd', null, text)]).flat());
  const link = (href, ic, text, title) => h('a', { class: 'btn btn-sm', href, target: '_blank', rel: 'noopener noreferrer', title: title || text }, icon(ic), text);

  function render() {
    const version = [store.version, store.commit].filter(Boolean).join(' · ') || 'development build';
    el.replaceChildren(
      h('div', { class: 'page-head' }, h('div', null, h('h1', null, 'About Kapture'), h('p', null, 'Kubernetes Application & Pod Tracking and Unified Resource Explorer'))),
      h('div', { class: 'card about-hero' },
        h('img', { src: '/appicon.png', width: 64, height: 64, alt: '' }),
        h('div', null,
          h('h2', null, 'Kapture'),
          h('p', null, 'A log catcher for Kubernetes that keeps what kubectl throws away: the logs of pods that restarted, rolled out or disappeared, stored on the node itself and readable days later.'),
          h('div', { class: 'about-badges' },
            h('span', { class: 'badge' }, icon('server'), version),
            store.timezone ? h('span', { class: 'badge' }, icon('clock'), store.timezone) : null,
            h('span', { class: 'badge' }, icon('lock'), 'MIT license')))),

      h('div', { class: 'grid grid-2e mt' },
        card('Why it exists', 'The 2 a.m. problem this was written for', defs(PROBLEMS)),
        card('Agnostic by design', 'It never needs to know what runs inside the container', defs(AGNOSTIC))),

      h('div', { class: 'grid grid-2 mt' },
        card('Technology', 'What it is built on and why', defs(STACK)),
        card('What it does', 'The short version', defs(FEATURES))),

      h('div', { class: 'grid grid-2e mt' },
        card('Author', 'Built and maintained by',
          h('div', { class: 'about-author' },
            h('div', { class: 'about-avatar' }, AUTHOR.name.split(' ').map((w) => w[0]).slice(0, 2).join('')),
            h('div', null,
              h('div', { class: 'strong' }, AUTHOR.name),
              h('div', { class: 'muted' }, `github.com/${AUTHOR.github}`),
              h('div', { class: 'muted' }, AUTHOR.email))),
          h('div', { class: 'about-links' },
            link(`https://github.com/${AUTHOR.github}`, 'github', 'GitHub profile'),
            link(`mailto:${AUTHOR.email}`, 'mail', 'Send an email'))),
        card('Built with AI assistance', 'Written by a human, with models in the loop',
          h('p', { class: 'about-text' },
            'Parts of Kapture were designed and written with the help of ', h('b', null, 'Claude'), ' (Anthropic) and ', h('b', null, 'Gemini'), ' (Google): ',
            'storage layout and key design, the agent pipeline, the dashboard and its charts, and this page. ',
            'Every suggestion was reviewed, tested and adjusted by hand before it shipped.'),
          h('p', { class: 'note' }, icon('sparkles'),
            'Fonts by JetBrains (SIL OFL) and icons from Lucide (ISC). Thanks to both projects.'))));
  }

  render();
  return { refresh: render, destroy() {} };
}
