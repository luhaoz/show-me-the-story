<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  import { taskRunning, addToast } from '../lib/stores.js';
  import { t } from '../lib/i18n/index.js';

  let baseURL = '';
  let token = '';
  let configured = false;
  let busy = false;
  onMount(async () => {
    try { const cfg = await api('GET', '/api/config/trilium'); baseURL = cfg.base_url || ''; configured = cfg.configured; }
    catch (e) { addToast(e.message, 'error'); }
  });
  async function save() {
    busy = true;
    try {
      const cfg = await api('PUT', '/api/config/trilium', { base_url: baseURL.trim(), token });
      configured = cfg.configured; token = '';
      addToast($t('trilium.saved'), 'success');
    } catch (e) { addToast(e.message, 'error'); }
    finally { busy = false; }
  }
  async function test() {
    busy = true;
    try { await api('POST', '/api/config/trilium/test'); addToast($t('trilium.connected'), 'success'); }
    catch (e) { addToast(e.message, 'error'); }
    finally { busy = false; }
  }
</script>

<section class="card bg-base-200" aria-label={$t('trilium.config.title')}>
  <div class="card-body p-4 gap-2">
    <h3 class="card-title text-base">{$t('trilium.config.title')}</h3>
    <p class="text-sm text-base-content/70">{$t('trilium.config.help')}</p>
    <label class="block text-sm" for="trilium-url">{$t('trilium.config.url')}</label>
    <input id="trilium-url" class="input input-sm w-full" type="url" placeholder="http://localhost:8080" bind:value={baseURL} disabled={$taskRunning || busy} />
    <label class="block text-sm" for="trilium-token">{$t('trilium.config.token')}</label>
    <input id="trilium-token" class="input input-sm w-full" type="password" bind:value={token} placeholder={configured ? $t('trilium.config.keepToken') : ''} disabled={$taskRunning || busy} />
    <div class="flex justify-end gap-2">
      <button class="btn btn-outline btn-sm" on:click={test} disabled={$taskRunning || busy || !configured}>{$t('trilium.config.test')}</button>
      <button class="btn btn-primary btn-sm" on:click={save} disabled={$taskRunning || busy || !baseURL.trim()}>{$t('common.save')}</button>
    </div>
  </div>
</section>
