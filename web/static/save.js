// zzaSave(blob, filename) hands a file to the user.
//
// In a browser it triggers a normal download. In the desktop app's own window
// the web view ignores downloads (WKWebView on macOS), so the page sets
// window.ZZA_NATIVE_SAVE and the file goes to the app, which shows a native
// "save as" dialog (POST /edit/_desktop/save-file). The file travels as base64
// text: WKWebView hands text bodies to the app but drops binary Blob bodies.
// Resolves when done; rejects with an Error whose message can be shown to the user.
(() => {
  function browserDownload(blob, filename) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  function toBase64(blob) {
    return new Promise((resolve, reject) => {
      const r = new FileReader();
      r.onerror = () => reject(new Error("Datei konnte nicht gelesen werden"));
      // result is "data:<type>;base64,<data>"
      r.onload = () => resolve(String(r.result).slice(String(r.result).indexOf(",") + 1));
      r.readAsDataURL(blob);
    });
  }

  window.zzaSave = async function (blob, filename) {
    if (!window.ZZA_NATIVE_SAVE) {
      browserDownload(blob, filename);
      return;
    }
    const res = await fetch("/edit/_desktop/save-file?name=" + encodeURIComponent(filename), {
      method: "POST",
      headers: { "Content-Type": "text/plain" },
      body: await toBase64(blob),
    });
    if (res.status === 501) {          // no native window (browser fallback)
      browserDownload(blob, filename);
      return;
    }
    if (!res.ok) throw new Error((await res.text()).trim() || "Speichern fehlgeschlagen");
  };
})();
