/* Food Permit & Licence Renewal Tracker — the local interface.
 *
 * Every rule lives in the Go package behind /api. This file reads the answers
 * and draws them; it works nothing out for itself, so there is no second copy
 * of the alert tiers or the prerequisite ordering to drift out of step with the
 * one that is tested.
 *
 * Everything is built with createElement and textContent rather than innerHTML:
 * a permit number or a note is whatever the owner typed, and it goes into the
 * page as text and never as markup. */
(function () {
  "use strict";

  /* ---------- small helpers ---------- */

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  function clear(node) {
    while (node.firstChild) node.removeChild(node.firstChild);
  }

  function byId(id) { return document.getElementById(id); }

  function flash(message, level) {
    var box = byId("flash");
    box.textContent = message;
    if (level) box.setAttribute("data-level", level); else box.removeAttribute("data-level");
    box.hidden = !message;
  }

  function query(params) {
    var parts = [];
    Object.keys(params).forEach(function (key) {
      var value = params[key];
      if (value === undefined || value === null) return;
      parts.push(encodeURIComponent(key) + "=" + encodeURIComponent(value));
    });
    return parts.length ? "?" + parts.join("&") : "";
  }

  function api(method, path, body) {
    var options = { method: method, headers: {} };
    if (body !== undefined) {
      options.headers["Content-Type"] = "application/json";
      options.body = JSON.stringify(body);
    }
    return fetch(path, options).then(function (response) {
      var type = response.headers.get("Content-Type") || "";
      if (type.indexOf("application/json") < 0) {
        if (!response.ok) throw new Error("the tool returned " + response.status);
        return response.text();
      }
      return response.json().then(function (payload) {
        if (!response.ok) throw new Error((payload && payload.error) || ("the tool returned " + response.status));
        return payload;
      });
    });
  }

  function fail(err) { flash(err && err.message ? err.message : String(err), "bad"); }

  /* ---------- state the page keeps ---------- */

  var state = null;          // the last /api/state payload
  var editingPrereqs = [];   // the prerequisite list in the permit form
  var lastSeason = null;

  function currentSite() {
    var select = byId("site");
    return select && select.value !== null ? select.value : "";
  }

  function asOf() {
    var value = byId("asof").value;
    return value || undefined;
  }

  function scope(extra) {
    var params = { site: currentSite(), asOf: asOf() };
    if (extra) Object.keys(extra).forEach(function (key) { params[key] = extra[key]; });
    return params;
  }

  function presetFor(kind) {
    if (!state) return null;
    for (var i = 0; i < state.presets.length; i++) {
      if (state.presets[i].kind === kind) return state.presets[i];
    }
    return null;
  }

  function labelFor(kind) {
    var preset = presetFor(kind);
    if (preset) return preset.label;
    if (state) {
      for (var i = 0; i < state.permits.length; i++) {
        if (state.permits[i].kind === kind) return state.permits[i].label;
      }
    }
    return kind;
  }

  /* ---------- tabs ---------- */

  var tabs = Array.prototype.slice.call(document.querySelectorAll("[data-tab]"));

  function show(name) {
    tabs.forEach(function (button) {
      var on = button.getAttribute("data-tab") === name;
      button.setAttribute("aria-pressed", on ? "true" : "false");
      var panel = byId("panel-" + button.getAttribute("data-tab"));
      if (panel) panel.hidden = !on;
    });
    flash("");
    if (name === "checklists") drawChecklists();
    if (name === "season") runSeason();
  }

  tabs.forEach(function (button) {
    button.addEventListener("click", function () { show(button.getAttribute("data-tab")); });
  });

  /* ---------- the trade verdict ---------- */

  function drawVerdict(verdict) {
    var host = byId("trade-answer");
    clear(host);

    var tone = verdict.canTrade ? (verdict.checked === 0 ? "unknown" : "yes") : "no";
    var box = el("div", "verdict " + tone);
    box.appendChild(el("p", "k", "As of " + verdict.asOf));
    box.appendChild(el("p", "v", verdict.headline));

    if (verdict.checked > 0) {
      box.appendChild(el("p", "sub", verdict.checked === 1
        ? "One permit on the register is marked as one you cannot trade without."
        : verdict.checked + " permits on the register are marked as ones you cannot trade without."));
    }

    var list = el("ul", "reasons");
    (verdict.blockers || []).forEach(function (blocker) {
      var item = el("li");
      item.appendChild(el("span", "what", blocker.label));
      item.appendChild(document.createTextNode(" — " + blocker.says));
      item.appendChild(el("span", "where", blocker.reason === "expired"
        ? "Expired " + blocker.expiredOn + ", " + blocker.daysOverdue + " day(s) ago."
        : "No expiry date on the register."));
      list.appendChild(item);
    });
    (verdict.atRisk || []).forEach(function (risk) {
      var item = el("li");
      item.appendChild(el("span", "what", risk.label));
      item.appendChild(document.createTextNode(" — " + risk.says));
      item.appendChild(el("span", "where", "Issued by " + risk.authority + "."));
      list.appendChild(item);
    });
    if (list.childNodes.length) box.appendChild(list);

    host.appendChild(box);
  }

  function loadVerdict() {
    return api("GET", "api/can-trade" + query(scope())).then(drawVerdict);
  }

  /* ---------- the register board ---------- */

  function chip(status) {
    var node = el("span", "chip " + status.colour, status.label);
    return node;
  }

  function readinessCell(readiness) {
    var wrap = el("div", "ready");
    if (!readiness.total) {
      wrap.appendChild(el("span", null, "—"));
      return wrap;
    }
    var bar = el("div", "bar");
    // The width comes from a class rather than an inline style, so the page
    // needs no style attribute anywhere and the strict policy in the document
    // head stays strict. Snapping to ten per cent is plenty for a 44-pixel
    // bar; the exact fraction is printed beside it.
    var fill = el("i", "w" + (Math.round(readiness.percent / 10) * 10));
    bar.appendChild(fill);
    wrap.appendChild(bar);
    wrap.appendChild(el("span", null, readiness.done + "/" + readiness.total));
    return wrap;
  }

  function drawCounts(counts) {
    var host = byId("board-counts");
    clear(host);
    [
      ["Expired", counts.expired, "red"],
      ["Final", counts.urgent, "red"],
      ["Second", counts.warn, "amber"],
      ["First", counts.notice, "amber"],
      ["No date", counts.unknown, "amber"],
      ["In hand", counts.ok, "green"],
      ["No expiry", counts.noExpiry, "grey"]
    ].forEach(function (row) {
      var card = el("div", "c " + row[2]);
      card.appendChild(el("div", "n", row[1]));
      card.appendChild(el("div", "l", row[0]));
      host.appendChild(card);
    });
  }

  function drawBoard(board) {
    drawCounts(board.counts);
    var body = byId("board-table").querySelector("tbody");
    clear(body);
    byId("board-empty").hidden = board.rows.length > 0;

    board.rows.forEach(function (row) {
      var permit = row.permit;
      var tr = el("tr", row.status.level === "expired" ? "gone" : null);

      tr.appendChild(el("td")).appendChild(chip(row.status));

      var name = el("td");
      name.appendChild(el("strong", null, permit.label));
      if (permit.site) name.appendChild(el("span", "sub2", permit.site));
      name.appendChild(el("span", "sub2", permit.kind));
      tr.appendChild(name);

      tr.appendChild(el("td", "auth", permit.authority || "—"));
      tr.appendChild(el("td", "mono", permit.number || "—"));
      tr.appendChild(el("td", "mono", permit.noExpiry ? "does not expire" : (permit.expiresOn || "not recorded")));

      var days = el("td", "num");
      days.textContent = permit.noExpiry || !permit.expiresOn ? "—" : String(row.status.daysLeft);
      tr.appendChild(days);

      tr.appendChild(el("td")).appendChild(readinessCell(row.readiness));
      tr.appendChild(el("td", null, permit.blocks ? "yes" : "no"));

      var acts = el("td", "acts");
      var edit = el("button", null, "Edit");
      edit.type = "button";
      edit.addEventListener("click", function () { loadIntoForm(permit); });
      acts.appendChild(edit);
      var remove = el("button", "danger", "Remove");
      remove.type = "button";
      remove.addEventListener("click", function () { removePermit(permit); });
      acts.appendChild(remove);
      tr.appendChild(acts);

      body.appendChild(tr);
    });
  }

  function loadBoard() {
    return api("GET", "api/board" + query(scope())).then(drawBoard);
  }

  function removePermit(permit) {
    api("DELETE", "api/permit" + query({ kind: permit.kind, site: permit.site }))
      .then(function () {
        flash("Removed " + permit.label + ".", "ok");
        return refresh();
      })
      .catch(fail);
  }

  /* ---------- gaps ---------- */

  function drawGaps() {
    var host = byId("gaps");
    clear(host);
    var site = currentSite();
    var any = false;
    var list = el("div", "gaplist");

    ["IN", "PH"].forEach(function (market) {
      var kinds = (state.gaps || {})[market + "\n" + site] || [];
      kinds.forEach(function (kind) {
        var preset = presetFor(kind);
        if (!preset) return;
        any = true;
        var card = el("div", "gap");
        card.appendChild(el("h4", null, preset.label));
        card.appendChild(el("p", null, preset.appliesTo));
        var add = el("button", null, "Add it to my register");
        add.type = "button";
        add.addEventListener("click", function () {
          api("POST", "api/preset" + query({ kind: kind, site: site }))
            .then(function () {
              flash("Added " + preset.label + ". Now type in its number and dates.", "ok");
              return refresh();
            })
            .catch(fail);
        });
        card.appendChild(add);
        list.appendChild(card);
      });
    });

    if (!any) {
      host.appendChild(el("p", "empty", "Every permit in both starting lists is on your register for this premises."));
      return;
    }
    host.appendChild(list);
  }

  /* ---------- the season plan ---------- */

  function basisChip(basis, label) {
    var cls = basis === "statute" ? "statute" : basis === "convention" ? "convention" : "";
    return el("span", "basis " + cls, label);
  }

  function drawSeason(season) {
    lastSeason = season;
    var host = byId("season-body");
    clear(host);

    var head = el("div", "plan-head");
    head.appendChild(el("h3", null, season.title));
    var deadline = el("p");
    deadline.appendChild(document.createTextNode("Window " + season.from + " to " + season.to + ". Worked backwards from "));
    deadline.appendChild(el("span", "deadline", season.dueBy));
    deadline.appendChild(document.createTextNode(" "));
    deadline.appendChild(basisChip(season.dueByBasis,
      season.dueByBasis === "statute" ? "fixed by law" : season.dueByBasis === "convention" ? "convention" : "you set this"));
    head.appendChild(deadline);
    head.appendChild(el("p", null, season.dueByWhy));
    head.appendChild(el("p", null, season.note));
    host.appendChild(head);

    (season.cycles || []).forEach(function (loop) {
      var names = loop.map(labelFor);
      host.appendChild(el("div", "cycle-warn",
        "These permits wait on each other in a loop, so they cannot be put in an order: " +
        names.join(" → ") + " → " + names[0] + ". Remove one of those links under Add or edit a permit."));
    });

    if (!season.rows.length) return;

    var waves = {};
    season.rows.forEach(function (row) {
      if (!waves[row.wave]) waves[row.wave] = [];
      waves[row.wave].push(row);
    });

    Object.keys(waves).sort(function (a, b) { return a - b; }).forEach(function (number) {
      var group = el("div", "wave");
      var heading = el("h3");
      heading.appendChild(el("span", "n", "Step " + number));
      heading.appendChild(document.createTextNode(
        waves[number].length === 1 ? "one permit" : waves[number].length + " permits, any order between them"));
      if (Number(number) > 1) {
        heading.appendChild(el("span", "note2", "waits on step " + (Number(number) - 1)));
      }
      group.appendChild(heading);
      waves[number].forEach(function (row) { group.appendChild(stepCard(row)); });
      host.appendChild(group);
    });
  }

  var whyText = {
    dueInWindow: "due in this window",
    alreadyLapsed: "lapsed earlier and never renewed — everything behind it is stuck",
    dateNotRecorded: "renews in this season, but no expiry date is on the register"
  };

  function stepCard(row) {
    var card = el("div", "step" + (row.critical ? " critical" : "") + (row.slack < 0 ? " late" : ""));

    var head = el("div", "head");
    head.appendChild(el("span", "order", "#" + row.order));
    head.appendChild(el("span", "name", row.permit.label));
    head.appendChild(chip(row.status));
    if (row.critical) head.appendChild(el("span", "basis", "on the longest chain"));
    card.appendChild(head);
    card.appendChild(el("p", "auth", row.permit.authority || ""));
    card.appendChild(el("p", "auth", "In this plan because it is " + (whyText[row.why] || row.why) + "."));

    var dl = el("dl");
    function pair(term, value, cls) {
      var wrap = el("div");
      wrap.appendChild(el("dt", null, term));
      wrap.appendChild(el("dd", cls || null, value));
      dl.appendChild(wrap);
    }
    pair("Start by", row.startBy, row.slack < 0 ? "late" : row.slack <= 7 ? "soon" : null);
    pair("Slack", row.slack < 0 ? Math.abs(row.slack) + " days past it" : row.slack + " days",
      row.slack < 0 ? "late" : null);
    pair("This permit takes", row.leadDays + " days");
    pair("Queued behind it", row.downstreamLeadDays + " days");
    pair("Expires", row.permit.expiresOn || "not recorded");
    pair("Ready", row.readiness.total ? row.readiness.done + " of " + row.readiness.total : "no checklist");
    card.appendChild(dl);

    if (row.blockedBy && row.blockedBy.length) {
      var waits = el("div", "waits");
      waits.appendChild(el("strong", null, "Cannot be done until:"));
      var list = el("ul");
      row.blockedBy.forEach(function (dep) {
        var item = el("li");
        item.appendChild(document.createTextNode(dep.label + " "));
        item.appendChild(basisChip(dep.basis,
          dep.basis === "statute" ? "fixed by law" : dep.basis === "convention" ? "convention — check locally" : "you set this"));
        if (!dep.onRegister) {
          item.appendChild(document.createTextNode(" — not on your register"));
        } else if (!dep.settled) {
          item.appendChild(document.createTextNode(" — not in order yet"));
        } else if (!dep.inSeason) {
          item.appendChild(document.createTextNode(" — already in order, not due in this window"));
        }
        if (dep.why) item.appendChild(el("span", "why", dep.why));
        list.appendChild(item);
      });
      waits.appendChild(list);
      card.appendChild(waits);
    }

    if (row.blocks && row.blocks.length) {
      card.appendChild(el("p", "waits", "Holds up: " + row.blocks.map(labelFor).join(", ") + "."));
    }
    return card;
  }

  function seasonParams() {
    var which = byId("season-window").value;
    var params = scope({ window: which });
    if (which === "custom") {
      params.from = byId("season-from").value;
      params.to = byId("season-to").value;
      params.dueBy = byId("season-dueby").value;
    } else {
      params.year = byId("season-year").value;
    }
    return params;
  }

  function runSeason() {
    var params = seasonParams();
    byId("season-export").setAttribute("href", "api/season.csv" + query(params));
    return api("GET", "api/season" + query(params)).then(drawSeason).catch(fail);
  }

  byId("season-run").addEventListener("click", runSeason);
  byId("season-print").addEventListener("click", function () { window.print(); });
  byId("season-window").addEventListener("change", function () {
    var custom = byId("season-window").value === "custom";
    byId("season-custom").hidden = !custom;
    byId("season-dueby-field").hidden = !custom;
    runSeason();
  });

  /* ---------- the permit form ---------- */

  function loadIntoForm(permit) {
    byId("p-preset").value = "";
    byId("p-kind").value = permit.kind || "";
    byId("p-site").value = permit.site || "";
    byId("p-market").value = permit.jurisdiction || "OTHER";
    byId("p-label").value = permit.label || "";
    byId("p-authority").value = permit.authority || "";
    byId("p-number").value = permit.number || "";
    byId("p-issued").value = permit.issuedOn || "";
    byId("p-expires").value = permit.expiresOn || "";
    byId("p-validity").value = permit.validityMonths || 0;
    byId("p-lead").value = permit.leadDays || 0;
    byId("p-noexpiry").checked = !!permit.noExpiry;
    byId("p-blocks").checked = !!permit.blocks;
    byId("p-anchor").checked = !!permit.seasonAnchor;
    byId("p-docref").value = permit.docRef || "";
    byId("p-notes").value = permit.notes || "";
    editingPrereqs = (permit.prereqs || []).slice();
    drawPrereqs();

    var preset = presetFor(permit.kind);
    byId("p-validity-note").textContent = preset ? preset.note : "";
    show("permit");
    window.scrollTo(0, 0);
  }

  function drawPrereqs() {
    var host = byId("p-prereqs");
    clear(host);
    if (!editingPrereqs.length) {
      host.appendChild(el("p", "empty", "Nothing. This permit can be applied for on its own."));
      return;
    }
    editingPrereqs.forEach(function (dep, index) {
      var row = el("div", "prereq-row");
      row.appendChild(el("span", "name2", labelFor(dep.kind)));
      row.appendChild(basisChip(dep.basis,
        dep.basis === "statute" ? "fixed by law" : dep.basis === "convention" ? "convention — check locally" : "you set this"));
      var drop = el("button", null, "Remove");
      drop.type = "button";
      drop.addEventListener("click", function () {
        editingPrereqs.splice(index, 1);
        drawPrereqs();
      });
      row.appendChild(drop);
      if (dep.why) row.appendChild(el("span", "why2", dep.why));
      host.appendChild(row);
    });
  }

  byId("p-prereq-add").addEventListener("click", function () {
    var kind = byId("p-prereq-kind").value;
    if (!kind) { flash("Pick a permit for this one to wait on.", "bad"); return; }
    if (kind === byId("p-kind").value) { flash("A permit cannot wait on itself.", "bad"); return; }
    for (var i = 0; i < editingPrereqs.length; i++) {
      if (editingPrereqs[i].kind === kind) { flash("That one is already listed.", "bad"); return; }
    }
    editingPrereqs.push({ kind: kind, basis: "owner", why: byId("p-prereq-why").value || "" });
    byId("p-prereq-why").value = "";
    drawPrereqs();
    flash("");
  });

  byId("p-preset").addEventListener("change", function () {
    var kind = byId("p-preset").value;
    if (!kind) return;
    var preset = presetFor(kind);
    if (!preset) return;
    var existing = null;
    for (var i = 0; i < state.permits.length; i++) {
      if (state.permits[i].kind === kind && state.permits[i].site === byId("p-site").value) {
        existing = state.permits[i];
      }
    }
    if (existing) { loadIntoForm(existing); byId("p-preset").value = ""; return; }

    byId("p-kind").value = preset.kind;
    byId("p-market").value = preset.jurisdiction;
    byId("p-label").value = preset.label;
    byId("p-authority").value = preset.authority;
    byId("p-validity").value = preset.validityMonths;
    byId("p-lead").value = preset.leadDays || state.defaults.leadDays;
    byId("p-noexpiry").checked = !!preset.noExpiry;
    byId("p-blocks").checked = !!preset.blocks;
    byId("p-anchor").checked = !!preset.seasonAnchor;
    byId("p-validity-note").textContent = preset.note;
    editingPrereqs = (preset.prereqs || []).slice();
    drawPrereqs();
  });

  byId("p-suggest").addEventListener("click", function () {
    var kind = byId("p-kind").value;
    var issued = byId("p-issued").value;
    if (!kind || !issued) { flash("A short code and an issue date are needed first.", "bad"); return; }
    api("GET", "api/suggest-expiry" + query({ kind: kind, site: byId("p-site").value, issuedOn: issued }))
      .then(function (answer) {
        if (!answer.found) { flash(answer.note, "bad"); return; }
        byId("p-expires").value = answer.suggested;
        flash("Worked out as " + answer.suggested + " from the validity period. The date on the paper always wins.", "ok");
      })
      .catch(fail);
  });

  byId("permit-form").addEventListener("submit", function (event) {
    event.preventDefault();
    var noExpiry = byId("p-noexpiry").checked;
    var permit = {
      kind: byId("p-kind").value.trim().toLowerCase(),
      jurisdiction: byId("p-market").value,
      site: byId("p-site").value,
      label: byId("p-label").value,
      authority: byId("p-authority").value,
      number: byId("p-number").value,
      issuedOn: byId("p-issued").value,
      expiresOn: noExpiry ? "" : byId("p-expires").value,
      noExpiry: noExpiry,
      blocks: byId("p-blocks").checked,
      validityMonths: Number(byId("p-validity").value) || 0,
      leadDays: Number(byId("p-lead").value) || 0,
      seasonAnchor: byId("p-anchor").checked,
      docRef: byId("p-docref").value,
      notes: byId("p-notes").value,
      prereqs: editingPrereqs,
      checklist: null,
      fromPreset: false
    };
    // Keep the checklist that is already on the register, so editing a date
    // does not wipe the ticks.
    for (var i = 0; i < state.permits.length; i++) {
      var existing = state.permits[i];
      if (existing.kind === permit.kind && (existing.site || "") === (permit.site || "")) {
        permit.checklist = existing.checklist;
        permit.fromPreset = existing.fromPreset;
      }
    }
    if (permit.checklist === null) {
      var preset = presetFor(permit.kind);
      if (preset) {
        permit.fromPreset = true;
        permit.checklist = (preset.checklist || []).map(function (text) { return { text: text, done: false }; });
      } else {
        permit.checklist = [];
      }
    }

    api("PUT", "api/permit", permit)
      .then(function (saved) {
        flash("Saved " + saved.label + ".", "ok");
        return refresh();
      })
      .catch(fail);
  });

  byId("permit-form").addEventListener("reset", function () {
    editingPrereqs = [];
    byId("p-validity-note").textContent = "";
    window.setTimeout(drawPrereqs, 0);
  });

  /* ---------- checklists ---------- */

  function drawChecklists() {
    var host = byId("checklist-body");
    clear(host);
    if (!state) return;
    var site = currentSite();
    var shown = 0;

    state.permits.forEach(function (permit) {
      if ((permit.site || "") !== site) return;
      if (!permit.checklist || !permit.checklist.length) return;
      shown++;

      var sheet = el("div", "sheet");
      sheet.appendChild(el("h3", null, permit.label));
      sheet.appendChild(el("p", "where2", permit.authority || ""));
      var preset = presetFor(permit.kind);
      if (preset) sheet.appendChild(el("p", "notes", preset.note));
      if (permit.docRef) {
        sheet.appendChild(el("p", "notes", "The certificate is kept at: " + permit.docRef));
      }

      var list = el("ul", "ticks");
      permit.checklist.forEach(function (item, index) {
        var li = el("li", item.done ? "done" : null);
        var label = el("label");
        var box = document.createElement("input");
        box.type = "checkbox";
        box.checked = !!item.done;
        box.addEventListener("change", function () {
          api("PUT", "api/checklist", {
            kind: permit.kind, site: permit.site, index: index, done: box.checked
          })
            .then(function () { return refresh(true); })
            .then(drawChecklists)
            .catch(function (err) { box.checked = !box.checked; fail(err); });
        });
        label.appendChild(box);
        label.appendChild(el("span", null, item.text));
        li.appendChild(label);
        list.appendChild(li);
      });
      sheet.appendChild(list);
      host.appendChild(sheet);
    });

    if (!shown) {
      host.appendChild(el("p", "empty",
        "No permit at this premises has a checklist yet. Permits added from a starting list come with one."));
    }
  }

  byId("checklist-print").addEventListener("click", function () { window.print(); });

  /* ---------- the late charge ---------- */

  byId("late-run").addEventListener("click", function () {
    var base = byId("late-base").value;
    if (base === "") { flash("Enter the amount you were assessed.", "bad"); return; }
    api("GET", "api/late-charge" + query({
      base: base, dueOn: byId("late-due").value, paidOn: byId("late-paid").value
    }))
      .then(function (charge) {
        var host = byId("late-answer");
        clear(host);
        var box = el("div", "result-primary");
        box.appendChild(el("div", "k", "Total to pay, " + (charge.currency || "")));
        box.appendChild(el("div", "v", charge.totalDue.toFixed(2)));
        box.appendChild(el("div", "sub", charge.note));
        host.appendChild(box);

        var table = el("table", "breakdown");
        var body = el("tbody");
        function line(name, value, cls) {
          var tr = el("tr", cls || null);
          tr.appendChild(el("td", null, name));
          tr.appendChild(el("td", "num", value.toFixed(2)));
          body.appendChild(tr);
        }
        line("Amount assessed", charge.base);
        line("Surcharge at " + (charge.surchargeRate * 100).toFixed(2) + " per cent", charge.surcharge);
        line("Interest at " + (charge.monthlyInterest * 100).toFixed(2) +
          " per cent a month for " + charge.interestMonths + " month(s)", charge.interest);
        if (charge.dailyFees) line("Daily late fee over " + charge.daysLate + " days", charge.dailyFees);
        var total = el("tr", "total");
        total.appendChild(el("td", null, "Total"));
        total.appendChild(el("td", "num", charge.totalDue.toFixed(2)));
        body.appendChild(total);
        table.appendChild(body);
        host.appendChild(table);
        host.appendChild(el("p", "note", charge.basis));
        flash("");
      })
      .catch(fail);
  });

  /* ---------- settings ---------- */

  function drawSettings() {
    byId("b-name").value = state.business.name || "";
    byId("b-locality").value = state.business.locality || "";
    byId("b-market").value = state.business.jurisdiction || "OTHER";
    byId("t-notice").value = state.tiers.noticeDays;
    byId("t-warn").value = state.tiers.warnDays;
    byId("t-urgent").value = state.tiers.urgentDays;
    byId("l-surcharge").value = state.lateRates.surchargeRate;
    byId("l-interest").value = state.lateRates.monthlyInterest;
    byId("l-cap").value = state.lateRates.interestCapMonths;
    byId("l-daily").value = state.lateRates.dailyLateFee;
    byId("l-currency").value = state.lateRates.currency || "";
    byId("data-file").textContent = state.dataFile;
    byId("data-file").setAttribute("title", state.dataFile);
    byId("data-file-full").textContent = state.dataFile;
    byId("tier-text").textContent = state.tiers.noticeDays + ", " + state.tiers.warnDays +
      " and " + state.tiers.urgentDays;
  }

  function saveSettings(payload) {
    return api("PUT", "api/settings", payload)
      .then(function (fresh) {
        state = fresh;
        drawSettings();
        flash("Settings saved.", "ok");
        return refresh();
      })
      .catch(fail);
  }

  byId("settings-save").addEventListener("click", function () {
    saveSettings({
      business: {
        name: byId("b-name").value,
        jurisdiction: byId("b-market").value,
        locality: byId("b-locality").value,
        note: state.business.note || ""
      },
      tiers: {
        noticeDays: Number(byId("t-notice").value) || 0,
        warnDays: Number(byId("t-warn").value) || 0,
        urgentDays: Number(byId("t-urgent").value) || 0
      },
      lateRates: {
        surchargeRate: Number(byId("l-surcharge").value) || 0,
        monthlyInterest: Number(byId("l-interest").value) || 0,
        interestCapMonths: Number(byId("l-cap").value) || state.defaults.interestCapMonths,
        dailyLateFee: Number(byId("l-daily").value) || 0,
        currency: byId("l-currency").value
      }
    });
  });

  byId("settings-reset").addEventListener("click", function () {
    var tiers = state.defaults.tiers;
    var philippine = byId("b-market").value === "PH";
    saveSettings({
      tiers: { noticeDays: tiers[0], warnDays: tiers[1], urgentDays: tiers[2] },
      lateRates: {
        surchargeRate: philippine ? state.defaults.surchargeCeiling : 0,
        monthlyInterest: philippine ? state.defaults.interestCeiling : 0,
        interestCapMonths: state.defaults.interestCapMonths,
        dailyLateFee: 0,
        currency: byId("l-currency").value
      }
    });
  });

  function drawPresetButtons() {
    var host = byId("preset-buttons");
    clear(host);
    [["IN", "India"], ["PH", "Philippines"]].forEach(function (market) {
      var button = el("button", "btn ghost", "Add the " + market[1] + " starting list");
      button.type = "button";
      button.addEventListener("click", function () {
        api("POST", "api/preset" + query({ market: market[0], site: currentSite() }))
          .then(function (result) {
            flash(result.added.length
              ? "Added " + result.added.length + " permits. Delete the ones that do not apply to you, then type in your dates."
              : "Every permit in that list is already on your register for this premises.", "ok");
            return refresh();
          })
          .catch(fail);
      });
      host.appendChild(button);
    });
  }

  /* ---------- export and import ---------- */

  byId("import-run").addEventListener("click", function () {
    var input = byId("import-file");
    if (!input.files || !input.files.length) { flash("Choose a CSV file first.", "bad"); return; }
    var reader = new FileReader();
    reader.onload = function () {
      fetch("api/register.csv" + query({ mode: byId("import-mode").value }), {
        method: "POST",
        headers: { "Content-Type": "text/csv" },
        body: reader.result
      })
        .then(function (response) {
          return response.json().then(function (payload) {
            if (!response.ok) throw new Error(payload.error || ("the tool returned " + response.status));
            return payload;
          });
        })
        .then(function (report) {
          var host = byId("import-report");
          clear(host);
          host.appendChild(el("p", "note",
            "Imported. The register now holds " + report.permits + " permits."));
          if (report.warnings.length) {
            var list = el("ul", "warn-list");
            report.warnings.forEach(function (warning) { list.appendChild(el("li", null, warning)); });
            host.appendChild(list);
          }
          flash("Import finished.", "ok");
          return refresh();
        })
        .catch(fail);
    };
    reader.onerror = function () { flash("That file could not be read.", "bad"); };
    reader.readAsText(input.files[0]);
  });

  /* ---------- wiring ---------- */

  function fillSelect(select, options, keep) {
    var previous = keep ? select.value : null;
    clear(select);
    options.forEach(function (option) {
      var node = document.createElement("option");
      node.value = option[0];
      node.textContent = option[1];
      select.appendChild(node);
    });
    if (previous !== null) {
      for (var i = 0; i < select.options.length; i++) {
        if (select.options[i].value === previous) { select.value = previous; return; }
      }
    }
  }

  function drawSites() {
    var sites = state.sites.length ? state.sites : [""];
    fillSelect(byId("site"), sites.map(function (site) {
      return [site, site || "The main premises"];
    }), true);
  }

  function drawPresetSelects() {
    var options = [["", "— pick one —"]];
    state.presets.forEach(function (preset) {
      options.push([preset.kind, preset.jurisdiction + " · " + preset.label]);
    });
    fillSelect(byId("p-preset"), options);

    var prereqOptions = [["", "— pick one —"]];
    var seen = {};
    state.presets.forEach(function (preset) {
      seen[preset.kind] = true;
      prereqOptions.push([preset.kind, preset.jurisdiction + " · " + preset.label]);
    });
    state.permits.forEach(function (permit) {
      if (seen[permit.kind]) return;
      seen[permit.kind] = true;
      prereqOptions.push([permit.kind, permit.label]);
    });
    fillSelect(byId("p-prereq-kind"), prereqOptions);

    var markets = state.markets.map(function (market) { return [market.code, market.label]; });
    fillSelect(byId("p-market"), markets, true);
    fillSelect(byId("b-market"), markets, true);
  }

  // refresh reloads the state and every view. Pass true to reload the state
  // only, when the caller is about to redraw one view itself.
  function refresh(stateOnly) {
    return api("GET", "api/state").then(function (fresh) {
      state = fresh;
      drawSites();
      drawPresetSelects();
      drawSettings();
      drawGaps();
      if (stateOnly) return null;
      return Promise.all([loadVerdict(), loadBoard()]).then(function () {
        if (!byId("panel-season").hidden) return runSeason();
        if (!byId("panel-checklists").hidden) drawChecklists();
        return null;
      });
    });
  }

  byId("site").addEventListener("change", function () {
    drawGaps();
    Promise.all([loadVerdict(), loadBoard()]).catch(fail);
    if (!byId("panel-season").hidden) runSeason();
    if (!byId("panel-checklists").hidden) drawChecklists();
  });

  byId("asof").addEventListener("change", function () {
    Promise.all([loadVerdict(), loadBoard()]).catch(fail);
    if (!byId("panel-season").hidden) runSeason();
  });

  api("GET", "api/state")
    .then(function (fresh) {
      state = fresh;
      byId("asof").value = fresh.today;
      // The January rush of the year the owner is planning for: in December
      // that is next year, otherwise this one.
      var year = Number(fresh.today.slice(0, 4));
      if (Number(fresh.today.slice(5, 7)) >= 11) year += 1;
      byId("season-year").value = year;
      byId("late-due").value = fresh.today;
      byId("late-paid").value = fresh.today;
      drawPresetButtons();
      return refresh();
    })
    .catch(function (err) {
      flash("The tool could not be reached: " + (err.message || err), "bad");
    });
})();
