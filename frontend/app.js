// ── config ────────────────────────────────────────────────────
const API_BASE = 'http://localhost:8080';
const MAX_LOG_ENTRIES = 20;

// ── state ─────────────────────────────────────────────────────
const logs = [];                   // array of log strings (max 20)
const loadingState = {};           // { [intentId]: bool }

// ── boot ──────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  checkHealth();
  bindRunButtons();
});

// ── health check ──────────────────────────────────────────────
async function checkHealth() {
  const dot   = document.getElementById('status-dot');
  const label = document.getElementById('status-label');
  try {
    const res = await fetch(`${API_BASE}/health`);
    const data = await res.json();
    if (data.status === 'ok') {
      dot.className   = 'status-dot ok';
      label.textContent = `backend v${data.version} ✓`;
    } else {
      throw new Error('non-ok status');
    }
  } catch {
    dot.className   = 'status-dot err';
    label.textContent = 'backend unreachable';
  }
}

// ── button binding ────────────────────────────────────────────
function bindRunButtons() {
  document.querySelectorAll('.run-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const intentId = btn.dataset.intent;
      if (!loadingState[intentId]) runIntent(intentId);
    });
  });
}

// ── run intent ────────────────────────────────────────────────
async function runIntent(intentId) {
  const card = document.querySelector(`.intent-card[data-intent="${intentId}"]`);
  const btn  = card.querySelector('.run-btn');
  const sources = parseInt(card.dataset.sources, 10);

  setLoading(intentId, true, card, btn, sources);
  const t0 = performance.now();

  try {
    const res = await fetch(`${API_BASE}/intent`, {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify({ intent_id: intentId, params: {} }),
    });

    const data = await res.json();
    const latencyMs = Math.round(performance.now() - t0);

    if (!res.ok) {
      const msg = data.error || `HTTP ${res.status}`;
      throw new Error(msg);
    }

    renderResults(data, latencyMs);
    appendLog({
      intentId,
      sources: data.results?.length ?? sources,
      latencyMs,
      partial: data.partial_failure,
      error:   null,
    });

  } catch (err) {
    const latencyMs = Math.round(performance.now() - t0);
    renderError(err.message);
    appendLog({
      intentId,
      sources,
      latencyMs,
      partial: false,
      error:   err.message,
    });
  } finally {
    setLoading(intentId, false, card, btn, sources);
  }
}

// ── loading state ─────────────────────────────────────────────
function setLoading(intentId, isLoading, card, btn, sources) {
  loadingState[intentId] = isLoading;

  const btnText    = btn.querySelector('.btn-text');
  const btnSpinner = btn.querySelector('.btn-loading');

  if (isLoading) {
    card.classList.add('loading');
    btn.disabled = true;
    btnText.hidden    = true;
    btnSpinner.hidden = false;
    btnSpinner.textContent = `◌ fetching from ${sources} source${sources !== 1 ? 's' : ''}...`;
  } else {
    card.classList.remove('loading');
    btn.disabled = false;
    btnText.hidden    = false;
    btnSpinner.hidden = true;
  }
}

// ── render results ────────────────────────────────────────────
function renderResults(data, clientLatencyMs) {
  const section = document.getElementById('results-section');
  const block   = document.getElementById('result-block');
  const latBadge = document.getElementById('latency-badge');
  const failBadge = document.getElementById('failure-badge');
  const srcBadge  = document.getElementById('source-badge');

  // latency badge — prefer server-reported, show client as fallback
  const latMs = data.total_latency_ms ?? clientLatencyMs;
  latBadge.textContent = `${latMs}ms`;

  // partial failure badge
  if (data.partial_failure) {
    failBadge.textContent  = '⚠ partial failure';
    failBadge.className    = 'meta-badge failure-badge-warn';
    failBadge.hidden       = false;
  } else {
    failBadge.textContent  = '✓ all sources ok';
    failBadge.className    = 'meta-badge failure-badge-ok';
    failBadge.hidden       = false;
  }

  // source count badge
  const resultCount = data.results?.length ?? 0;
  srcBadge.textContent = `${resultCount} source${resultCount !== 1 ? 's' : ''}`;

  // syntax-highlighted JSON
  block.innerHTML = highlightJSON(JSON.stringify(data, null, 2));

  // reveal with animation (re-trigger by toggling hidden)
  section.hidden = true;
  requestAnimationFrame(() => { section.hidden = false; });
}

