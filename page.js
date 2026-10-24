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
  const SENT = 4000; // chars of each body reported: a keepalive fetch over 64 KiB is refused by Chrome

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
  const report = c => {
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
    try {
      res = await realFetch(input, init);
    } catch (e) {
      report({ m, u, s: 0, req, res: String(e) });
      throw e;
    }
    res.clone().text().then(res_ => report({ m, u, s: res.status, req, res: res_ })).catch(() => {});
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
      this.addEventListener('loadend', () => {
        const text = this.responseType === '' || this.responseType === 'text' ? this.responseText : kind(this.response);
        report({ m: c.m, u: c.u, s: this.status, req: c.req, res: text });
      });
    }
    return xhrSend.apply(this, arguments);
  };

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

  const state = a => {
    const out = ['URL: ' + location.pathname + location.search + location.hash, 'Title: ' + norm(document.title)];
    const h = texts('h1,h2,h3').slice(0, 8);
    if (h.length) out.push('Headings: ' + h.join(' | '));
    const f = fields();
    if (f.length) {
      out.push('Fields:');
      f.forEach(e => out.push('  [' + (e.type || e.tagName.toLowerCase()) + '] ' + (labelOf(e) || '(no label)') + ' = "' + valueOf(e).replace(/\n/g, '⏎') + '"'));
    }
    const b = [...new Set(all(CLICKABLE).map(textOf).filter(Boolean))].slice(0, 30);
    if (b.length) out.push('Buttons: ' + b.join(' | '));
    const al = texts('[role=alert],.v-alert,.v-snackbar__content,.alert,.toast');
    if (al.length) out.push('Alerts: ' + al.join(' | '));
    const er = texts('.v-messages__message,.invalid-feedback,.error--text,.text-danger');
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
    const n = lower(name), f = fields();
    return f.find(e => lower(labelOf(e)) === n) ||
      f.find(e => [e.name, e.id, e.placeholder].some(x => x && lower(x) === n)) ||
      f.find(e => lower(labelOf(e)).includes(n));
  };
  const setValue = (e, v) => {
    e.focus();
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
  const fill = a => {
    if (!a.fields || typeof a.fields !== 'object') return fail('fill needs fields {label: value}');
    let miss = false;
    const out = Object.entries(a.fields).map(([k, v]) => {
      const e = findField(k);
      if (!e) { miss = true; return k + ': not found'; }
      try {
        setValue(e, v);
        const got = valueOf(e), want = typeof v === 'boolean' ? (v ? 'on' : 'off') : String(v);
        if (e.type === 'checkbox' || e.type === 'radio' || e.tagName === 'SELECT' || got === want) return k + ': ok';
        miss = true; // a v-select keeps its model and puts it back: the field is a menu, so click it and then the option
        return k + ': the page shows "' + got + '" instead of "' + want + '"';
      } catch (x) { miss = true; return k + ': ' + x.message; }
    });
    if (miss) out.push('Next: page(state) lists the fields and their labels.');
    return out.join('\n');
  };

  const click = a => {
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

  const press = a => {
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

  const simulate = a => {
    if (a.clear) { sims.length = 0; return 'simulations cleared'; }
    const status = Number(a.status);
    if (!a.match || !(status >= 400 && status <= 599)) return fail('simulate needs match (part of the URL) and status 400-599');
    const times = Number(a.times) || 1;
    sims.push({ match: String(a.match), status, times, method: a.method ? String(a.method).toUpperCase() : '' });
    return 'the next ' + times + ' request(s) with "' + a.match + '" answer ' + status;
  };

  const wrap = f => a => { try { return f(a || {}); } catch (e) { return fail(e.message); } };
  window.__ws = { state: wrap(state), read: wrap(read), fill: wrap(fill), click: wrap(click), press: wrap(press), simulate: wrap(simulate) };
})();
