// ── config ─────────────────────────────────────────────────────────────────
const API_BASE = window.location.hostname === 'localhost'
  ? 'http://localhost:8081'
  : 'https://wireos-backend.fly.dev';

const MAX_LOG_ENTRIES = 20;

// ── state ───────────────────────────────────────────────────────────────────
const logs        = [];
const loadingState = {};
let   lastRawData  = null;   // stores last successful response for raw toggle
let   showingRaw   = false;

// ── boot ────────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  checkHealth();
  loadCatalog();
  bindRunButtons();
  bindRawToggle();
});

// ── health check ────────────────────────────────────────────────────────────
async function checkHealth() {
  const dot   = document.getElementById('status-dot');
  const label = document.getElementById('status-label');
  try {
    const res  = await fetch(`${API_BASE}/health`);
    const data = await res.json();
    if (data.status === 'ok') {
      dot.className     = 'status-dot ok';
      label.textContent = `backend v${data.version} ✓`;
    } else {
      throw new Error('non-ok status');
    }
  } catch {
    dot.className     = 'status-dot err';
    label.textContent = 'backend unreachable';
  }
}

// ── catalog / status bar ────────────────────────────────────────────────────
async function loadCatalog() {
  const modeEl    = document.getElementById('catalog-mode');
  const countEl   = document.getElementById('catalog-count');
  const updatedEl = document.getElementById('catalog-updated');

  try {
    const res  = await fetch(`${API_BASE}/catalog`);
    const data = await res.json();

    const isLive = data.mode === 'live';
    modeEl.textContent  = isLive ? 'LIVE' : 'DEMO';
    modeEl.className    = 'mode-badge ' + (isLive ? 'mode-live' : 'mode-demo');
    countEl.textContent = `${data.intent_count} intents loaded`;

    const t = new Date();
    updatedEl.textContent = `last updated ${t.toLocaleTimeString()}`;
  } catch {
    modeEl.textContent  = 'OFFLINE';
    modeEl.className    = 'mode-badge mode-err';
    countEl.textContent = 'could not reach backend';
  }
}

// ── button binding ──────────────────────────────────────────────────────────
function bindRunButtons() {
  document.querySelectorAll('.run-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const intentId = btn.dataset.intent;
      if (!loadingState[intentId]) runIntent(intentId);
    });
  });
}

// ── raw JSON toggle ─────────────────────────────────────────────────────────
function bindRawToggle() {
  const btn = document.getElementById('toggle-raw-btn');
  if (!btn) return;

  btn.addEventListener('click', () => {
    if (!lastRawData) return;

    const highlighted = document.getElementById('result-block');
    const raw         = document.getElementById('raw-block');

    showingRaw = !showingRaw;
    if (showingRaw) {
      raw.textContent = JSON.stringify(lastRawData, null, 2);
      raw.hidden      = false;
      highlighted.hidden = true;
      btn.textContent = 'View Highlighted';
    } else {
      raw.hidden         = true;
      highlighted.hidden = false;
      btn.textContent    = 'View Raw JSON';
    }
  });
}

// ── run intent ───────────────────────────────────────────────────────────────
async function runIntent(intentId) {
  const card    = document.querySelector(`.intent-card[data-intent="${intentId}"]`);
  const btn     = card.querySelector('.run-btn');
  const sources = parseInt(card.dataset.sources, 10);

  // Reset raw toggle state on new run
  showingRaw   = false;
  lastRawData  = null;
  const rawBtn = document.getElementById('toggle-raw-btn');
  if (rawBtn) rawBtn.textContent = 'View Raw JSON';

  setLoading(intentId, true, card, btn, sources);
  const t0 = performance.now();

  try {
    const res = await fetch(`${API_BASE}/intent`, {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify({ intent_id: intentId, params: {} }),
    });

    const data      = await res.json();
    const latencyMs = Math.round(performance.now() - t0);

    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);

    lastRawData = data;
    renderResults(data, latencyMs);
    appendLog({ intentId, sources: data.results?.length ?? sources, latencyMs, partial: data.partial_failure, error: null });

  } catch (err) {
    const latencyMs = Math.round(performance.now() - t0);
    renderError(err.message);
    appendLog({ intentId, sources, latencyMs, partial: false, error: err.message });
  } finally {
    setLoading(intentId, false, card, btn, sources);
  }
}

// ── loading state ────────────────────────────────────────────────────────────
function setLoading(intentId, isLoading, card, btn, sources) {
  loadingState[intentId] = isLoading;

  const btnText    = btn.querySelector('.btn-text');
  const btnSpinner = btn.querySelector('.btn-loading');

  if (isLoading) {
    card.classList.add('loading');
    btn.disabled = true;
    btnText.hidden    = true;
    btnSpinner.hidden = false;
    btnSpinner.textContent = `◌ fetching from ${sources} source${sources !== 1 ? 's' : ''}…`;
  } else {
    card.classList.remove('loading');
    btn.disabled = false;
    btnText.hidden    = false;
    btnSpinner.hidden = true;
  }
}

