// webscope page helper. Served by the local listener and loaded by the one-line bootstrap that page(load) returns.
// Every action returns a short string; failures start with "Failed:" and name the next step.
(() => {
  // Loading again (the MCP restarted: new port, new token) points the helper at the new listener; the wrappers are installed once.
  const src = document.currentScript.src;
  const cfg = (window.__wsCfg = window.__wsCfg || {});
  cfg.base = src.slice(0, src.indexOf('/page.js'));
  cfg.t = new URL(src).searchParams.get('t');
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
      log.push({ i: seq++, t: c.m + ' ' + u.pathname + ' ' + c.s }); // no query string: it can hold a token, and payloads still shows the whole address
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
    res.clone().text().then(res_ => report({ m, u, s: res.status, req, res: res_ }, true)).catch(() => { pending = Math.max(0, pending - 1); });
    return res;
  };

  // A simulated XMLHttpRequest is sent to the listener's /sim instead, so the app sees a real response with that status.
  const xhrOpen = XMLHttpRequest.prototype.open;
  const xhrSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (m, u) {
    this.__ws = { m: String(m).toUpperCase(), u: String(u) };
    return xhrOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function (body) {
    const c = this.__ws;
    if (c && !c.u.startsWith(cfg.base)) {
      c.req = typeof body === 'string' ? body : body == null ? '' : kind(body);
      const sim = takeSim(c.m, c.u);
      if (sim) {
        xhrOpen.call(this, c.m, cfg.base + '/sim?t=' + cfg.t + '&status=' + sim.status, true);
        this.withCredentials = false;
      }
      pending++;
      this.addEventListener('loadend', () => {
        const text = this.responseType === '' || this.responseType === 'text' ? this.responseText : kind(this.response);
        report({ m: c.m, u: c.u, s: this.status, req: c.req, res: text }, true);
      });
    }
    return xhrSend.apply(this, arguments);
  };

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
    if (e.labels && e.labels.length) return norm(e.labels[0].textContent);
    const box = e.closest('.v-input, .v-field'); // Vuetify keeps the label next to the native input
    const l = box && box.querySelector('label');
    return norm((l && l.textContent) || e.getAttribute('aria-label') || e.placeholder || e.name || e.id);
  };
  const FIELDS = 'input:not([type=hidden]):not([type=button]):not([type=submit]):not([type=reset]),textarea,select';
  const fields = () => all(FIELDS);
  const valueOf = e => {
    if (e.type === 'checkbox' || e.type === 'radio') return e.checked ? 'on' : 'off';
    if (e.tagName === 'SELECT') return norm(e.selectedOptions[0] && e.selectedOptions[0].text);
    return e.value;
  };
  const texts = sel => [...new Set(all(sel).map(e => norm(e.innerText)).filter(Boolean))];
  const CLICKABLE = 'button,a,[role=button],[role=tab],[role=menuitem],[role=option],input[type=submit],input[type=button],.v-btn,.v-list-item';
  const ALERTS = '[role=alert],.v-alert,.v-snackbar__content,.alert,.toast';
  const ERRORS = '.v-messages__message,.invalid-feedback,.error--text,.text-danger';

  const state = a => {
    const out = ['URL: ' + location.pathname + location.search + location.hash, 'Title: ' + norm(document.title)];
    const h = texts('h1,h2,h3').slice(0, 8);
    if (clog.length) out.push('Console (last ' + Math.min(5, clog.length) + ' of ' + clog.length + ' since the helper loaded): ' + clog.slice(-5).map(e => e.t).join(' | '));
    if (h.length) out.push('Headings: ' + h.join(' | '));
    const f = fields();
    if (f.length) {
      out.push('Fields:');
      f.forEach(e => out.push('  [' + (e.type || e.tagName.toLowerCase()) + '] ' + (labelOf(e) || '(no label)') + ' = "' + valueOf(e).replace(/\n/g, '⏎') + '"'));
    }
    const b = [...new Set(all(CLICKABLE).map(textOf).filter(Boolean))].slice(0, 30);
    if (b.length) out.push('Buttons: ' + b.join(' | '));
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
    return clip(out.join('\n'), a.max || CAP);
  };

  const read = a => {
    let e;
    try { e = a.selector ? document.querySelector(a.selector) : document.body; } catch (x) { return fail('bad selector ' + a.selector); }
    if (!e) return fail('nothing matches ' + a.selector + '. Next: page(state) shows what is on the screen.');
    return clip(norm(e.innerText != null ? e.innerText : e.textContent), a.max || CAP);
  };

  // ---- acting on the screen
  const findField = name => {
    const n = lower(name), f = fields().sort((a, b) => !!b.closest('[role=dialog],.v-dialog') - !!a.closest('[role=dialog],.v-dialog'));
    return f.find(e => lower(labelOf(e)) === n) ||
      f.find(e => [e.name, e.id, e.placeholder].some(x => x && lower(x) === n)) ||
      f.find(e => lower(labelOf(e)).includes(n));
  };
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
  const setValue = async (e, v) => {
    e.focus();
    if (e.closest(MENU)) return choose(e, v);
    if (e.type === 'checkbox' || e.type === 'radio') {
      const want = v === true || /^(true|on|1|yes|si|sí)$/i.test(String(v));
      if (e.checked !== want) e.click();
    } else {
      if (e.tagName === 'SELECT') {
        const o = [...e.options].find(o => o.value === String(v) || norm(o.text) === norm(v));
        if (!o) throw new Error('no option ' + v);
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
    if (!a.fields || typeof a.fields !== 'object') return fail('fill needs fields {label: value}');
    let miss = false;
    const out = [];
    for (const [k, v] of Object.entries(a.fields)) {
      const e = findField(k);
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
    if (miss) out.push('Next: page(state) lists the fields and their labels.');
    return out.join('\n');
  };

  // Runs `f` (a click or a key) and appends what it caused: the requests with their status, the new alerts and field errors, a URL change, or that nothing changed.
  const withEffect = async f => {
    const url = () => location.pathname + location.search + location.hash;
    const before = { seq, cseq, alerts: new Set(texts(ALERTS)), errors: new Set(texts(ERRORS)), url: url(), text: norm(document.body.innerText) };
    const r = f();
    if (/^Failed:/.test(r)) return r;
    await sleep(400);
    for (let i = 0; i < 30 && pending > 0; i++) await sleep(70); // a slow answer: about two more seconds
    await sleep(150); // the screen draws the answer
    const out = [r], reqs = log.filter(e => e.i >= before.seq).map(e => e.t);
    if (reqs.length) out.push('Requests: ' + reqs.join(' | '));
    const na = texts(ALERTS).filter(x => !before.alerts.has(x));
    if (na.length) out.push('New alerts: ' + na.join(' | '));
    const ne = texts(ERRORS).filter(x => !before.errors.has(x));
    if (ne.length) out.push('New errors: ' + ne.join(' | '));
    const nc = clog.filter(e => e.i >= before.cseq).map(e => e.t);
    if (nc.length) out.push('New console errors: ' + nc.join(' | '));
    if (url() !== before.url) out.push('URL: ' + before.url + ' → ' + url());
    if (out.length === 1) out.push(norm(document.body.innerText) === before.text ? 'No effect seen: no request, no new alert, the screen text is the same.' : 'The screen text changed; no request, alert or URL change.');
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
      if (els.length > 1) return fail(els.length + ' elements match "' + a.text + '": ' + els.slice(0, 6).map(describe).join(', ') + '. Next: click with a selector.');
    } else return fail('click needs text or selector');
    if (!els.length) return fail('nothing to click for "' + (a.selector || a.text) + '". Buttons here: ' + [...new Set(all(CLICKABLE).map(textOf).filter(Boolean))].slice(0, 12).join(' | '));
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

  const simulate = a => {
    if (a.clear) { sims.length = 0; return 'simulations cleared'; }
    const status = Number(a.status);
    if (!a.match || !(status >= 400 && status <= 599)) return fail('simulate needs match (part of the URL) and status 400-599');
    const times = Number(a.times) || 1;
    sims.push({ match: String(a.match), status, times, method: a.method ? String(a.method).toUpperCase() : '' });
    return 'the next ' + times + ' request(s) with "' + a.match + '" answer ' + status;
  };

  const wrap = f => a => { try { const r = f(a || {}); return r && r.then ? r.catch(e => fail(e.message)) : r; } catch (e) { return fail(e.message); } };
  window.__ws = { state: wrap(state), read: wrap(read), fill: wrap(fill), click: wrap(click), press: wrap(press), simulate: wrap(simulate) };
})();
