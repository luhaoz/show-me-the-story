<script>
  import { createEventDispatcher } from 'svelte';
  import { api } from '../lib/api.js';
  import { addToast } from '../lib/stores.js';
  import { t } from '../lib/i18n/index.js';

  export let detail;

  const dispatch = createEventDispatcher();
  const categories = ['geography', 'faction', 'rule', 'knowledge', 'history', 'other'];
  let draft = null;
  let loadedID = '';
  let saving = false;
  let relationSource = '';
  let relationTarget = '';

  $: if (detail?.card?.id && detail.card.id !== loadedID) initialize(detail);
  $: characters = draft?.dependencies?.characters || [];
  $: worldview = draft?.dependencies?.worldview || [];
  $: organizations = draft?.dependencies?.organizations || [];
  $: dependencyOptions = [
    ...characters.map(item => ({ key: `character:${item.id}`, label: item.name })),
    ...organizations.map(item => ({ key: `organization:${item.id}`, label: item.name })),
    ...worldview.map(item => ({ key: `worldview:${item.id}`, label: item.name })),
  ];

  function initialize(value) {
    loadedID = value.card.id;
    draft = JSON.parse(JSON.stringify(value.card));
    if (draft.api) draft.api.api_key = '';
    if (draft.organization) draft.organization.members ||= [];
    if (draft.relation) {
      relationSource = `${draft.relation.source_type}:${draft.relation.source_id}`;
      relationTarget = `${draft.relation.target_type}:${draft.relation.target_id}`;
    }
  }

  function splitEntity(value) {
    const index = value.indexOf(':');
    return { type: value.slice(0, index), id: value.slice(index + 1) };
  }

  async function save() {
    if (!draft.name?.trim()) {
      addToast($t('sharedCards.nameRequired'), 'error');
      return;
    }
    if (draft.character && !draft.character.name?.trim()) {
      addToast($t('config.char.nameRequired'), 'error');
      return;
    }
    if (draft.worldview && (!draft.worldview.name?.trim() || !draft.worldview.description?.trim())) {
      addToast($t('config.wv.requiredFields'), 'error');
      return;
    }
    if (draft.organization && !draft.organization.name?.trim()) {
      addToast($t('config.org.nameRequired'), 'error');
      return;
    }
    if (draft.story && Number(draft.story.target_words_per_chapter) <= 0) {
      addToast($t('sharedCards.targetWordsInvalid'), 'error');
      return;
    }
    if (draft.relation) {
      if (!relationSource || !relationTarget) {
        addToast($t('config.rel.bothRequired'), 'error');
        return;
      }
      if (relationSource === relationTarget) {
        addToast($t('config.rel.sameEntity'), 'error');
        return;
      }
      if (!draft.relation.label?.trim()) {
        addToast($t('config.rel.labelRequired'), 'error');
        return;
      }
      const source = splitEntity(relationSource);
      const target = splitEntity(relationTarget);
      draft.relation.source_type = source.type;
      draft.relation.source_id = source.id;
      draft.relation.target_type = target.type;
      draft.relation.target_id = target.id;
    }
    const payload = { name: draft.name.trim(), description: draft.description || '' };
    payload[draft.kind] = draft[draft.kind];
    saving = true;
    try {
      await api('PUT', `/api/shared-cards/${draft.id}`, payload);
      addToast($t('sharedCards.updated'), 'success');
      dispatch('saved');
    } catch (e) {
      addToast(e.message, 'error');
    } finally {
      saving = false;
    }
  }
</script>