// ── render results ───────────────────────────────────────────────────────────
function renderResults(data, clientLatencyMs) {
  const section    = document.getElementById('results-section');
  const block      = document.getElementById('result-block');
  const rawBlock   = document.getElementById('raw-block');
  const latBadge   = document.getElementById('latency-badge');
  const failBadge  = document.getElementById('failure-badge');
  const srcBadge   = document.getElementById('source-badge');

  const latMs = data.total_latency_ms ?? clientLatencyMs;
  latBadge.textContent = `${latMs}ms`;
  latBadge.style.color = '';

  if (data.partial_failure) {
    failBadge.textContent = '⚠ partial failure';
    failBadge.className   = 'meta-badge failure-badge-warn';
    failBadge.hidden      = false;
  } else {
    failBadge.textContent = '✓ all sources ok';
    failBadge.className   = 'meta-badge failure-badge-ok';
    failBadge.hidden      = false;
  }

  const resultCount    = data.results?.length ?? 0;
  srcBadge.textContent = `${resultCount} result${resultCount !== 1 ? 's' : ''}`;
  srcBadge.hidden      = false;

  // Highlighted view
  block.innerHTML = highlightJSON(JSON.stringify(data, null, 2));
  block.hidden    = false;
  rawBlock.hidden = true;

  section.hidden = true;
  requestAnimationFrame(() => { section.hidden = false; });
}

function renderError(message) {
  const section   = document.getElementById('results-section');
  const block     = document.getElementById('result-block');
  const rawBlock  = document.getElementById('raw-block');
  const latBadge  = document.getElementById('latency-badge');
  const failBadge = document.getElementById('failure-badge');
  const srcBadge  = document.getElementById('source-badge');

  latBadge.textContent = 'error';
  latBadge.style.color = 'var(--error, #f87171)';
  failBadge.hidden     = true;
  srcBadge.hidden      = true;

  block.innerHTML = `<span style="color:var(--error,#f87171)">✕ ${escapeHTML(message)}</span>`;
  block.hidden    = false;
  rawBlock.hidden = true;

  section.hidden = true;
  requestAnimationFrame(() => { section.hidden = false; });
}

// ── JSON syntax highlighter ──────────────────────────────────────────────────
function highlightJSON(json) {
  let out = '';
  let i   = 0;

  while (i < json.length) {
    const ch = json[i];

    if (ch === ' ' || ch === '\n' || ch === '\r' || ch === '\t') {
      out += ch;
      i++;
      continue;
    }

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
      let j = i;
      while (j < json.length && (json[j] === ' ' || json[j] === '\n')) j++;
      const isKey = json[j] === ':';
      out += `<span class="${isKey ? 'json-key' : 'json-str'}">${escapeHTML(str)}</span>`;
      continue;
    }

    if (ch === '-' || (ch >= '0' && ch <= '9')) {
      let num = '';
      while (i < json.length && '0123456789.-+eE'.includes(json[i])) num += json[i++];
      out += `<span class="json-num">${escapeHTML(num)}</span>`;
      continue;
    }

    if (json.startsWith('true',  i)) { out += `<span class="json-bool">true</span>`;  i += 4; continue; }
    if (json.startsWith('false', i)) { out += `<span class="json-bool">false</span>`; i += 5; continue; }
    if (json.startsWith('null',  i)) { out += `<span class="json-null">null</span>`;  i += 4; continue; }

    out += escapeHTML(ch);
    i++;
  }

  return out;
}

// ── log strip ────────────────────────────────────────────────────────────────
function appendLog({ intentId, sources, latencyMs, partial, error }) {
  const time = new Date().toTimeString().slice(0, 8);
  logs.unshift({ time, intentId, sources, latencyMs, partial, error: error ?? null });
  if (logs.length > MAX_LOG_ENTRIES) logs.pop();
  renderLogs();
}

function renderLogs() {
  const strip = document.getElementById('log-strip');
  if (!logs.length) {
    strip.innerHTML = '<div class="log-empty">no activity yet — run an intent to begin</div>';
    return;
  }
  strip.innerHTML = logs.map(e => {
    const timeSpan   = `<span class="log-time">[${e.time}]</span>`;
    const intentSpan = `<span class="log-intent">${e.intentId}</span>`;
    const arrow      = `<span class="log-arrow"> → </span>`;
    if (e.error) {
      return `<div class="log-entry">${timeSpan} ${intentSpan}${arrow}<span class="log-err">✕ ${escapeHTML(e.error)}</span></div>`;
    }
    const detail = `${e.sources} result${e.sources !== 1 ? 's' : ''} · ${e.latencyMs}ms${e.partial ? ' ⚠ partial' : ''}`;
    return `<div class="log-entry">${timeSpan} ${intentSpan}${arrow}<span class="log-detail">${detail}</span></div>`;
  }).join('');
}

// ── utils ─────────────────────────────────────────────────────────────────────
function escapeHTML(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}