// Terminal color codes (ESC [ ... m) that apps such as Spring Boot write into
// their logs even without a TTY. They are rendered as colors, never shown raw.

const SEQ = /\x1b\[([0-9;?]*)[ -/]*([@-~])/g;

export const hasAnsi = (s) => !!s && s.includes('\x1b');

// Plain text without any escape sequence (for copying and one-line previews).
export const stripAnsi = (s) => (hasAnsi(s) ? s.replace(SEQ, '') : s);

const plain = () => ({ fg: null, bg: null, bold: false, dim: false, italic: false, underline: false });

// xterm 256-color index -> CSS color (0-15 use the themed palette).
function color256(n) {
  if (n < 16) return `var(--ansi-${n})`;
  if (n >= 232) {
    const v = 8 + (n - 232) * 10;
    return `rgb(${v} ${v} ${v})`;
  }
  const i = n - 16;
  const c = (x) => (x ? 55 + x * 40 : 0);
  return `rgb(${c(Math.floor(i / 36))} ${c(Math.floor(i / 6) % 6)} ${c(i % 6)})`;
}

function apply(st, params) {
  const codes = params === '' ? [0] : params.split(';').map((x) => Number(x) || 0);
  for (let i = 0; i < codes.length; i++) {
    const c = codes[i];
    if (c === 0) Object.assign(st, plain());
    else if (c === 1) st.bold = true;
    else if (c === 2) st.dim = true;
    else if (c === 3) st.italic = true;
    else if (c === 4) st.underline = true;
    else if (c === 22) st.bold = st.dim = false;
    else if (c === 23) st.italic = false;
    else if (c === 24) st.underline = false;
    else if (c >= 30 && c <= 37) st.fg = `var(--ansi-${c - 30})`;
    else if (c >= 90 && c <= 97) st.fg = `var(--ansi-${c - 82})`;
    else if (c >= 40 && c <= 47) st.bg = `var(--ansi-${c - 40})`;
    else if (c >= 100 && c <= 107) st.bg = `var(--ansi-${c - 92})`;
    else if (c === 39) st.fg = null;
    else if (c === 49) st.bg = null;
    else if (c === 38 || c === 48) {
      let col = null;
      if (codes[i + 1] === 5) {
        col = color256(codes[i + 2] || 0);
        i += 2;
      } else if (codes[i + 1] === 2) {
        col = `rgb(${codes[i + 2] || 0} ${codes[i + 3] || 0} ${codes[i + 4] || 0})`;
        i += 4;
      }
      if (c === 38) st.fg = col;
      else st.bg = col;
    }
  }
}

// Splits text into [{text, st}] runs of one style.
export function ansiSegments(text) {
  if (!hasAnsi(text)) return [{ text, st: null }];
  const out = [];
  const st = plain();
  let last = 0;
  for (const m of text.matchAll(SEQ)) {
    if (m.index > last) out.push({ text: text.slice(last, m.index), st: { ...st } });
    last = m.index + m[0].length;
    if (m[2] === 'm') apply(st, m[1]);
  }
  if (last < text.length) out.push({ text: text.slice(last), st: { ...st } });
  return out;
}

// Nodes for text with its colors; fmt(text) renders each run (e.g. search highlighting).
export function renderAnsi(text, fmt = (t) => t) {
  return ansiSegments(text).map(({ text: t, st }) => {
    if (!st || (!st.fg && !st.bg && !st.bold && !st.dim && !st.italic && !st.underline)) return fmt(t);
    const span = document.createElement('span');
    if (st.fg) span.style.color = st.fg;
    if (st.bg) span.style.background = st.bg;
    if (st.bold) span.style.fontWeight = '700';
    if (st.dim) span.style.opacity = '.7';
    if (st.italic) span.style.fontStyle = 'italic';
    if (st.underline) span.style.textDecoration = 'underline';
    const inner = fmt(t);
    span.append(...(Array.isArray(inner) ? inner : [inner]).filter((x) => x != null && x !== ''));
    return span;
  });
}
