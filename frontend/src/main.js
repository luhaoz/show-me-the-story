import App from './App.svelte';
import './app.css';
import { initRuntimeTransport } from './lib/desktop.js';

async function bootstrap() {
  await initRuntimeTransport();
  return new App({
    target: document.getElementById('app')
  });
}

export default bootstrap().catch(error => {
  const target = document.getElementById('app');
  if (target) target.textContent = `Application startup failed: ${error?.message || error}`;
  throw error;
});
