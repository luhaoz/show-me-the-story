import assert from 'node:assert/strict';

delete globalThis.go;
let desktop = await import('./desktop.js?web');
await desktop.initRuntimeTransport();
assert.equal(desktop.apiURL('/api/status'), '/api/status');
assert.equal(desktop.eventURL('/api/events', { locale: 'zh' }), '/api/events?locale=zh');
assert.deepEqual(desktop.desktopHeaders(), {});

const calls = [];
globalThis.go = { main: { DesktopBridge: {
  RuntimeInfo: async () => ({ mode: 'desktop', api_base: 'http://127.0.0.1:54321', api_token: 'secret' }),
  SaveExport: async (...args) => { calls.push(['export', ...args]); return { cancelled: false, path: 'x' }; },
  SaveText: async (...args) => { calls.push(['text', ...args]); return { cancelled: true }; },
  OpenExternalURL: async (url) => { calls.push(['open', url]); },
} } };
desktop = await import('./desktop.js?desktop');
await desktop.initRuntimeTransport();
assert.equal(desktop.apiURL('/api/status'), 'http://127.0.0.1:54321/api/status');
assert.deepEqual(desktop.desktopHeaders(), { 'X-Desktop-Token': 'secret' });
assert.equal(desktop.eventURL('/api/events', { locale: 'en' }), 'http://127.0.0.1:54321/api/events?locale=en&desktop_token=secret');
assert.equal((await desktop.nativeSaveExport('book', '', 'novel.txt')).cancelled, false);
assert.equal((await desktop.nativeSaveText('notes.md', 'body')).cancelled, true);
await desktop.openExternalURL('https://example.com');
assert.deepEqual(calls, [
  ['export', 'book', '', 'novel.txt'],
  ['text', 'notes.md', 'body'],
  ['open', 'https://example.com'],
]);

console.log('desktop runtime checks passed');
