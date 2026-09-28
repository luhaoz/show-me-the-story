import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const editor = readFileSync(new URL('../components/SharedCardEditor.svelte', import.meta.url), 'utf8');
const library = readFileSync(new URL('../components/SharedCardLibrary.svelte', import.meta.url), 'utf8');

assert.equal(editor.includes('class="form-control'), false, 'editor must not use the drifting DaisyUI form-control layout');
assert.match(editor, /overflow-y-auto[^"\n]*min-h-0/, 'editor body must scroll independently');
assert.match(editor, /border-t[^"\n]*bg-base-100[^"\n]*shrink-0/, 'editor actions must remain in a stable footer');
assert.ok((editor.match(/input input-sm w-full/g) || []).length >= 10, 'editor inputs must use the project form width');
assert.ok((editor.match(/textarea[^"\n]*w-full/g) || []).length >= 8, 'editor textareas must use the project form width');
assert.match(library, /items-start gap-2\.5 bg-base-300 rounded-lg/, 'library cards must match setting-card structure');
assert.match(library, /flex flex-col items-end gap-1 shrink-0/, 'library card actions must stay aligned on the right');

console.log('shared card UI checks passed');
