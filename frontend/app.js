// WireOS frontend
const API_BASE = 'http://localhost:8080';

async function init() {
  const res = await fetch(`${API_BASE}/health`);
  const data = await res.json();
  console.log('WireOS backend:', data);
}

document.addEventListener('DOMContentLoaded', init);