const WEB_RUNTIME = Object.freeze({ mode: 'web', apiBase: '', apiToken: '' });

let runtime = WEB_RUNTIME;

function bridge() {
  return globalThis.go?.main?.DesktopBridge || null;
}

function validLoopbackBase(value) {
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'http:' && (parsed.hostname === '127.0.0.1' || parsed.hostname === 'localhost');
  } catch (_) {
    return false;
  }
}

export async function initRuntimeTransport() {
  const desktopBridge = bridge();
  if (!desktopBridge?.RuntimeInfo) {
    runtime = WEB_RUNTIME;
    return runtime;
  }
  const info = await desktopBridge.RuntimeInfo();
  const apiBase = info?.api_base || info?.apiBase || '';
  const apiToken = info?.api_token || info?.apiToken || '';
  if (info?.mode !== 'desktop' || !validLoopbackBase(apiBase) || !apiToken) {
    throw new Error('Invalid desktop runtime configuration');
  }
  runtime = { mode: 'desktop', apiBase: apiBase.replace(/\/$/, ''), apiToken };
  return runtime;
}

export function isDesktop() {
  return runtime.mode === 'desktop';
}

export function apiURL(path) {
  if (!isDesktop()) return path;
  return `${runtime.apiBase}${path.startsWith('/') ? path : `/${path}`}`;
}

export function desktopHeaders() {
  return isDesktop() ? { 'X-Desktop-Token': runtime.apiToken } : {};
}

export function eventURL(path, params = {}) {
  const query = new URLSearchParams(params);
  if (isDesktop()) query.set('desktop_token', runtime.apiToken);
  const suffix = query.toString();
  return apiURL(path) + (suffix ? `?${suffix}` : '');
}

export async function nativeSaveExport(kind, projectName, suggestedName) {
  if (!isDesktop()) return null;
  return bridge().SaveExport(kind, projectName || '', suggestedName || '');
}

export async function nativeSaveText(suggestedName, content) {
  if (!isDesktop()) return null;
  return bridge().SaveText(suggestedName || '', content || '');
}

export async function openExternalURL(url) {
  if (!isDesktop()) return false;
  await bridge().OpenExternalURL(url);
  return true;
}

export function runtimeForTests() {
  return { ...runtime };
}
