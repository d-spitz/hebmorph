/* hebmorph reader — no dependencies.
   Texts live in localStorage, the analysis comes from the hebmorph API. */

const API = 'https://hebmorph.onrender.com/api/v1/analyze';
const KEY = 'hebmorph.texts';
const GLOSS_KEY = 'hebmorph.glossary';
const DESKTOP = '(min-width: 900px)';
const $ = (sel) => document.querySelector(sel);
const esc = (v) => String(v).replace(/[&<>"']/g, (c) =>
  ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

/* ---------- storage ---------- */

const loadAll = () => {
  try { return JSON.parse(localStorage.getItem(KEY)) || []; } catch { return []; }
};
const saveAll = (list) => localStorage.setItem(KEY, JSON.stringify(list));
const titleOf = (text) => text.trim().split('\n')[0].slice(0, 42) || 'Untitled';
const newId = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 8);

/* { [textId]: { [word]: { added, data } } }. The analysis is kept with the
   word so the list and the export never ask the API again. */
const loadGloss = () => {
  try { return JSON.parse(localStorage.getItem(GLOSS_KEY)) || {}; } catch { return {}; }
};
const saveGloss = (all) => localStorage.setItem(GLOSS_KEY, JSON.stringify(all));
const glossOf = (id) => loadGloss()[id] || {};

function addToGloss(id, word, data) {
  const all = loadGloss();
  const entries = (all[id] ||= {});
  if (entries[word]) return false;
  entries[word] = { added: Date.now(), data };
  saveGloss(all);
  return true;
}

function removeFromGloss(id, word) {
  const all = loadGloss();
  if (!all[id]) return;
  delete all[id][word];
  saveGloss(all);
}

/* ---------- words ----------
   The API takes only Hebrew letters plus ' and " (geresh and gershayim) and
   rejects niqqud with a 400, so normalize before asking. Typographic quotes
   fold to the ASCII ones the dictionary actually stores.

   Written as escapes, not literals: U+0591-U+05C7 are combining marks that
   merge with neighbouring source characters and do not survive editing. */
const MARKS = '\\u0591-\\u05C7';           // niqqud + cantillation
const GERESH = '\\u05F3\\u2018\\u2019';    // geresh, curly single quotes
const GERSHAYIM = '\\u05F4\\u201C\\u201D'; // gershayim, curly double quotes

const WORD_RE = new RegExp(
  `[א-ת${MARKS}]+(?:['"\\u05F3\\u05F4][א-ת${MARKS}]+)*['\\u05F3]?`, 'g');
const QUERYABLE = /^['"א-ת]+$/;

const normalize = (word) => word
  .normalize('NFC')
  .replace(new RegExp(`[${GERESH}]`, 'g'), "'")
  .replace(new RegExp(`[${GERSHAYIM}]`, 'g'), '"')
  .replace(new RegExp(`[${MARKS}]`, 'g'), '')
  .replace(/[^'"א-ת]/g, ''); // anything else the API refuses

/* Analyzable words become buttons; punctuation and anything unqueryable stays
   plain text, so we never fire a request we know would 400. */
function renderText(host, text) {
  const out = document.createDocumentFragment();
  let cursor = 0;

  for (const match of text.matchAll(WORD_RE)) {
    if (match.index > cursor) out.append(text.slice(cursor, match.index));

    const raw = match[0];
    const query = normalize(raw);

    if (QUERYABLE.test(query)) {
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'w';
      b.textContent = raw;
      b.dataset.word = query;
      b.dataset.at = match.index; // where to cut the context from, later
      out.append(b);
    } else {
      out.append(raw);
    }
    cursor = match.index + raw.length;
  }
  if (cursor < text.length) out.append(text.slice(cursor));

  host.replaceChildren(out);
}

/* ---------- analysis markup ---------- */

// Plain words, so the popover can tag them and the export can write them.
function featureList(f = {}) {
  const out = [];
  if (f.part_of_speech) out.push(f.part_of_speech);
  if (f.gender) out.push(f.gender);
  if (f.number) out.push(f.number);
  if (f.person) out.push(`person ${f.person}`);
  if (f.tense) out.push(f.tense);
  if (f.construct) out.push('construct');
  if (f.proper_noun) out.push('proper noun');
  if (f.possessive) {
    const { gender, person, number } = f.possessive;
    out.push(`suffix · ${[gender, person, number].filter(Boolean).join(' ')}`);
  }
  return out;
}

function featureTags(f = {}) {
  return featureList(f).map((text, i) => {
    const cls = i === 0 && f.part_of_speech ? 'tag tag-pos' : 'tag';
    return `<span class="${cls}">${esc(text)}</span>`;
  }).join('');
}

// hspell files 1012 readings with no part of speech at all, across 68 stems:
// about half are the שונות ("miscellaneous") bucket, the rest are prepositions
// whose inflected forms record only a stem (אחריו → אחרי). Triggered by
// "nothing decoded" rather than by any particular stem, so it covers every
// such entry rather than just the one that prompted it.
const NO_FEATURES = '<p class="note">No part of speech recorded — typically a ' +
  'particle, preposition, or other fixed form.</p>';

function readingHTML(r) {
  const tags = featureTags(r.features);
  // "x" is the native notation for "no type", i.e. it says nothing the note
  // above does not. Anything richer is still worth showing.
  const desc = r.desc && r.desc !== 'x'
    ? `<div class="desc" dir="rtl" lang="he">${esc(r.desc)}</div>`
    : '';
  // English senses of the stem, in this reading's part of speech. Absent for
  // proper nouns and the handful of stems still untranslated.
  const glosses = r.glosses && r.glosses.length
    ? `<div class="glosses" dir="ltr" lang="en">${r.glosses.map(esc).join(', ')}</div>`
    : '';
  return `<li class="reading">
      <div class="stem" dir="rtl" lang="he">${esc(r.stem)}</div>
      ${glosses}
      ${tags ? `<div class="tags">${tags}</div>` : NO_FEATURES}
      ${desc}
    </li>`;
}

function analysisHTML(data) {
  const parts = [];
  if (data.gimatria) {
    parts.push(`<p class="numeral">Hebrew numeral · <b>${esc(data.gimatria)}</b></p>`);
  }
  if (!data.splits || !data.splits.length) {
    parts.push('<p class="empty">No analysis found for this word.</p>');
    return parts.join('');
  }
  for (const split of data.splits) {
    const head = split.whole_word
      ? `<span class="base">${esc(split.base)}</span>`
      : `<span class="pfx">${esc(split.prefix)}</span><span class="join">+</span>` +
        `<span class="base">${esc(split.base)}</span>`;

    const readings = split.readings && split.readings.length
      ? `<ul class="readings">${split.readings.map(readingHTML).join('')}</ul>`
      : '<p class="empty">No readings for this split.</p>';

    parts.push(`<section class="split">` +
      `<h2 class="split-word" dir="rtl" lang="he">${head}</h2>${readings}</section>`);
  }
  return parts.join('');
}

// A plain GET with no custom headers stays a "simple" cross-origin request, so
// the browser sends it straight out with no preflight.
let inflight = null;

async function analyze(word) {
  const body = $('#pop-body');

  if (inflight) inflight.abort(); // moving between words: the last one wins
  const ctrl = (inflight = new AbortController());

  try {
    const res = await fetch(`${API}/${encodeURIComponent(word)}`, { signal: ctrl.signal });
    if (!res.ok) {
      // The API explains itself on 400, so show what it said.
      const err = await res.json().catch(() => null);
      body.innerHTML = `<p class="empty">${esc(
        (err && err.message) || `The analyzer returned ${res.status}.`)}</p>`;
      return null;
    }
    const data = await res.json();
    body.innerHTML = analysisHTML(data);
    return data;
  } catch (e) {
    if (e.name === 'AbortError') return null;
    body.innerHTML = '<p class="empty">Could not reach the analyzer — ' +
      'it may be asleep. Try again in a moment.</p>';
    return null;
  } finally {
    if (inflight === ctrl) inflight = null;
  }
}

/* ---------- lookups ---------- */

const CONTEXT = 180; // characters either side of the word

function contextOf(word) {
  const doc = currentDoc();
  const at = Number(word.dataset.at);
  if (!doc || !Number.isInteger(at)) return '';
  return doc.text
    .slice(Math.max(0, at - CONTEXT), at + word.textContent.length + CONTEXT)
    .replace(/\s+/g, ' ')
    .trim();
}

function gptPrompt(word, data) {
  const json = data ? JSON.stringify(data) : '';
  const context = contextOf(word);
  return [
    `I am reading Hebrew and want to understand the word ${word.textContent}` +
      (word.textContent === word.dataset.word ? '.'
        : ` (without niqqud: ${word.dataset.word}).`),
    context && `It appears here: ${context}`,
    json && `A morphological analyzer built on hspell returned this JSON: ${json}`,
    'In English, explain: what the word means in this context; its full ' +
      'morphology (root, binyan or mishkal, part of speech, gender, number, ' +
      'person, tense, and any prefixes or suffixes such as the conjunction ' +
      'vav, the definite article, prepositions or possessive endings); its ' +
      'dictionary form and how it inflects into this one; and anything the ' +
      'analyzer above got wrong, missed, or could not disambiguate. Be ' +
      'precise and concise.',
  ].filter(Boolean).join('\n\n');
}

// Pealim and Reverso index undotted spelling, so they get the normalized word;
// Google Translate reads the word as it stands in the text.
function setLookups(word, data) {
  const plain = encodeURIComponent(word.dataset.word);
  $('#link-reverso').href =
    `https://context.reverso.net/translation/hebrew-english/${plain}`;
  $('#link-translate').href = 'https://translate.google.com/?sl=iw&tl=en&op=translate' +
    `&text=${encodeURIComponent(word.textContent)}`;
  $('#link-pealim').href = `https://www.pealim.com/search/?q=${plain}`;
  // Hebrew costs nine URL characters a letter, so a long analysis can outgrow
  // what a browser will send; that one drops the JSON rather than the link.
  const gpt = (d) => `https://chatgpt.com/?prompt=${encodeURIComponent(gptPrompt(word, d))}`;
  const url = gpt(data);
  $('#link-gpt').href = url.length > 12000 ? gpt(null) : url;
}

/* ---------- popover ---------- */

const pop = $('#pop');
const CAN_ANCHOR = CSS.supports('anchor-name: --a');
const CAN_HOVER = matchMedia('(hover: hover) and (pointer: fine)');
const OPEN_DELAY = 160;  // ignore words merely swept past
const CLOSE_DELAY = 260; // long enough to cross the gap into the popover

let active = null;   // the word the popover currently describes
let anchored = null; // the word carrying anchor-name (outlives the fade)
let pinned = false;  // opened by click: stays until dismissed
let openTimer = null;
let closeTimer = null;

function clearActive() {
  if (!active) return;
  active.classList.remove('is-active');
  active = null;
}

// The anchor deliberately survives closing: the popover is still on screen
// during the fade, and dropping anchor-name mid-transition would unresolve
// position-anchor and fling it to the corner. Only a new word takes it over,
// and only ever one element at a time.
function anchorTo(word) {
  if (!CAN_ANCHOR || anchored === word) return;
  if (anchored) anchored.style.anchorName = '';
  word.style.anchorName = '--word';
  anchored = word;
}

// Switching words never closes and reopens — it re-anchors the open popover,
// so there is no flicker and no stray toggle event.
async function openPop(word, byClick, collect = byClick) {
  clearTimeout(openTimer);
  clearTimeout(closeTimer);
  pinned = byClick;

  if (active !== word) {
    clearActive();
    active = word;
    word.classList.add('is-active');
    anchorTo(word);

    $('#pop-word').textContent = word.dataset.word;
    $('#pop-body').innerHTML = '<p class="empty">Analyzing…</p>';
    setLookups(word, null); // usable before the analysis lands
  }

  if (!pop.matches(':popover-open')) pop.showPopover();
  if (!CAN_ANCHOR) place(word);

  const data = await analyze(word.dataset.word);
  if (data && word === active) setLookups(word, data);

  // Only a click collects: hovering across a line must not fill the glossary
  // with everything the pointer passed over.
  if (collect && data && currentId && addToGloss(currentId, word.dataset.word, data)) {
    renderGloss(currentId);
    markCollected();
  }
}

function closePop() {
  clearTimeout(openTimer);
  clearTimeout(closeTimer);
  pinned = false;
  if (pop.matches(':popover-open')) pop.hidePopover();
  else clearActive();
}

// Asking ":hover" at the deadline beats tracking enter/leave pairs, which race
// whenever the pointer crosses the gap between the word and the popover.
function scheduleClose() {
  clearTimeout(closeTimer);
  closeTimer = setTimeout(function check() {
    if (pinned) return;
    if (active && (active.matches(':hover') || pop.matches(':hover'))) {
      closeTimer = setTimeout(check, CLOSE_DELAY);
      return;
    }
    closePop();
  }, CLOSE_DELAY);
}

// Fallback for browsers without CSS anchor positioning: below the word, or
// above when it would not fit. Runs after showPopover so the box has a size.
function place(word) {
  const w = word.getBoundingClientRect();
  const p = pop.getBoundingClientRect();
  const gap = 7;

  const below = w.bottom + gap;
  const fitsBelow = below + p.height <= innerHeight - 4;
  pop.style.top = `${fitsBelow ? below : Math.max(4, w.top - p.height - gap)}px`;

  const ideal = w.left + w.width / 2 - p.width / 2;
  pop.style.left = `${Math.max(4, Math.min(ideal, innerWidth - p.width - 4))}px`;
}

// Deliberately popover="manual". An auto popover light-dismisses on
// pointerdown, so by the time a click lands the popover has already closed —
// leaving no way to tell "clicked the open word" from "clicked a new one".
// Owning dismissal keeps that decision unambiguous. The event still fires on
// our own show/hide, and is dispatched asynchronously, so ignore a close that
// something has already reopened.
pop.addEventListener('toggle', (e) => {
  if (e.newState === 'open' || pop.matches(':popover-open')) return;
  pinned = false;
  clearActive();
});

const readBody = $('#read-body');

readBody.addEventListener('click', (e) => {
  const word = e.target.closest('.w');
  if (!word) return;

  // Clicking a word already in the glossary takes it back out.
  const collected = !!currentId && !!glossOf(currentId)[word.dataset.word];
  if (collected) {
    removeFromGloss(currentId, word.dataset.word);
    renderGloss(currentId);
    markCollected();
  }

  // Clicking the word it is already pinned to closes it; clicking a word the
  // pointer merely opened by hovering pins it instead.
  if (word === active && pinned && pop.matches(':popover-open')) closePop();
  else openPop(word, true, !collected);
});

// The dismissal an auto popover would have given us.
document.addEventListener('click', (e) => {
  if (e.target.closest('.w') || e.target.closest('#pop')) return;
  closePop();
});

if (CAN_HOVER.matches) {
  readBody.addEventListener('mouseover', (e) => {
    const word = e.target.closest('.w');
    if (!word || pinned || word === active) return;
    clearTimeout(closeTimer); // moving to another word must not close
    clearTimeout(openTimer);
    openTimer = setTimeout(() => openPop(word, false), OPEN_DELAY);
  });
  readBody.addEventListener('mouseout', (e) => {
    if (!e.target.closest('.w')) return;
    clearTimeout(openTimer);
    scheduleClose();
  });
  pop.addEventListener('mouseenter', () => clearTimeout(closeTimer));
  pop.addEventListener('mouseleave', scheduleClose);
}

/* ---------- glossary ---------- */

const glossList = $('#gloss-list');
const glossExport = $('#gloss-export');
const glossCopy = $('#gloss-copy');

// The same sense usually turns up under several readings, so dedupe.
function sensesOf(data) {
  const seen = new Set();
  for (const split of data.splits || []) {
    for (const r of split.readings || []) {
      for (const g of r.glosses || []) seen.add(g);
    }
  }
  return [...seen];
}

// Underlines the collected words, so clicking one again to drop it reads as
// undoing something rather than as nothing happening.
function markCollected() {
  const entries = currentId ? glossOf(currentId) : {};
  for (const w of readBody.querySelectorAll('.w')) {
    w.classList.toggle('is-collected', !!entries[w.dataset.word]);
  }
}

// Newest first, so a word just clicked lands at the top.
const glossEntries = (id) =>
  Object.entries(glossOf(id)).sort((a, b) => b[1].added - a[1].added);

function renderGloss(id) {
  const entries = glossEntries(id);
  glossExport.disabled = glossCopy.disabled = !entries.length;

  if (!entries.length) {
    glossList.innerHTML =
      '<li class="empty">Click a word above to collect it here.</li>';
    return;
  }

  glossList.innerHTML = entries.map(([word, { data }]) => {
    const senses = sensesOf(data);
    return `<li class="gloss-item">
        <span class="gloss-word" dir="rtl" lang="he">${esc(word)}</span>
        <span class="gloss-sense">${senses.length
          ? esc(senses.join(', '))
          : '<span class="empty">no gloss recorded</span>'}</span>
      </li>`;
  }).join('');
}

/* ---------- export ---------- */

function glossMarkdown(doc) {
  const entries = glossEntries(doc.id);
  const when = new Date().toISOString().slice(0, 10);
  const lines = [
    `# Glossary — ${doc.title}`,
    '',
    `${entries.length} word${entries.length === 1 ? '' : 's'} · exported ${when}`,
    '',
  ];

  for (const [word, { data }] of entries) {
    lines.push(`## ${word}`, '');
    if (data.gimatria) lines.push(`Hebrew numeral · **${data.gimatria}**`, '');

    if (!data.splits || !data.splits.length) {
      lines.push('- No analysis found for this word.', '');
      continue;
    }
    for (const split of data.splits) {
      lines.push(`- ${split.whole_word
        ? `**${split.base}**`
        : `**${split.prefix}** + **${split.base}**`}`);
      for (const r of split.readings || []) {
        const bits = [`**${r.stem}**`];
        const features = featureList(r.features).join(', ');
        if (features) bits.push(`_${features}_`);
        if (r.glosses && r.glosses.length) bits.push(r.glosses.join(', '));
        lines.push(`  - ${bits.join(' — ')}`);
      }
    }
    lines.push('');
  }
  return lines.join('\n');
}

// A text titled only in Hebrew leaves nothing behind, so fall back to its id.
const slug = (title, id) =>
  title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || id;

function download(name, text) {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/markdown' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

const currentDoc = () => currentId && loadAll().find((t) => t.id === currentId);

glossExport.addEventListener('click', () => {
  const doc = currentDoc();
  if (!doc) return;
  download(`hebmorph-glossary-${slug(doc.title, doc.id)}.md`, glossMarkdown(doc));
});

glossCopy.addEventListener('click', async () => {
  const doc = currentDoc();
  if (!doc) return;
  try {
    await navigator.clipboard.writeText(glossMarkdown(doc));
    glossCopy.classList.add('is-done');
    setTimeout(() => glossCopy.classList.remove('is-done'), 1400);
  } catch {
    glossCopy.title = 'Could not reach the clipboard';
  }
});

/* ---------- views ---------- */

function renderList(activeId) {
  const list = loadAll();
  const ul = $('#text-list');

  if (!list.length) {
    ul.innerHTML = '<li class="sidebar-foot">No texts yet.</li>';
    return;
  }
  ul.replaceChildren(...list.map(({ id, title }) => {
    const li = document.createElement('li');
    const a = document.createElement('a');
    a.className = 'text-link';
    a.href = `#/t/${id}`;
    a.dir = 'auto';
    a.textContent = title;
    if (id === activeId) a.setAttribute('aria-current', 'page');
    li.append(a);
    return li;
  }));
}

let currentId = null; // the text on screen, and so the glossary being filled

function route() {
  const id = location.hash.startsWith('#/t/') ? location.hash.slice(4) : null;
  const doc = id && loadAll().find((t) => t.id === id);

  if (id && !doc) { location.hash = '#/'; return; } // stale link

  closePop();
  anchored = null; // the words it pointed at are about to be replaced
  currentId = doc ? doc.id : null;
  $('#view-new').hidden = !!doc;
  $('#view-read').hidden = !doc;
  $('#view-gloss').hidden = !doc;

  if (doc) {
    $('#read-title').textContent = doc.title;
    renderText(readBody, doc.text);
    renderGloss(doc.id);
    markCollected();
  }

  renderList(doc ? doc.id : null);

  // Leave the desktop sidebar as the reader left it; only the mobile drawer
  // is transient.
  if (!matchMedia(DESKTOP).matches) document.body.classList.remove('nav-open');
  window.scrollTo(0, 0);
}

/* ---------- events ---------- */

$('#new-form').addEventListener('submit', (e) => {
  e.preventDefault();
  const text = $('#new-text').value.trim();
  if (!text) return;

  const doc = { id: newId(), title: titleOf(text), text, created: Date.now() };
  saveAll([doc, ...loadAll()]);
  $('#new-text').value = '';
  location.hash = `#/t/${doc.id}`;
});

$('#nav-toggle').addEventListener('click', () => {
  const open = document.body.classList.toggle('nav-open');
  $('#nav-toggle').setAttribute('aria-expanded', String(open));
});
$('#scrim').addEventListener('click', () => document.body.classList.remove('nav-open'));

document.addEventListener('keydown', (e) => {
  if (e.key !== 'Escape') return;
  document.body.classList.remove('nav-open');
  closePop(); // manual popovers do not dismiss themselves
});

/* ---------- start ---------- */

if (matchMedia(DESKTOP).matches) document.body.classList.add('nav-open');
window.addEventListener('hashchange', route);
route();
