<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  import { sharedCardLibraryOpen, taskRunning, addToast, showConfirm } from '../lib/stores.js';
  import { beginSharedCardDrag, readSettingCardDrag, importSharedCard, saveSettingCard } from '../lib/sharedCards.js';
  import { t } from '../lib/i18n/index.js';
  import SharedCardEditor from './SharedCardEditor.svelte';

  let cards = [];
  let loading = false;
  let dragActive = false;
  let editing = null;
  let editingLoadingID = '';
  let lastOpen = false;

  $: if ($sharedCardLibraryOpen !== lastOpen) {
    if ($sharedCardLibraryOpen) loadCards();
    lastOpen = $sharedCardLibraryOpen;
  }

  onMount(() => {
    const refresh = () => loadCards();
    window.addEventListener('shared-card-library-changed', refresh);
    loadCards();
    return () => window.removeEventListener('shared-card-library-changed', refresh);
  });

  async function loadCards() {
    loading = true;
    try { cards = await api('GET', '/api/shared-cards'); }
    catch (e) { addToast(e.message, 'error'); }
    finally { loading = false; }
  }

  async function captureDrop(event) {
    event.preventDefault();
    dragActive = false;
    const source = readSettingCardDrag(event);
    if (!source) return;
    await saveSettingCard(source.kind, source.entity_id, $t);
  }

  async function openEdit(card) {
    editingLoadingID = card.id;
    try {
      editing = await api('GET', `/api/shared-cards/${card.id}`);
    } catch (e) { addToast(e.message, 'error'); }
    finally { editingLoadingID = ''; }
  }

  async function editorSaved() {
    editing = null;
    await loadCards();
  }

  function removeCard(card) {
    showConfirm($t('sharedCards.deleteConfirm', { name: card.name }), async () => {
      try { await api('DELETE', `/api/shared-cards/${card.id}`); await loadCards(); addToast($t('sharedCards.deleted'), 'success'); }
      catch (e) { addToast(e.message, 'error'); }
    });
  }

  function dependencyText(card) {
    const d = card.dependency_counts || {};
    const total = (d.characters || 0) + (d.worldview || 0) + (d.organizations || 0);
    return total ? $t('sharedCards.dependencies', { n: total }) : '';
  }

  function cardGlyph(card) {
    if (card.kind === 'api') return 'API';
    if (card.kind === 'story') return '文';
    if (card.kind === 'style') return '✦';
    if (card.kind === 'planning') return '⌁';
    if (card.kind === 'relation') return '↔';
    return (card.name || '?').trim().slice(0, 1).toUpperCase();
  }
</script>

{#if $sharedCardLibraryOpen}
  <aside class="fixed top-14 right-0 bottom-0 z-50 w-[min(26rem,92vw)] bg-base-100 border-l border-base-content/15 flex flex-col" aria-label={$t('sharedCards.title')}>
    <div class="flex items-center gap-2 p-3 border-b border-base-content/10">
      <div class="flex-1"><h2 class="font-semibold">{$t('sharedCards.title')}</h2><p class="text-xs text-base-content/60">{$t('sharedCards.hint')}</p></div>
      <button class="btn btn-ghost btn-sm" on:click={() => sharedCardLibraryOpen.set(false)}>{$t('common.close')}</button>
    </div>
    <div
      class="m-3 rounded-lg border-2 border-dashed p-4 text-center text-sm transition-colors {dragActive ? 'border-primary bg-primary/10' : 'border-base-content/20'}"
      role="region"
      aria-label={$t('sharedCards.dropToSave')}
      on:dragenter|preventDefault={() => dragActive = true}
      on:dragover|preventDefault
      on:dragleave={() => dragActive = false}
      on:drop={captureDrop}
    >{$t('sharedCards.dropToSave')}</div>
    <div class="alert alert-warning mx-3 py-2 text-xs"><span>{$t('sharedCards.secretWarning')}</span></div>
    <div class="flex-1 overflow-y-auto p-3 space-y-2">
      {#if loading}<div class="flex justify-center p-6"><span class="loading loading-spinner"></span></div>
      {:else if cards.length === 0}<p class="text-sm text-base-content/55 text-center py-8">{$t('sharedCards.empty')}</p>
      {:else}{#each cards as card (card.id)}
        <div class="flex items-start gap-2.5 bg-base-300 rounded-lg border border-base-content/10 p-2.5 group cursor-grab" role="group" draggable="true" on:dragstart={(event) => beginSharedCardDrag(event, card)}>
          <div class="w-9 h-9 rounded-lg flex items-center justify-center text-xs font-bold shrink-0 {card.kind === 'character' ? 'rounded-full bg-primary/20 text-primary' : card.kind === 'worldview' ? 'bg-accent/20 text-accent' : card.kind === 'organization' ? 'bg-warning/20 text-warning' : 'bg-secondary/15 text-secondary'}">{cardGlyph(card)}</div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1.5 min-w-0">
              <div class="font-medium text-sm truncate">{card.name}</div>
              <span class="badge badge-outline badge-xs shrink-0">{$t('sharedCards.kind.' + card.kind)}</span>
            </div>
            {#if card.description}<p class="text-xs text-base-content/65 line-clamp-2 mt-0.5">{card.description}</p>{/if}
            {#if card.kind === 'planning'}<p class="text-xs text-base-content/65 line-clamp-2 mt-0.5">{$t('ending.' + card.planning_ending_intent)}{#if card.planning_ending_style && card.planning_ending_intent !== 'serial'} · {$t('ending.' + card.planning_ending_style)}{/if}{#if card.planning_direction} · {card.planning_direction}{/if}</p>{/if}
            <div class="flex flex-wrap items-center gap-1 mt-1 text-xs text-base-content/50">
              <span class="truncate max-w-full">{card.source_project || '—'}{dependencyText(card) ? ' · ' + dependencyText(card) : ''}</span>
              {#if card.source_language}<span class="badge badge-outline badge-xs uppercase">{card.source_language}</span>{/if}
              {#if card.has_api_key}<span class="badge badge-warning badge-xs">{$t('sharedCards.hasKey')}</span>{/if}
            </div>
          </div>
          <div class="flex flex-col items-end gap-1 shrink-0">
            <button class="btn btn-primary btn-xs h-6 min-h-0 px-2 whitespace-nowrap" on:click={() => importSharedCard(card, $t)} disabled={$taskRunning}>{$t('sharedCards.use')}</button>
            <div class="flex gap-0.5">
              <button class="btn btn-outline btn-xs px-1" on:click={() => openEdit(card)} disabled={$taskRunning || editingLoadingID === card.id}>{#if editingLoadingID === card.id}<span class="loading loading-spinner loading-xs"></span>{/if}{$t('common.edit')}</button>
              <button class="btn btn-error btn-outline btn-xs px-1" on:click={() => removeCard(card)} disabled={$taskRunning}>{$t('common.delete')}</button>
            </div>
          </div>
        </div>
      {/each}{/if}
    </div>
  </aside>
{/if}

{#if editing}
  <SharedCardEditor detail={editing} on:saved={editorSaved} on:cancel={() => editing = null} />
{/if}
