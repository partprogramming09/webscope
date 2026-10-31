// webscope page helper. Served by the local listener and loaded by the one-line bootstrap that page(load) returns.
// Every action returns a short string; failures start with "Failed:" and name the next step.
(() => {
  // Loading again (the MCP restarted: new port, new token) points the helper at the new listener; the wrappers are installed once.
  const src = document.currentScript.src;
  const cfg = (window.__wsCfg = window.__wsCfg || {});
  cfg.base = src.slice(0, src.indexOf('/page.js'));
  cfg.t = new URL(src).searchParams.get('t');
  cfg.o = new URL(src).searchParams.get('o'); // the address WEBSCOPE_APP_URL names: the only origin the listener answers
  window.__wsVer = new URL(src).searchParams.get('v'); // the bootstrap refuses to run over a helper of another version
  if (window.__ws) return;
  const CAP = 3000;
  const SENT = 8000; // chars of each body reported (check compares them): request and answer together stay under the 64 KiB a keepalive fetch may carry, even at 3 bytes a character

  const norm = s => String(s == null ? '' : s).replace(/\s+/g, ' ').trim();
  const lower = s => norm(s).toLowerCase();
  const clip = (s, n) => (s.length > n ? s.slice(0, n) + '…(+' + (s.length - n) + ' chars)' : s);
  const fail = m => 'Failed: ' + m;
  const vis = e => !!(e.offsetWidth || e.offsetHeight || e.getClientRects().length);
  const all = sel => [...document.querySelectorAll(sel)].filter(vis);
  const kind = v => '[' + Object.prototype.toString.call(v).slice(8, -1) + ']';
  const textOf = e => norm(e.innerText || e.value || e.getAttribute('aria-label') || e.title);
  const describe = e => '<' + e.tagName.toLowerCase() + (e.id ? '#' + e.id : '') + '> "' + clip(textOf(e), 40) + '"';

  // ---- what the page sends: fetch and XMLHttpRequest are wrapped, and each request is reported to the listener
  const realFetch = window.fetch.bind(window);
  const log = []; // the last 200 requests with their status, for the effect of an action
  let seq = 0, pending = 0;
  const report = (c, counted) => {
    if (counted) pending = Math.max(0, pending - 1);
    try {
      const u = new URL(c.u, location.href);
      log.push({ i: seq++, t: c.m + ' ' + u.pathname + ' ' + (c.o ? 'opaque (no-cors: its status cannot be read)' : c.s === 0 ? '0 (no answer)' : c.s) }); // no query string: it can hold a token, and payloads still shows the whole address
      if (log.length > 200) log.shift();
    } catch (e) { /* an address that does not parse is left out of the log */ }
    try {
      c.req = clip(c.req, SENT);
      c.res = clip(c.res, SENT);
      realFetch(cfg.base + '/capture?t=' + cfg.t, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(c), keepalive: true }).catch(() => {});
    } catch (e) { /* the capture is lost, the page keeps working */ }
  };

  const sims = [];
  const takeSim = (m, u) => {
    const i = sims.findIndex(s => u.includes(s.match) && (!s.method || s.method === m));
    if (i < 0) return null;
    const s = sims[i];
    if (--s.times <= 0) sims.splice(i, 1);
    return s;
  };
  const SIM_BODY = '{"message":"simulated by webscope"}';

  window.fetch = async function (input, init) {
    const isReq = typeof Request !== 'undefined' && input instanceof Request;
    const u = String(isReq ? input.url : input);
    if (u.startsWith(cfg.base)) return realFetch(input, init);
    const m = String((init && init.method) || (isReq && input.method) || 'GET').toUpperCase();
    const b = init && init.body;
    const req = typeof b === 'string' ? b : b == null ? '' : kind(b);
    const sim = takeSim(m, u);
    if (sim) {
      report({ m, u, s: sim.status, req, res: SIM_BODY });
      return new Response(SIM_BODY, { status: sim.status, headers: { 'Content-Type': 'application/json' } });
    }
    let res;
    pending++;
    try {
      res = await realFetch(input, init);
    } catch (e) {
      report({ m, u, s: 0, req, res: String(e) }, true);
      throw e;
    }
    res.clone().text().then(res_ => report({ m, u, s: res.status, req, res: res_, o: res.type === 'opaque' || res.type === 'opaqueredirect' }, true)).catch(() => { pending = Math.max(0, pending - 1); });
    return res;
  };

  // A simulated XMLHttpRequest is sent to the listener's /sim instead, so the app sees a real response with that status.
  const xhrProto = Object.getPrototypeOf(new XMLHttpRequest()); // the prototype the instances really have: a page that replaced the constructor may give it another one
  const xhrOpen = xhrProto.open;
  const xhrSend = xhrProto.send;
  const xhrNote = (x, m, u) => { x.__ws = { m: String(m).toUpperCase(), u: String(u) }; };
  const xhrWatch = (x, body, reopen) => {
    const c = x.__ws;
    if (c && !x.__wsSeen && !c.u.startsWith(cfg.base)) {
      x.__wsSeen = true; // a send that reaches both hooks is reported once
      c.req = typeof body === 'string' ? body : body == null ? '' : kind(body);
      const sim = takeSim(c.m, c.u);
      if (sim) {
        reopen(c.m, cfg.base + '/sim?t=' + cfg.t + '&status=' + sim.status);
        x.withCredentials = false;
      }
      pending++;
      x.addEventListener('loadend', () => {
        const text = x.responseType === '' || x.responseType === 'text' ? x.responseText : kind(x.response);
        report({ m: c.m, u: c.u, s: x.status, req: c.req, res: text }, true);
      });
    }
  };
  xhrProto.open = function (m, u) {
    xhrNote(this, m, u);
    return xhrOpen.apply(this, arguments);
  };
  xhrProto.send = function (body) {
    xhrWatch(this, body, (m, u) => xhrOpen.call(this, m, u, true));
    return xhrSend.apply(this, arguments);
  };
  // A page that replaced the constructor (the legacy pages do) gives each instance its own open and send, which call the native ones the page saved
  // and skip the prototype: those instances are hooked here, once, when they are made.
  const PageXHR = window.XMLHttpRequest;
  if (!/\[native code\]/.test(Function.prototype.toString.call(PageXHR))) {
    const own = (x, k) => Object.prototype.hasOwnProperty.call(x, k) && typeof x[k] === 'function';
    window.XMLHttpRequest = function () {
      const x = new PageXHR();
      const open = own(x, 'open') ? x.open : null;
      if (open) x.open = function (m, u) { xhrNote(x, m, u); return open.apply(x, arguments); };
      if (own(x, 'send')) {
        const send = x.send;
        x.send = function (body) { xhrWatch(x, body, (m, u) => (open || xhrOpen).call(x, m, u, true)); return send.apply(x, arguments); };
      }
      return x;
    };
    window.XMLHttpRequest.prototype = PageXHR.prototype;
    Object.setPrototypeOf(window.XMLHttpRequest, PageXHR);
  }

  // ---- what the page logs from now on: console.error and console.warn, uncaught errors and rejected promises
  const clog = [];
  let cseq = 0;
  const noQuery = s => String(s).replace(/\?[^\s)|]*/g, '');
  const show = a => { if (a instanceof Error) return a.message; if (typeof a === 'string') return a; try { return JSON.stringify(a); } catch (e) { return String(a); } };
  const logLine = (level, args) => {
    clog.push({ i: cseq++, t: level + ': ' + clip(noQuery(args.map(show).join(' ')), 200) });
    if (clog.length > 100) clog.shift();
  };
  ['error', 'warn'].forEach(k => { const o = console[k].bind(console); console[k] = (...a) => { logLine(k, a); o(...a); }; });
  window.addEventListener('error', e => logLine('uncaught', [String(e.message).replace(/^Uncaught /, '')]));
  window.addEventListener('unhandledrejection', e => logLine('rejected', [e.reason && e.reason.message ? e.reason.message : e.reason]));

  // ---- reading the screen
  const labelOf = e => {
    if (e.labels && e.labels.length && norm(e.labels[0].textContent)) return norm(e.labels[0].textContent); // an empty label names nothing: the placeholder or the name still can
    const box = e.closest('.v-input, .v-field'); // Vuetify keeps the label next to the native input
    const l = box && box.querySelector('label');
    return norm((l && l.textContent) || e.getAttribute('aria-label') || e.placeholder || e.name || e.id || (e.isContentEditable ? 'editor' : ''));
  };
  const FIELDS = 'input:not([type=hidden]):not([type=button]):not([type=submit]):not([type=reset]),textarea,select,[contenteditable=true],[contenteditable=""]';
  const fields = () => all(FIELDS);
  const valueOf = e => {
    if (e.isContentEditable) return e.innerText.replace(/\n$/, '');
    if (e.type === 'checkbox' || e.type === 'radio') return e.checked ? 'on' : 'off';
    if (e.tagName === 'SELECT') return norm(e.selectedOptions[0] && e.selectedOptions[0].text);
    return e.value;
  };
  const texts = sel => [...new Set(all(sel).map(e => norm(e.innerText)).filter(Boolean))];
  const CLICKABLE = 'button,a,[role=button],[role=tab],[role=menuitem],[role=option],input[type=submit],input[type=button],.v-btn,.v-list-item';
  const disabled = e => e.disabled || e.getAttribute('aria-disabled') === 'true' || /--disabled/.test(e.className);
  // the dialog's buttons first, the page's next, the side menu and the top bar last: they come first in the DOM and would fill the 30 places
  const rank = e => e.closest('[role=dialog],.v-dialog') ? 0 : e.closest('.v-pagination') ? 1 : e.closest('nav,aside,header,[role=navigation],.v-navigation-drawer,.v-app-bar,.v-toolbar') ? 2 : 1; // a pager is a nav too, but it belongs to the page
  const ALERTS ='[role=alert],.v-alert,.v-snackbar__content,.alert,.toast';
  const ERRORS = '.v-messages__message,.invalid-feedback,.error--text,.text-danger';

  const wrongOrigin = () => cfg.o && !cfg.o.split(',').includes(location.origin) ? 'This tab is ' + location.origin + ' but WEBSCOPE_APP_URL is ' + cfg.o.split(',').join(', ') + ': captures and simulate are refused. Open the app at that address.' : '';

  // showAddress is an address as the answers show it: parameter names only, because a value can be a token and a URL with a query is what output filters cut.
  const showAddress = l => {
    const names = [...new Set([...new URLSearchParams(l.search).keys()])].join(',');
    return l.pathname + (names ? ' ?' + names : '') + (l.hash ? ' ' + l.hash.split('?')[0] : '');
  };
  const NOT_FOUND = /(^|\D)404(\D|$)|not found|no (se )?encontrad[ao]|p[aá]gina no (existe|encontrada)/i;
  // lines keeps whole lines up to max characters and says how many it left out, instead of cutting one in the middle.
  const lines = (l, max) => {
    let n = 0, i = 0;
    while (i < l.length && n + l[i].length + 1 <= max) n += l[i++].length + 1;
    if (i === 0) return clip(l[0], max);
    return i === l.length ? l.join('\n') : l.slice(0, i).join('\n') + '\n(… ' + (l.length - i) + ' more lines not shown: raise max)';
  };

  const state = a => {
    const out = ['URL: ' + showAddress(location), 'Title: ' + norm(document.title)];
    if (NOT_FOUND.test([document.title, ...texts('h1,h2,h3')].join(' '))) out.unshift('Looks like a 404 page of the app (the title or a heading says not found): this is not the screen asked for. Next: open the right address.');
    if (wrongOrigin()) out.push('Warning: ' + wrongOrigin());
    const h = texts('h1,h2,h3').slice(0, 8);
    if (clog.length) out.push('Console (last ' + Math.min(5, clog.length) + ' of ' + clog.length + ' since the helper loaded): ' + clog.slice(-5).map(e => e.t).join(' | '));
    if (h.length) out.push('Headings: ' + h.join(' | '));
    const f = fields();
    if (f.length) {
      out.push('Fields:');
      f.forEach(e => out.push('  [' + (e.isContentEditable ? 'editor' : e.type || e.tagName.toLowerCase()) + '] ' + (labelOf(e) || '(no label)') + ' = "' + valueOf(e).replace(/\n/g, '⏎') + '"'));
    }
    const b = [...new Set(all(CLICKABLE).filter(textOf).map((e, i) => ({ i, t: textOf(e) + (disabled(e) ? ' (disabled)' : ''), r: rank(e) })).sort((x, y) => x.r - y.r || x.i - y.i).map(x => x.t))].slice(0, 30);
    if (b.length) out.push('Buttons (dialog first, side menu last): ' + b.join(' | '));
    const al = texts(ALERTS);
    if (al.length) out.push('Alerts: ' + al.join(' | '));
    const er = texts(ERRORS);
    if (er.length) out.push('Errors: ' + er.join(' | '));
    const dlg = all('[role=dialog],dialog[open]')[0];
    if (dlg) out.push('Dialog: ' + clip(norm(dlg.innerText), 200));
    const tables = all('table').map(t => t.querySelectorAll('tbody tr').length);
    if (tables.length) out.push('Tables (rows): ' + tables.join(', '));
    const foot = texts('.v-data-table-footer__info,.v-pagination__info');
    if (foot.length) out.push('Footer: ' + foot.join(' | '));
    return lines(out, a.max || CAP);
  };

  const read = a => {
    let e;
    try { e = a.selector ? document.querySelector(a.selector) : document.body; } catch (x) { return fail('bad selector ' + a.selector); }
    if (!e) return fail('nothing matches ' + a.selector + '. Next: page(state) shows what is on the screen.');
    return clip(norm(e.innerText != null ? e.innerText : e.textContent), a.max || CAP);
  };

  // ---- acting on the screen
  const findFields = name => {
    const n = lower(name), f = fields().sort((a, b) => !!b.closest('[role=dialog],.v-dialog') - !!a.closest('[role=dialog],.v-dialog'));
    return [f.filter(e => lower(labelOf(e)) === n), f.filter(e => [e.name, e.id, e.placeholder].some(x => x && lower(x) === n)), f.filter(e => lower(labelOf(e)).includes(n))].find(l => l.length) || [];
  };
  const idOf = e => e.id ? '#' + e.id : e.name ? '[name=' + e.name + ']' : '"' + labelOf(e) + '"';
  // Vuetify menus: v-select/v-autocomplete show the option text, keep a code in the input and render only the rows in view.
  const MENU = '.v-select,.v-autocomplete,.v-combobox';
  const sleep = ms => new Promise(r => setTimeout(r, ms));
  const rows = () => all('.v-overlay-container .v-list-item');
  const shown = e => { const c = all('.v-field *').filter(x => e.closest('.v-field').contains(x) && /__selection/.test(x.className)).map(textOf).filter((s, i, a) => s && a.indexOf(s) === i); return c.length ? c.join(', ') : e.value; };
  const choose = async (e, v) => {
    const field = e.closest('.v-field'), want = lower(v), late = !!e.closest('.v-autocomplete,.v-combobox');
    for (let i = 0; i < 14 && rows().length; i++) await sleep(70); // the previous menu may still be leaving
    ['pointerdown', 'mousedown', 'pointerup', 'mouseup', 'click'].forEach(t => field.dispatchEvent(new MouseEvent(t, { bubbles: true, cancelable: true, view: window })));
    const type = s => { Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(e, s); e.dispatchEvent(new Event('input', { bubbles: true })); };
    if (late) type(String(v)); // the search is the option text; the answer comes later
    const match = () => { const r = rows(), eq = r.filter(i => lower(i.innerText) === want || lower(i.innerText.split('\n')[0]) === want), inc = r.filter(i => lower(i.innerText).includes(want)); return eq[0] || (inc.length === 1 && inc[0]); };
    const seen = new Set();
    const look = async () => {
      let hit, list, end = 0;
      for (let i = 0; i < 400 && !hit; i++) { // shortcut: 400 steps of 70 ms at most (a list of 4000 rows), then it gives up
        await sleep(70);
        rows().forEach(r => seen.add(norm(r.innerText)));
        hit = match();
        list = [...document.querySelectorAll('.v-overlay-container .v-list')].pop();
        if (hit || i < (late ? 25 : 4)) continue; // an autocomplete asks the server: give it about two seconds
        if (!list) { if (i > 30) break; continue; } // no menu appeared: two more seconds, then give up
        if (list.scrollTop + list.clientHeight >= list.scrollHeight - 2) { if (++end > 2) break; } // the end of the list: look three more times, then give up
        else list.scrollTop += list.clientHeight * 0.8;
      }
      return hit;
    };
    let hit = await look();
    const first = String(v).trim().split(/\s+/)[0];
    if (!hit && late && !seen.size && first !== String(v).trim()) { type(first); hit = await look(); } // the server found nothing for the whole text: search its first word
    if (!hit) { document.body.click(); throw new Error('no option "' + v + '" among ' + seen.size + ' seen: ' + [...seen].slice(0, 5).join(' | ')); }
    const text = norm(hit.innerText);
    if (!hit.classList.contains('v-list-item--active') || !e.closest('.v-select--multiple')) hit.click(); // multiple: choosing adds, so one already chosen is left alone
    await sleep(300);
    document.body.click();
    await sleep(300);
    return text;
  };
  // listOptions is the text of the choices of a field: all of a native select; of a menu, the rows seen while it scrolls (and, in an autocomplete, searching match), until max rows contain match.
  const listOptions = async (e, match, max) => {
    const want = lower(match || ''), keep = l => l.filter(s => lower(s).includes(want));
    if (e.tagName === 'SELECT') return [...e.options].map(o => norm(o.text)).filter(s => lower(s).includes(want));
    if (!e.closest(MENU)) throw new Error('is not a select or a menu');
    const field = e.closest('.v-field'), late = !!e.closest('.v-autocomplete,.v-combobox');
    for (let i = 0; i < 14 && rows().length; i++) await sleep(70);
    ['pointerdown', 'mousedown', 'pointerup', 'mouseup', 'click'].forEach(t => field.dispatchEvent(new MouseEvent(t, { bubbles: true, cancelable: true, view: window })));
    if (late && match) { Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(e, String(match)); e.dispatchEvent(new Event('input', { bubbles: true })); }
    const seen = new Set();
    let end = 0;
    for (let i = 0; i < 400 && keep([...seen]).length < max; i++) { // shortcut: 400 steps of 70 ms at most, as fill
      await sleep(70);
      rows().forEach(r => seen.add(norm(r.innerText.split('\n')[0])));
      const list = [...document.querySelectorAll('.v-overlay-container .v-list')].pop();
      if (i < (late ? 25 : 4)) continue;
      if (!list) { if (i > 30) break; continue; }
      if (list.scrollTop + list.clientHeight >= list.scrollHeight - 2) { if (++end > 2) break; }
      else list.scrollTop += list.clientHeight * 0.8;
    }
    document.body.click();
    await sleep(300);
    return keep([...seen]);
  };
  const options = async a => {
    if (!a.field) return fail('options needs field, the label, name or id of a select or menu, e.g. options {"field":"Ciudad","match":"AAC"}');
    const e = findFields(String(a.field))[0];
    if (!e) return fail('no field «' + a.field + '». Next: page(state) lists the fields');
    const max = Number(a.max) > 0 ? Number(a.max) : 30;
    let list;
    try { list = await listOptions(e, a.match, max); } catch (x) { return fail('«' + a.field + '» ' + x.message); }
    return a.field + ': ' + (list.slice(0, max).join(' | ') || '(none)') + (list.length > max ? ' (… ' + (list.length - max) + ' more: narrow with match, or raise max)' : '');
  };

  const setValue = async (e, v) => {
    e.focus();
    if (e.closest(MENU)) return choose(e, v);
    if (e.isContentEditable) { // a rich-text editor has no value setter: select its content and insert the text, as typing would
      const r = document.createRange();
      r.selectNodeContents(e);
      getSelection().removeAllRanges();
      getSelection().addRange(r);
      document.execCommand(String(v) === '' ? 'delete' : 'insertText', false, String(v));
      e.blur();
      return;
    }
    if (e.type === 'checkbox' || e.type === 'radio') {
      const want = v === true || /^(true|on|1|yes|si|sí)$/i.test(String(v));
      if (e.checked !== want) e.click();
    } else {
      if (e.tagName === 'SELECT') {
        const o = [...e.options].find(o => o.value === String(v) || norm(o.text) === norm(v));
        if (!o) throw new Error('no option "' + v + '" among ' + e.options.length + ' seen: ' + [...e.options].slice(0, 5).map(o => norm(o.text)).join(' | '));
        e.value = o.value;
      } else {
        const proto = e.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
        Object.getOwnPropertyDescriptor(proto, 'value').set.call(e, String(v)); // the setter the framework listens to
      }
      e.dispatchEvent(new Event('input', { bubbles: true }));
      e.dispatchEvent(new Event('change', { bubbles: true }));
    }
    e.blur();
  };
  const fill = async a => {
    const fields = a.fields && typeof a.fields === 'object' ? a.fields : a;
    if (!Object.keys(fields).length) return fail('fill needs fields {label: value}, e.g. fill {"Nombre":"Ana","Acepto":true}');
    let miss = false;
    const out = [], notes = [];
    for (const [k, v] of Object.entries(fields)) {
      const found = findFields(k), e = found[0];
      notes.push(found.length > 1 ? ' (warning: ' + found.length + ' fields answer to this name; filled ' + idOf(e) + ', the others are ' + found.slice(1, 4).map(idOf).join(', ') + '. Name one by its id to be sure)' : '');
      if (!e) { miss = true; out.push(k + ': not found'); continue; }
      try {
        const clicked = await setValue(e, v);
        const got = valueOf(e), want = typeof v === 'boolean' ? (v ? 'on' : 'off') : String(v);
        if (clicked) { // a menu: the readback is the text it shows, which holds the option (an autocomplete shows only its first line)
          const now = lower(shown(e));
          if (now && (lower(clicked).includes(now) || now.includes(lower(want)))) { out.push(k + ': ok'); continue; }
          miss = true; out.push(k + ': the page shows "' + shown(e) + '" instead of "' + want + '"'); continue;
        }
        if (e.type === 'checkbox' || e.type === 'radio' || e.tagName === 'SELECT' || got === want) { out.push(k + ': ok'); continue; }
        miss = true; // a v-select keeps its model and puts it back: the field is a menu, so click it and then the option
        out.push(k + ': the page shows "' + got + '" instead of "' + want + '"');
      } catch (x) { miss = true; out.push(k + ': ' + x.message); }
    }
    const lines = out.map((l, i) => l + notes[i]);
    if (miss) lines.push('Next: page(state) lists the fields and their labels.');
    return lines.join('\n');
  };

  // Runs `f` (a click or a key) and appends what it caused: the requests with their status, the new alerts and field errors, a URL change, or that nothing changed.
  const withEffect = async f => {
    const url = () => location.pathname + location.search + location.hash;
    const before = { seq, cseq, alerts: new Set(texts(ALERTS)), errors: new Set(texts(ERRORS)), url: url(), text: norm(document.body.innerText) };
    let mark = '';
    try { const m = await realFetch(cfg.base + '/logmark?t=' + cfg.t); mark = m.ok ? await m.text() : ''; } catch (e) { /* no listener: no log lines */ }
    const r = f();
    if (/^Failed:/.test(r)) return r;
    await sleep(400);
    for (let i = 0; i < 30 && pending > 0; i++) await sleep(70); // a slow answer: about two more seconds
    await sleep(150); // the screen draws the answer
    const out = [r], reqs = log.filter(e => e.i >= before.seq).map(e => e.t);
    if (reqs.length) out.push('Requests: ' + reqs.join(' | '));
    if (pending > 0) out.push('Still pending: ' + pending + ' request(s) had not answered when this was written (it waited about 2.5 s), so the effect may be incomplete. Next: page(payloads) later shows how they ended.');
    if (reqs.some(r => / (0 \(no answer\)|5\d\d)$/.test(r))) { // a request with no answer or a 5xx: the backend log may say why
      try { const r = await realFetch(cfg.base + '/log?t=' + cfg.t + '&from=' + mark); const t = r.ok ? await r.text() : ''; if (t) out.push('Backend log (the last error lines of WEBSCOPE_LOG, as the log wrote them):\n' + t); } catch (e) { /* the listener is gone: no log lines */ }
    }
    const na = texts(ALERTS).filter(x => !before.alerts.has(x));
    if (na.length) out.push('New alerts: ' + na.join(' | '));
    const ne = texts(ERRORS).filter(x => !before.errors.has(x));
    if (ne.length) out.push('New errors: ' + ne.join(' | '));
    const nc = clog.filter(e => e.i >= before.cseq).map(e => e.t);
    if (nc.length) out.push('New console errors: ' + nc.join(' | '));
    if (url() !== before.url) out.push('URL: ' + showAddress(new URL(before.url, location.origin)) + ' → ' + showAddress(location));
    if (out.length === 1) out.push(norm(document.body.innerText) === before.text ? 'No effect seen: no request, no new alert, the screen text is the same.' : 'The screen text changed; no request, alert or URL change.');
    if (wrongOrigin()) out.push('Warning: ' + wrongOrigin());
    return out.join('\n');
  };

  const clickOnce = a => {
    let els;
    if (a.selector) {
      try { els = all(a.selector); } catch (x) { return fail('bad selector ' + a.selector); }
    } else if (a.text) {
      const t = lower(a.text), c = all(CLICKABLE);
      els = c.filter(e => lower(textOf(e)) === t);
      if (!els.length) els = c.filter(e => lower(textOf(e)).includes(t));
      if (!els.length) els = all('body *').filter(e => !e.children.length && lower(textOf(e)) === t); // a confirmation whose button is a plain span or div
      if (els.length > 1) return fail(els.length + ' elements match "' + a.text + '": ' + els.slice(0, 6).map(describe).join(', ') + '. Next: click with a selector.');
    } else return fail('click needs text or selector');
    if (!els.length) return fail('nothing to click for "' + (a.selector || a.text) + '". Buttons here: ' + [...new Set(all(CLICKABLE).filter(textOf).map((e, i) => ({ t: textOf(e), i, r: rank(e) })).sort((x, y) => x.r - y.r || x.i - y.i).map(x => x.t))].slice(0, 12).join(' | '));
    els[0].click();
    return 'clicked ' + describe(els[0]);
  };

  const pressOnce = a => {
    if (!a.key) return fail('press needs key');
    let e;
    try { e = a.selector ? document.querySelector(a.selector) : document.activeElement || document.body; } catch (x) { return fail('bad selector ' + a.selector); }
    if (!e) return fail('nothing matches ' + a.selector);
    e.focus();
    const init = { key: a.key, code: a.code || a.key, bubbles: true, cancelable: true };
    const go = e.dispatchEvent(new KeyboardEvent('keydown', init));
    e.dispatchEvent(new KeyboardEvent('keypress', init));
    e.dispatchEvent(new KeyboardEvent('keyup', init));
    if (a.key === 'Enter' && go && e.tagName === 'INPUT' && e.form) e.form.requestSubmit(); // a synthetic Enter does not submit by itself
    return 'pressed ' + a.key + ' on ' + describe(e);
  };

  const click = a => withEffect(() => clickOnce(a));
  const press = a => withEffect(() => pressOnce(a));

  // routerOf is the Vue Router of the app in this page, or undefined.
  const routerOf = () => {
    const e = document.querySelector('#app') || [...document.querySelectorAll('body *')].find(n => n.__vue_app__);
    return e && e.__vue_app__ && e.__vue_app__.config.globalProperties.$router;
  };

  // open goes to a router path through the app's own router, so the base (such as a tenant prefix) is added by the app; a path no route owns is refused with the similar ones.
  const open = a => {
    const path = String(a.path || '');
    if (path[0] !== '/') return fail('open needs path, a router path that starts with /, such as /crm/quotes. Next: take it from code map');
    const r = routerOf();
    if (!r) return fail('no Vue Router in this page. Next: navigate to the full address, with the tenant prefix if the app has one');
    const res = r.resolve(path);
    if (!res.matched.length || res.matched.some(m => /pathMatch/.test(m.path))) {
      const word = (path.split('/').filter(Boolean).pop() || '').replace(/[^\w-]/g, '');
      const near = word ? r.getRoutes().map(x => x.path).filter(p => p.includes(word)).slice(0, 8) : [];
      return fail('no route «' + path + '» in this app' + (near.length ? '; similar: ' + near.join(' | ') : '') + '. Next: use a path from code map');
    }
    return r.push(path).then(() => {
      const now = r.currentRoute.value.fullPath;
      return 'opened ' + now + (now === res.fullPath ? '' : ' (redirected from ' + res.fullPath + ': session or permission?)');
    });
  };

  const simulate = a => {
    if (a.clear) { sims.length = 0; return 'simulations cleared'; }
    const status = Number(a.status);
    if (!a.match || !(status >= 400 && status <= 599)) return fail('simulate needs match (part of the URL) and status 400-599');
    const times = Number(a.times) || 1;
    sims.push({ match: String(a.match), status, times, method: a.method ? String(a.method).toUpperCase() : '' });
    return 'the next ' + times + ' request(s) with "' + a.match + '" answer ' + status;
  };

  // ---- facts the page itself shows about its own screen, with no reference screen to compare to
  const where = e => {
    if (e.id && document.querySelectorAll('#' + CSS.escape(e.id)).length === 1) return '#' + CSS.escape(e.id);
    const p = [];
    for (let n = e; n && n.nodeType === 1 && n !== document.body && p.length < 4; n = n.parentElement) {
      let s = n.tagName.toLowerCase();
      const sib = n.parentElement ? [...n.parentElement.children].filter(c => c.tagName === n.tagName) : [];
      if (sib.length > 1) s += ':nth-of-type(' + (sib.indexOf(n) + 1) + ')';
      p.unshift(s);
    }
    return p.join(' > ');
  };
  const tagId = e => '<' + e.tagName.toLowerCase() + (e.id ? '#' + e.id : '') + '>';
  const audit = a => {
    let root = document;
    if (a.selector) {
      try { root = document.querySelector(a.selector); } catch (x) { return fail('bad selector ' + a.selector); }
      if (!root) return fail('nothing matches ' + a.selector);
    }
    const q = sel => [...root.querySelectorAll(sel)].filter(vis);
    const out = [];
    const add = (name, items) => { if (items.length) out.push(name + ' (' + items.length + '): ' + items.slice(0, 5).join(' | ') + (items.length > 5 ? ' | … +' + (items.length - 5) : '')); };
    // text the framework or the server left as it was written
    const bad = [], walker = document.createTreeWalker(root === document ? document.body : root, NodeFilter.SHOW_TEXT);
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const p = n.parentElement, t = norm(n.nodeValue);
      if (!t || !p || p.closest('script,style,noscript,textarea,template') || !vis(p)) continue;
      if (/\{\{.*?\}\}|\[object Object\]|(^|[^\w.])(undefined|NaN)([^\w]|$)|\$t\(/.test(t) || /^(null|undefined|NaN)$/.test(t)) bad.push(tagId(p) + ' "' + clip(t, 40) + '"');
    }
    add('unrendered', bad);
    // a control that something else sits on: the element at its centre is not it, nor inside it, nor its label
    const MODAL = '[role=dialog],.v-overlay,.v-menu,.v-tooltip,.modal';
    const covered = [];
    new Set(q(FIELDS + ',' + CLICKABLE)).forEach(e => {
      const r = e.getBoundingClientRect();
      if (r.width < 2 || r.height < 2 || r.bottom < 0 || r.right < 0 || r.top > innerHeight || r.left > innerWidth) return;
      const top = document.elementFromPoint(Math.min(innerWidth - 1, r.left + r.width / 2), Math.min(innerHeight - 1, r.top + r.height / 2));
      if (!top || e.contains(top) || top.contains(e) || (e.labels && [...e.labels].some(l => l.contains(top)))) return;
      if (top.closest(MODAL) && !e.closest(MODAL)) return; // a modal over the page is expected
      const cx = r.left + r.width / 2, cy = r.top + r.height / 2;
      for (let n = e.parentElement; n && n !== document.body; n = n.parentElement) { // scrolled out of a container that clips: out of view, not covered
        if (getComputedStyle(n).overflow !== 'visible') { const b = n.getBoundingClientRect(); if (cx < b.left || cx > b.right || cy < b.top || cy > b.bottom) return; }
      }
      const own = e.closest('.v-input,.v-field,label,.select2-container');
      if (own && own.contains(top)) return; // the wrapper of the control itself, which sits over its native input
      covered.push(tagId(e) + ' "' + clip(textOf(e) || labelOf(e), 30) + '" covered by ' + tagId(top));
    });
    add('covered', covered);
    // fields a person or `fill` cannot name
    const real = e => norm([...(e.labels || [])].map(l => l.textContent).join(' ')) || norm(e.getAttribute('aria-label')) || norm(e.placeholder) || norm(((e.closest('.v-input,.v-field') || document.createElement('i')).querySelector('label') || {}).textContent) || norm((e.closest('label') || {}).textContent); // a label with no text names nothing
    const fs = q(FIELDS);
    add('no label', fs.filter(e => !real(e)).map(e => '[' + (e.type || e.tagName.toLowerCase()) + '] ' + [e.name && 'name=' + e.name, e.id && 'id=' + e.id].filter(Boolean).join(' ') + ' @ ' + where(e)));
    const seen = {};
    fs.filter(real).forEach(e => { const l = lower(labelOf(e)); if (l) seen[l] = [...(seen[l] || []), labelOf(e)]; });
    add('same label', Object.values(seen).filter(v => v.length > 1).map(v => v[0] + ' x' + v.length));
    add('no name', q(CLICKABLE).filter(e => !textOf(e)).map(e => tagId(e) + ' @ ' + where(e)));
    return out.length ? out.join('\n') : 'No findings: no unrendered text, covered control, unlabeled field, repeated label or unnamed button.';
  };

  // shape keeps what the screen offers (no values) in the listener, under a name, for parity; the side menu, the top bar, the options of a list and the pagination are left out
  const shape = async a => {
    const name = String(a.name || '').trim();
    if (!name) return fail('shape needs a name, such as legacy or modern. Next: page(shape, {name: "legacy"}).');
    let root = document;
    if (a.selector) {
      try { root = document.querySelector(a.selector); } catch (x) { return fail('bad selector ' + a.selector); }
      if (!root) return fail('nothing matches ' + a.selector + '. Next: page(state) shows what is on the screen.');
    }
    const q = sel => [...root.querySelectorAll(sel)].filter(vis);
    const uniq = l => [...new Set(l.filter(Boolean))];
    // a field named only by its name or id gets the first line of the text around it when that is short
    const nearText = e => { const t = norm(((e.parentElement && e.parentElement.innerText) || '').split('\n')[0]); return t.length <= 40 ? t : ''; };
    const visibleLabel = e => { const l = labelOf(e); return l && (l === e.name || l === e.id) ? nearText(e) || l : l; };
    const inList = e => e.closest('.select2-results,[role=listbox],[role=option],.v-list,.v-pagination,.v-data-table-footer,[role=navigation]');
    const s = {
      fields: q(FIELDS).filter(e => labelOf(e) !== 'Open').map(e => ({ l: visibleLabel(e), t: e.isContentEditable ? 'editor' : e.type || e.tagName.toLowerCase() })).filter(f => f.l),
      buttons: uniq(q(CLICKABLE).filter(e => rank(e) < 2 && !e.matches('[role=tab]') && !inList(e)).map(textOf)),
      columns: uniq(q('th').map(e => norm(e.innerText))),
      tabs: uniq(q('[role=tab]').map(textOf)),
      headings: uniq(q('h1,h2,h3').map(e => norm(e.innerText))),
    };
    const r = await realFetch(cfg.base + '/shape?t=' + cfg.t + '&name=' + encodeURIComponent(name), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(s) });
    if (!r.ok) return fail('the listener refused the shape (' + r.status + '). Next: page(load) again and check WEBSCOPE_APP_URL.');
    return 'shape «' + clip(name, 40) + '» saved: ' + s.fields.length + ' fields, ' + s.buttons.length + ' buttons, ' + s.columns.length + ' columns, ' + s.tabs.length + ' tabs, ' + s.headings.length + ' headings. Next: open the other screen, page(shape) with another name, then page(parity, {a, b}).';
  };

  // ---- interceptors: the functions below, as they are once the helper is in place; a page script or the agent that replaces one is reported
  const mine = { fetch: window.fetch, send: xhrProto.send, 'anchor click': HTMLAnchorElement.prototype.click };
  const current = () => ({ fetch: window.fetch, send: xhrProto.send, 'anchor click': HTMLAnchorElement.prototype.click });
  const patched = () => { const c = current(); return Object.keys(mine).filter(k => c[k] !== mine[k]); };
  const MARK = l => 'Interceptor: ' + l.join(', ') + ' replaced after the helper loaded. What follows is what the page did with that patch, not what a real click did: do not report it as a check of the real flow.';
  let told = '';
  setInterval(() => {
    const now = patched().join(',');
    if (now === told) return;
    told = now;
    realFetch(cfg.base + '/tamper?t=' + cfg.t + '&n=' + encodeURIComponent(now), { method: 'POST', keepalive: true }).catch(() => {});
  }, 400);
  const marked = r => { const l = patched(); return l.length ? MARK(l) + '\n' + r : r; };
  const wrap = f => a => { try { const r = f(a || {}); return r && r.then ? r.then(marked, e => fail(e.message)) : marked(r); } catch (e) { return fail(e.message); } };
  window.__ws = { state: wrap(state), read: wrap(read), fill: wrap(fill), click: wrap(click), press: wrap(press), simulate: wrap(simulate), open: wrap(open), options: wrap(options), audit: wrap(audit), shape: wrap(shape) };
})();
