// Run: node frontend/src/lib/sharedCards.check.js
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  SETTING_CARD_MIME,
  SHARED_CARD_MIME,
  beginSettingCardDrag,
  beginSharedCardDrag,
  readSettingCardDrag,
  readSharedCardDrag,
} from './sharedCards.js';

function transfer() {
  const values = new Map();
  return {
    effectAllowed: '',
    setData(type, value) { values.set(type, value); },
    getData(type) { return values.get(type) || ''; },
  };
}

const settingEvent = { dataTransfer: transfer() };
beginSettingCardDrag(settingEvent, 'organization', 'o_7');
assert.deepEqual(readSettingCardDrag(settingEvent), { kind: 'organization', entity_id: 'o_7' });
assert.ok(settingEvent.dataTransfer.getData(SETTING_CARD_MIME));

const sharedEvent = { dataTransfer: transfer() };
beginSharedCardDrag(sharedEvent, { id: 'card-id', kind: 'relation', name: 'friends' });
assert.deepEqual(readSharedCardDrag(sharedEvent), { id: 'card-id', kind: 'relation' });
assert.ok(sharedEvent.dataTransfer.getData(SHARED_CARD_MIME));

const configSource = readFileSync(new URL('../pages/Config.svelte', import.meta.url), 'utf8');
for (const kind of ['api', 'story', 'style', 'character', 'worldview', 'organization', 'relation']) {
  assert.match(configSource, new RegExp(`dropSharedCard\\(event, '${kind}'\\)`), `missing ${kind} drop target`);
  assert.match(configSource, new RegExp(`saveSettingCard\\('${kind}'`), `missing ${kind} add-to-library button`);
}
const outlineSource = readFileSync(new URL('../pages/Outline.svelte', import.meta.url), 'utf8');
assert.match(outlineSource, /saveSettingCard\('planning'/, 'outline must save the planning form as a card');
assert.match(outlineSource, /planningCardDraft\.subscribe/, 'outline must apply a planning card to the form');
const drawerSource = readFileSync(new URL('../components/SharedCardLibrary.svelte', import.meta.url), 'utf8');
assert.match(drawerSource, /readSettingCardDrag/);
assert.match(drawerSource, /beginSharedCardDrag/);
assert.match(drawerSource, /sharedCards\.use/);
assert.match(drawerSource, /SharedCardEditor/);
const editorSource = readFileSync(new URL('../components/SharedCardEditor.svelte', import.meta.url), 'utf8');
for (const field of ['appearance', 'personality', 'background', 'motivation', 'abilities', 'notes']) {
  assert.match(editorSource, new RegExp(`draft\\.character\\.${field}`), `shared character editor missing ${field}`);
}
for (const kind of ['worldview', 'organization', 'relation', 'api', 'story', 'style', 'planning']) {
  assert.match(editorSource, new RegExp(`draft\\.${kind}`), `shared editor missing ${kind} form`);
}

console.log('sharedCards.check.js: ok');
