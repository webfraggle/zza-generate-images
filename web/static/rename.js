// zzaRename(name, target) opens the "rename template" dialog (desktop app).
// The folder name is the template's name; it must be at most 24 characters,
// else the display cannot install the template. On success the page moves to
// target(newName), e.g. n => "/edit/" + n.
(() => {
  const MAX = 24;
  let dlg = null;

  function build() {
    dlg = document.createElement("dialog");
    dlg.id = "rename-dialog";
    dlg.innerHTML =
      '<form method="dialog" novalidate>' +
      '<p><label for="rename-input">Neuer Name des Templates</label></p>' +
      '<input id="rename-input" type="text" autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false">' +
      '<p class="rename-hint"><span id="rename-count"></span> · Kleinbuchstaben, Ziffern und Bindestriche. ' +
      'Der Name steht in der Adresse und im ZZA-Paket; der Titel in <code>meta.name</code> bleibt.</p>' +
      '<p id="rename-error" class="error-text" hidden></p>' +
      '<div class="dialog-actions">' +
      '<button id="rename-ok" class="btn btn-sm btn-primary" type="submit">Umbenennen</button>' +
      '<button id="rename-cancel" class="btn btn-sm btn-level2" type="button">Abbrechen</button>' +
      "</div></form>";
    document.body.appendChild(dlg);
  }

  window.zzaRename = function (name, target) {
    if (!dlg) build();
    const input = dlg.querySelector("#rename-input");
    const count = dlg.querySelector("#rename-count");
    const error = dlg.querySelector("#rename-error");
    const ok    = dlg.querySelector("#rename-ok");

    // Why the name cannot be used, or "" when it can. The button is never
    // disabled without this text being shown.
    function reason(v) {
      if (v === "") return "Bitte einen Namen eingeben.";
      const bad = [...new Set(v.replace(/[a-z0-9-]/g, ""))].map((c) => (c === " " ? "Leerzeichen" : "„" + c + "“"));
      if (bad.length) return "Nicht erlaubt: " + bad.join(", ") + ". Nur Kleinbuchstaben, Ziffern und Bindestriche.";
      if (v.length > MAX) return "Zu lang: " + v.length + " Zeichen, höchstens " + MAX + ".";
      if (v === name) return "Das ist der bisherige Name.";
      return "";
    }

    function check() {
      const v = input.value.trim();
      count.textContent = v.length + " von höchstens " + MAX + " Zeichen";
      count.className = v.length > MAX ? "error-text" : "";
      const why = reason(v);
      ok.disabled = why !== "";
      error.textContent = why;
      error.hidden = why === "" || v === name;
    }

    input.value = name;
    input.oninput = check;
    dlg.querySelector("#rename-cancel").onclick = () => dlg.close();
    dlg.querySelector("form").onsubmit = async (ev) => {
      ev.preventDefault();
      const v = input.value.trim();
      ok.disabled = true;
      let msg = "";
      try {
        const res = await fetch("/edit/" + name + "/rename", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: v }),
        });
        if (res.ok) { location.href = target(v); return; }
        msg = (await res.text()).trim() || "Umbenennen fehlgeschlagen.";
      } catch (e) {
        msg = "Umbenennen fehlgeschlagen: " + e.message;
      }
      error.textContent = msg;
      error.hidden = false;
      ok.disabled = false;
    };
    check();
    dlg.showModal();
    input.select();
  };
})();
