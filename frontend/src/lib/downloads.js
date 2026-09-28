import { apiFetch } from './api.js';
import { isDesktop, nativeSaveExport, nativeSaveText } from './desktop.js';

function browserDownload(blob, suggestedName) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = suggestedName;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export async function saveServerExport({ kind, projectName = '', suggestedName, url }) {
  if (isDesktop()) {
    return nativeSaveExport(kind, projectName, suggestedName);
  }
  const response = await apiFetch(url);
  browserDownload(await response.blob(), suggestedName);
  return { cancelled: false };
}

export async function saveText({ suggestedName, content, mime = 'text/plain;charset=utf-8' }) {
  if (isDesktop()) {
    return nativeSaveText(suggestedName, content);
  }
  browserDownload(new Blob([content], { type: mime }), suggestedName);
  return { cancelled: false };
}