function renderError(message) {
  const section = document.getElementById('results-section');
  const block   = document.getElementById('result-block');
  const latBadge  = document.getElementById('latency-badge');
  const failBadge = document.getElementById('failure-badge');
  const srcBadge  = document.getElementById('source-badge');

  latBadge.textContent   = 'error';
  latBadge.style.color   = 'var(--error)';
  failBadge.hidden       = true;
  srcBadge.hidden        = true;

  block.innerHTML = `<span style="color:var(--error)">✕ ${escapeHTML(message)}</span>`;

  section.hidden = true;
  requestAnimationFrame(() => { section.hidden = false; });
}

// ── JSON syntax highlighter ───────────────────────────────────
// A minimal but complete tokenizer — no regex over the whole string,
// walks character by character to correctly handle strings containing : or digits.
function highlightJSON(json) {
  let out = '';
  let i   = 0;

  while (i < json.length) {
    const ch = json[i];

    // whitespace
    if (ch === ' ' || ch === '\n' || ch === '\r' || ch === '\t') {
      out += ch === '\n' ? '\n' : ch === '\t' ? '\t' : ' ';
      i++;
      continue;
    }

    // structural
    if ('{}[]'.includes(ch)) {
      out += `<span class="json-brace">${escapeHTML(ch)}</span>`;
      i++;
      continue;
    }

    if (ch === ',' || ch === ':') {
      out += escapeHTML(ch);
      i++;
      continue;
    }

    // string — walk until unescaped closing quote
    if (ch === '"') {
      let str = '"';
      i++;
      while (i < json.length) {
        const c = json[i];
        str += c;
        if (c === '\\') { str += json[++i]; i++; continue; }
        if (c === '"')  { i++; break; }
        i++;
      }
      // decide if key or value: look ahead past whitespace for ':'
      let j = i;
      while (j < json.length && (json[j] === ' ' || json[j] === '\n')) j++;
      const isKey = json[j] === ':';
      const cls   = isKey ? 'json-key' : 'json-str';
      out += `<span class="${cls}">${escapeHTML(str)}</span>`;
      continue;
    }

    // number
    if (ch === '-' || (ch >= '0' && ch <= '9')) {
      let num = '';
      while (i < json.length && '0123456789.-+eE'.includes(json[i])) {
        num += json[i++];
      }
      out += `<span class="json-num">${escapeHTML(num)}</span>`;
      continue;
    }

    // true / false / null
    if (json.startsWith('true', i)) {
      out += `<span class="json-bool">true</span>`;
      i += 4; continue;
    }
    if (json.startsWith('false', i)) {
      out += `<span class="json-bool">false</span>`;
      i += 5; continue;
    }
    if (json.startsWith('null', i)) {
      out += `<span class="json-null">null</span>`;
      i += 4; continue;
    }

    // fallback
    out += escapeHTML(ch);
    i++;
  }

  return out;
}

// ── log strip ─────────────────────────────────────────────────
function appendLog({ intentId, sources, latencyMs, partial, error }) {
  const now  = new Date();
  const time = now.toTimeString().slice(0, 8);

  let entry;
  if (error) {
    entry = { time, intentId, sources, latencyMs, error };
  } else {
    entry = { time, intentId, sources, latencyMs, partial, error: null };
  }

  logs.unshift(entry);           // newest first
  if (logs.length > MAX_LOG_ENTRIES) logs.pop();

  renderLogs();
}

function renderLogs() {
  const strip = document.getElementById('log-strip');

  if (logs.length === 0) {
    strip.innerHTML = '<div class="log-empty">no activity yet — run an intent to begin</div>';
    return;
  }

  strip.innerHTML = logs.map(entry => {
    const timeSpan   = `<span class="log-time">[${entry.time}]</span>`;
    const intentSpan = `<span class="log-intent">${entry.intentId}</span>`;
    const arrow      = `<span class="log-arrow"> → </span>`;

    if (entry.error) {
      return `<div class="log-entry">${timeSpan} ${intentSpan}${arrow}<span class="log-err">✕ ${escapeHTML(entry.error)}</span></div>`;
    }

    const detail = `${entry.sources} source${entry.sources !== 1 ? 's' : ''} → ${entry.latencyMs}ms${entry.partial ? ' ⚠ partial' : ''}`;
    return `<div class="log-entry">${timeSpan} ${intentSpan}${arrow}<span class="log-detail">${detail}</span></div>`;
  }).join('');
}

// ── utils ─────────────────────────────────────────────────────
function escapeHTML(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}