{#if draft}
  <div class="modal modal-open z-[60]" role="dialog" aria-modal="true" aria-labelledby="shared-card-edit-title">
    <div class="modal-box w-[min(64rem,calc(100vw-2rem))] max-w-5xl max-h-[88vh] p-0 overflow-hidden flex flex-col">
      <div class="flex items-start justify-between gap-3 p-4 sm:p-5 border-b border-base-content/10 shrink-0">
        <div>
          <h3 id="shared-card-edit-title" class="font-semibold text-lg">{$t('sharedCards.editTitle')}</h3>
          <p class="text-xs text-base-content/60 mt-1">{$t('sharedCards.editHint')}</p>
        </div>
        <span class="badge badge-primary">{$t('sharedCards.kind.' + draft.kind)}</span>
      </div>

      <div class="overflow-y-auto p-4 sm:p-5 space-y-4 min-h-0">
        <div class="bg-base-300 rounded-lg p-3 space-y-2">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-x-3 gap-y-2">
            <div>
              <span class="text-xs text-base-content/65 mb-0.5 block">{$t('sharedCards.cardName')}</span>
              <input class="input input-sm w-full" bind:value={draft.name} maxlength="100" />
            </div>
            <div>
              <span class="text-xs text-base-content/65 mb-0.5 block">{$t('sharedCards.cardDescription')}</span>
              <input class="input input-sm w-full" bind:value={draft.description} maxlength="1000" />
            </div>
          </div>
        </div>

        <div>
          <h4 class="text-sm font-semibold mb-2">{$t('sharedCards.content')}</h4>
          <div class="bg-base-300 rounded-lg p-3 space-y-2">
            {#if draft.character}
              <div class="grid grid-cols-1 md:grid-cols-2 gap-x-3 gap-y-2">
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.char.name')}</span><input class="input input-sm w-full" bind:value={draft.character.name} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.char.age')}</span><input class="input input-sm w-full" bind:value={draft.character.age} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.char.appearance')}</span><textarea class="textarea textarea-sm w-full h-20 text-sm" bind:value={draft.character.appearance}></textarea></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.char.personality')}</span><textarea class="textarea textarea-sm w-full h-20 text-sm" bind:value={draft.character.personality}></textarea></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.char.background')}</span><textarea class="textarea textarea-sm w-full h-20 text-sm" bind:value={draft.character.background}></textarea></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.char.motivation')}</span><textarea class="textarea textarea-sm w-full h-20 text-sm" bind:value={draft.character.motivation}></textarea></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.char.abilities')}</span><textarea class="textarea textarea-sm w-full h-20 text-sm" bind:value={draft.character.abilities}></textarea></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.char.notes')}</span><textarea class="textarea textarea-sm w-full h-20 text-sm" bind:value={draft.character.notes}></textarea></div>
              </div>
            {:else if draft.worldview}
              <div class="grid grid-cols-1 md:grid-cols-2 gap-x-3 gap-y-2">
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.wv.name')}</span><input class="input input-sm w-full" bind:value={draft.worldview.name} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.wv.category')}</span><select class="select select-sm w-full" bind:value={draft.worldview.category}>{#if !categories.includes(draft.worldview.category)}<option value={draft.worldview.category}>{draft.worldview.category}</option>{/if}{#each categories as category}<option value={category}>{$t('config.wv.cat.' + category)}</option>{/each}</select></div>
                <div class="md:col-span-2"><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.wv.description')}</span><textarea class="textarea textarea-sm w-full h-24 text-sm" bind:value={draft.worldview.description}></textarea></div>
                <div class="md:col-span-2"><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.wv.tags')}</span><input class="input input-sm w-full" bind:value={draft.worldview.tags} /></div>
              </div>
            {:else if draft.organization}
              <div class="grid grid-cols-1 md:grid-cols-2 gap-x-3 gap-y-2">
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.org.name')}</span><input class="input input-sm w-full" bind:value={draft.organization.name} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.org.type')}</span><input class="input input-sm w-full" bind:value={draft.organization.type} /></div>
                <div class="md:col-span-2"><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.org.description')}</span><textarea class="textarea textarea-sm w-full h-24 text-sm" bind:value={draft.organization.description}></textarea></div>
                <div class="md:col-span-2">
                  <span class="text-xs text-base-content/65 mb-1 block">{$t('config.org.members')}</span>
                  <div class="flex flex-wrap gap-x-3 gap-y-1 min-h-8 rounded-md border border-base-content/10 bg-base-100/50 p-2">
                    {#each characters as character}<label class="flex items-center gap-1 cursor-pointer text-sm"><input type="checkbox" class="checkbox checkbox-xs" bind:group={draft.organization.members} value={character.id} />{character.name}</label>{/each}
                  </div>
                </div>
              </div>
            {:else if draft.relation}
              <div class="space-y-2">
                <div class="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] gap-2 items-end">
                  <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.rel.source')}</span><select class="select select-sm w-full" bind:value={relationSource}>{#each dependencyOptions as option}<option value={option.key}>{option.label}</option>{/each}</select></div>
                  <span class="text-base-content/65 pb-1.5">→</span>
                  <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.rel.target')}</span><select class="select select-sm w-full" bind:value={relationTarget}>{#each dependencyOptions as option}<option value={option.key}>{option.label}</option>{/each}</select></div>
                </div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.rel.label')}</span><input class="input input-sm w-full" bind:value={draft.relation.label} /></div>
              </div>
            {:else if draft.api}
              <div class="grid grid-cols-1 md:grid-cols-2 gap-x-3 gap-y-2">
                <div class="md:col-span-2">
                  <span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.api.baseUrl')}</span>
                  <input class="input input-sm w-full" bind:value={draft.api.base_url} />
                  <label class="label cursor-pointer justify-start gap-2 py-1 px-0 min-h-0"><input type="checkbox" class="toggle toggle-xs" bind:checked={draft.api.url_strict} /><span class="text-xs text-base-content/60">{$t('config.api.urlStrict')}</span></label>
                </div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.api.model')}</span><input class="input input-sm w-full" bind:value={draft.api.model} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.api.timeout')}</span><input type="number" class="input input-sm w-full" bind:value={draft.api.http_timeout_seconds} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.api.maxTokens')}</span><input type="number" class="input input-sm w-full" bind:value={draft.api.max_tokens} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('sharedCards.contextBudget')}</span><input type="number" class="input input-sm w-full" bind:value={draft.api.context_budget_tokens} /></div>
                <div class="md:col-span-2"><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.api.key')}</span><input type="password" class="input input-sm w-full" bind:value={draft.api.api_key} placeholder={detail.has_api_key ? $t('sharedCards.apiKeyPreserved') : ''} /><span class="text-xs text-warning mt-1 block">{$t('sharedCards.secretWarning')}</span></div>
              </div>
            {:else if draft.story}
              <div class="grid grid-cols-1 md:grid-cols-2 gap-x-3 gap-y-2">
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.story.type')}</span><input class="input input-sm w-full" bind:value={draft.story.type} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.story.titleField')}</span><input class="input input-sm w-full" bind:value={draft.story.title} /></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.story.targetWords')}</span><input type="number" class="input input-sm w-full" bind:value={draft.story.target_words_per_chapter} /></div>
              </div>
            {:else if draft.style}
              <div class="space-y-2">
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.style.label')}</span><textarea class="textarea w-full h-28 text-base" bind:value={draft.style.writing_style}></textarea></div>
                <div><span class="text-xs text-base-content/65 mb-0.5 block">{$t('config.pov.label')}</span><textarea class="textarea w-full h-20 text-base" bind:value={draft.style.writing_pov}></textarea></div>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <div class="flex justify-end gap-2 p-3 sm:px-5 border-t border-base-content/10 bg-base-100 shrink-0">
        <button class="btn btn-ghost" on:click={() => dispatch('cancel')} disabled={saving}>{$t('common.cancel')}</button>
        <button class="btn btn-primary" on:click={save} disabled={saving}>{#if saving}<span class="loading loading-spinner loading-xs"></span>{/if}{$t('common.save')}</button>
      </div>
    </div>
  </div>
{/if}
