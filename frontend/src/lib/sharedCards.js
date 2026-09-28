import { api } from './api.js';
import { get } from 'svelte/store';
import { apiConfig, config, settings, projectLanguage, addToast, showConfirm } from './stores.js';

export const SETTING_CARD_MIME = 'application/x-story-setting-card';
export const SHARED_CARD_MIME = 'application/x-shared-story-card';

export function beginSettingCardDrag(event, kind, entityId = '') {
  event.dataTransfer.effectAllowed = 'copy';
  event.dataTransfer.setData(SETTING_CARD_MIME, JSON.stringify({ kind, entity_id: entityId }));
  event.dataTransfer.setData('text/plain', kind);
}

export function beginSharedCardDrag(event, card) {
  event.dataTransfer.effectAllowed = 'copy';
  event.dataTransfer.setData(SHARED_CARD_MIME, JSON.stringify({ id: card.id, kind: card.kind }));
  event.dataTransfer.setData('text/plain', card.name);
}

export function readSettingCardDrag(event) {
  try { return JSON.parse(event.dataTransfer.getData(SETTING_CARD_MIME)); } catch { return null; }
}

export function readSharedCardDrag(event) {
  try { return JSON.parse(event.dataTransfer.getData(SHARED_CARD_MIME)); } catch { return null; }
}

export async function saveSettingCard(kind, entityId, translate) {
  try {
    await api('POST', '/api/shared-cards', { kind, entity_id: entityId || '' });
    addToast(translate('sharedCards.saved'), 'success');
    if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent('shared-card-library-changed'));
    return true;
  } catch (e) {
    addToast(e.message, 'error');
    return false;
  }
}

async function refreshConfigurationStores() {
  const [nextAPI, nextConfig, nextSettings] = await Promise.all([
    api('GET', '/api/config/api'), api('GET', '/api/config'), api('GET', '/api/settings'),
  ]);
  apiConfig.set(nextAPI); config.set(nextConfig); settings.set(nextSettings);
}

export async function importSharedCard(card, translate, confirmWritten = false, confirmLanguage = false) {
  if (!confirmLanguage && card.source_language && card.source_language !== get(projectLanguage)) {
    showConfirm(translate('sharedCards.languageConfirm', { language: card.source_language.toUpperCase() }), () => importSharedCard(card, translate, confirmWritten, true));
    return false;
  }
  try {
    const result = await api('POST', `/api/shared-cards/${card.id}/import`, { confirm_written: confirmWritten });
    await refreshConfigurationStores();
    addToast(translate('sharedCards.imported', { name: card.name }), 'success');
    if (result.requires_reconciliation) {
      showConfirm(translate('config.story.reconcileAsk'), async () => {
        try {
          const current = await api('GET', '/api/config');
          await api('POST', '/api/settings/reconcile', current.story);
          addToast(translate('config.story.reconcileStarted'), 'info');
        } catch (e) { addToast(e.message, 'error'); }
      });
    }
    return true;
  } catch (e) {
    if (e.code === 'shared_card_written_confirm' && !confirmWritten) {
      showConfirm(translate('sharedCards.writtenConfirm'), () => importSharedCard(card, translate, true, true));
      return false;
    }
    addToast(e.message, 'error');
    return false;
  }
}
