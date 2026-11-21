/* sniffile — hand-rolled front end. no framework, no build step.
   if you are reading this to learn, start at load(). */

(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const KINDS = ["image", "video", "audio", "text", "code", "doc", "archive", "other"];

  // The UI follows the system/browser language. Russian and English are
  // deliberately the only dictionaries: adding another one is just another
  // object here, no build step.
  const LANG = String(
    (navigator.languages && navigator.languages[0]) || navigator.language || "en"
  )
    .toLowerCase()
    .startsWith("ru")
    ? "ru"
    : "en";
  const STR = {
    en: {
      title: "sniffile — local file search",
      sub: "local file search",
      placeholder: "photo of a cute pug",
      clear: "clear",
      stop: "stop",
      reindex: "reindex",
      describePhotos: "describe photos",
      files: "files",
      allFolders: "all folders",
      addFolder: "+ folder",
      removeRoot: "Remove {path} from the index?",
      everything: "everything",
      image: "image",
      video: "video",
      audio: "audio",
      text: "text",
      code: "code",
      doc: "doc",
      archive: "archive",
      other: "other",
      indexing: "indexing {root} · {n} files · {path}",
      describing: "describing photos · {n} captions · {path}",
      cancelled: "scan cancelled.",
      warning: "scan finished with a warning: {err}",
      ready: "index ready.",
      hits: "{n} hits · {ms} ms",
      hitOne: "1 hit · {ms} ms",
      partial: " · indexing {n} files (partial)",
      indexingNow: " · indexing {n} files",
      noHits: "no hits for “{q}”",
      tryFewer: "Try fewer words, a different spelling, or switch the folder scope above.",
      onlyFolders: "Only folders added above are searched. If the file lives elsewhere, use “+ folder” to add its parent folder.",
      visionHint: "Photos are searchable by name right now. Pull a vision model (ollama pull moondream) and reindex with “describe photos” to search what is in the picture.",
      reindexScope: "reindex this scope",
      zeroHits: "0 hits · {ms} ms",
      stillIndexing: "still indexing…",
      scanned: "Scanned {n} files so far. This search updates automatically as the index grows.",
      noMatchesYet: "no matches in the scanned part yet for “{q}”.",
      indexingPartial: "indexing · {n} files · partial results",
      idle: "  no query yet.\n  type what you remember — a name, a folder, a phrase.",
      startFolder: "start with a folder",
      pickOne: "Pick one folder, or add several. “all folders” searches everything you added.",
      addFolderBtn: "+ add a folder",
      whatWorks: "what works",
      works1: "file and folder names — “invoice march”, “screenshot 2026”",
      works2: "text inside txt / md / json / source files",
      works3: "dates, image orientation, extensions, sizes",
      works4: "plain-language queries, expanded by the local ollama model",
      works4off: "plain-language queries (add ollama for translation and photo captions)",
      indexedIn: "{n} files indexed in {folders} · everything stays on this machine",
      folderOne: "1 folder",
      folderMany: "{n} folders",
      open: "open",
      folder: "folder",
      pickFolder: "pick a folder to index",
      addFolderTitle: "add a folder",
      useFolder: "use this folder",
      go: "go",
      noSubfolders: "no subfolders",
      added: "added {path} — indexing starts now.",
      scanQueued: "scan queued.",
      stopping: "stopping after the current folder…",
      readyLog: "ready.",
      ollamaText: "text: {model}",
      ollamaNoText: "text: none",
      ollamaVision: "vision: {model}",
      ollamaNoVision: "vision: none (ollama pull moondream)",
      ollamaAlso: "ollama also tried: {words}",
    },
    ru: {
      title: "sniffile — поиск по файлам",
      sub: "локальный поиск файлов",
      placeholder: "фотка с красивым мопсом",
      clear: "очистить",
      stop: "стоп",
      reindex: "переиндексировать",
      describePhotos: "описывать фото",
      files: "файлов",
      allFolders: "все папки",
      addFolder: "+ папка",
      removeRoot: "Убрать {path} из индекса?",
      everything: "всё",
      image: "картинки",
      video: "видео",
      audio: "аудио",
      text: "текст",
      code: "код",
      doc: "документы",
      archive: "архивы",
      other: "другое",
      indexing: "индексация {root} · {n} файлов · {path}",
      describing: "описываю фото · {n} подписей · {path}",
      cancelled: "сканирование отменено.",
      warning: "скан завершился с предупреждением: {err}",
      ready: "индекс готов.",
      hits: "{n} совпадений · {ms} мс",
      hitOne: "1 совпадение · {ms} мс",
      partial: " · индексация {n} файлов (частично)",
      indexingNow: " · индексация {n} файлов",
      noHits: "ничего не найдено по «{q}»",
      tryFewer: "Попробуй меньше слов, другое написание или другую папку сверху.",
      onlyFolders: "Ищутся только добавленные папки. Если файл лежит в другом месте, добавь его папку через «+ папка».",
      visionHint: "Пока фото ищутся по имени. Поставь vision-модель (ollama pull moondream) и переиндексируй с галочкой «описывать фото», чтобы искать по содержимому картинки.",
      reindexScope: "переиндексировать эту папку",
      zeroHits: "0 совпадений · {ms} мс",
      stillIndexing: "ещё индексирую…",
      scanned: "Просканировано {n} файлов. Результаты обновятся автоматически.",
      noMatchesYet: "в просканированной части пока нет совпадений по «{q}».",
      indexingPartial: "индексация · {n} файлов · частичные результаты",
      idle: "  пока нет запроса.\n  напиши, что помнишь — имя, папку, фразу.",
      startFolder: "начни с папки",
      pickOne: "Выбери папку или добавь несколько. «все папки» ищет по всем добавленным.",
      addFolderBtn: "+ добавить папку",
      whatWorks: "что ищется",
      works1: "имена файлов и папок — «счёт март», «скриншот 2026»",
      works2: "текст внутри txt / md / json / исходников",
      works3: "даты, ориентация картинок, расширения, размеры",
      works4: "запросы обычными словами через локальную ollama",
      works4off: "запросы обычными словами (добавь ollama для перевода и описания фото)",
      indexedIn: "{n} файлов в {folders} · всё остаётся на этой машине",
      folderOne: "1 папке",
      folderMany: "{n} папках",
      open: "открыть",
      folder: "папка",
      pickFolder: "выбери папку для индексации",
      addFolderTitle: "добавить папку",
      useFolder: "использовать эту папку",
      go: "перейти",
      noSubfolders: "нет вложенных папок",
      added: "добавлена {path} — индексация началась.",
      scanQueued: "скан поставлен в очередь.",
      stopping: "останавливаюсь после текущей папки…",
      readyLog: "готово.",
      ollamaText: "текст: {model}",
      ollamaNoText: "текст: нет",
      ollamaVision: "картинки: {model}",
      ollamaNoVision: "картинки: нет (ollama pull moondream)",
      ollamaAlso: "ollama также пробовала: {words}",
    },
  };
  const t = (key, vars) => {
    let s = (STR[LANG] && STR[LANG][key]) || STR.en[key] || key;
    return String(s).replace(/\{(\w+)\}/g, (_, k) =>
      vars && vars[k] != null ? String(vars[k]) : ""
    );
  };
  const kindLabel = (k) => t(k);

  const S = {
    roots: [],
    root: "all",
    kind: "",
    q: "",
    results: [],
    selected: -1,
    home: "",
    total: 0,
    ollama: false,
    vision: "",
    pick: "",
    seq: 0,
    timer: 0,
    poll: null,
    wasRunning: false,
    job: {},
    lastLive: 0,
    lastLiveFiles: -1,
  };

  /* ------------------------------------------------------------ helpers */

  const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  };

  const fmtNum = (n) =>
    String(n || 0).replace(/\B(?=(\d{3})+(?!\d))/g, " ");

  const fmtSize = (b) => {
    if (!b && b !== 0) return "";
    const units = ["B", "KB", "MB", "GB", "TB"];
    let v = b;
    let i = 0;
    while (v >= 1024 && i < units.length - 1) {
      v /= 1024;
      i++;
    }
    return (i === 0 ? v : v.toFixed(v >= 10 ? 0 : 1)) + " " + units[i];
  };

  const splitPath = (p) => String(p || "").split(/[\\/]/).filter(Boolean);
  const baseName = (p) => splitPath(p).pop() || p || "";

  const shortPath = (p) => {
    if (!p) return "";
    let s = String(p).replace(/\\/g, "/");
    const home = S.home.replace(/\\/g, "/");
    if (home && (s === home || s.startsWith(home + "/"))) {
      s = "~" + s.slice(home.length);
    }
    if (s.length > 48) {
      const parts = splitPath(s);
      if (parts.length > 2) s = "…/" + parts.slice(-2).join("/");
    }
    return s;
  };

  const setLog = (text, bad) => {
    const node = $("log");
    node.textContent = text;
    node.className = "log" + (bad ? " bad" : "");
  };

  async function api(url, opts) {
    const res = await fetch(url, opts);
    let data = null;
    try {
      data = await res.json();
    } catch (_) {
      /* a 404 from the static handler is not json, and that is fine */
    }
    if (!res.ok) {
      throw new Error((data && data.error) || res.status + " " + res.statusText);
    }
    return data;
  }

  const postJSON = (url, body) =>
    api(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    });

  /* --------------------------------------------------------------- state */

  async function refreshState() {
    try {
      applyState(await api("/api/state"));
    } catch (e) {
      setLog("! " + e.message, true);
    }
  }

  function applyState(st) {
    S.home = st.home || "";
    S.roots = st.roots || [];
    S.total = st.total || 0;
    S.ollama = !!st.ollama;
    S.vision = st.ollama_vision || "";
    if (S.root !== "all" && !S.roots.some((r) => r.path === S.root)) {
      S.root = "all";
    }
    renderRoots();
    updateMeta(st);
    S.job = st.job || {};
    handleJob(S.job);
  }

  function updateMeta(st) {
    $("total").textContent = fmtNum(st.total) + " " + t("files");

    const pill = $("ollama");
    if (st.ollama) {
      pill.hidden = false;
      pill.textContent = st.ollama_vision ? "ollama · vision" : "ollama · text";
      pill.title = [
        st.ollama_text ? t("ollamaText", { model: st.ollama_text }) : t("ollamaNoText"),
        st.ollama_vision ? t("ollamaVision", { model: st.ollama_vision }) : t("ollamaNoVision"),
      ].join("\n");
    } else {
      pill.hidden = true;
    }

    $("enrich-wrap").hidden = !(st.ollama && st.ollama_vision);
  }

  function renderRoots() {
    const host = $("roots");
    host.textContent = "";

    const all = el("button", "root" + (S.root === "all" ? " on" : ""), t("allFolders"));
    all.addEventListener("click", () => {
      S.root = "all";
      renderRoots();
      searchNow();
    });
    host.appendChild(all);

    for (const r of S.roots) {
      const chip = el("button", "root" + (S.root === r.path ? " on" : ""));
      chip.title = r.path;
      chip.appendChild(el("span", "root-name", shortPath(r.path)));
      chip.appendChild(el("span", "root-count", fmtNum(r.files)));
      chip.addEventListener("click", () => {
        S.root = r.path;
        renderRoots();
        searchNow();
      });

      const x = el("span", "root-x", "×");
      x.title = "remove this folder";
      x.addEventListener("click", async (ev) => {
        ev.stopPropagation();
        if (!confirm(t("removeRoot", { path: r.path }))) return;
        try {
          applyState(
            await api("/api/roots?path=" + encodeURIComponent(r.path), {
              method: "DELETE",
            })
          );
          searchNow();
        } catch (e) {
          setLog("! " + e.message, true);
        }
      });
      chip.appendChild(x);
      host.appendChild(chip);
    }

    const add = el("button", "root add", t("addFolder"));
    add.addEventListener("click", () => openPicker(S.home));
    host.appendChild(add);
  }

  function renderFilters() {
    const host = $("filters");
    host.textContent = "";
    const mk = (kind, label) => {
      const b = el("button", "f" + (S.kind === kind ? " on" : ""), label);
      b.addEventListener("click", () => {
        S.kind = kind;
        renderFilters();
        searchNow();
      });
      return b;
    };
    host.appendChild(mk("", t("everything")));
    for (const k of KINDS) host.appendChild(mk(k, kindLabel(k)));
  }

  /* ---------------------------------------------------------------- jobs */

  function handleJob(job) {
    const running = !!job.running;
    $("stop").hidden = !running;
    $("reindex").disabled = running;

    if (running) {
      S.wasRunning = true;
      // Live results: the backend publishes a partial index while a first
      // scan is running, so refresh an active query as files arrive.
      if (
        S.q.trim() &&
        job.phase !== "describing" &&
        job.files !== S.lastLiveFiles &&
        Date.now() - S.lastLive > 1200
      ) {
        S.lastLive = Date.now();
        S.lastLiveFiles = job.files;
        searchNow();
      }
      if (job.phase === "describing") {
        setLog(
          t("describing", {
            n: fmtNum(job.captions),
            path: shortPath(job.path || ""),
          })
        );
      } else {
        setLog(
          t("indexing", {
            root: shortPath(job.root || ""),
            n: fmtNum(job.files),
            path: shortPath(job.path || ""),
          })
        );
      }
    } else if (S.wasRunning) {
      S.wasRunning = false;
      S.lastLiveFiles = -1;
      S.lastLive = 0;
      if (job.phase === "cancelled") {
        setLog(t("cancelled"));
      } else if (job.error) {
        setLog(t("warning", { err: job.error }), true);
      } else {
        setLog(t("ready"));
      }
      refreshState();
      if (S.q) searchNow();
    }
  }

  /* -------------------------------------------------------------- search */

  function syncURL() {
    const p = new URLSearchParams();
    if (S.q.trim()) p.set("q", S.q.trim());
    if (S.root !== "all") p.set("root", S.root);
    if (S.kind) p.set("kind", S.kind);
    const qs = p.toString();
    history.replaceState(null, "", qs ? "/?" + qs : "/");
  }

  function scheduleSearch() {
    clearTimeout(S.timer);
    S.timer = setTimeout(searchNow, 220);
  }

  async function searchNow() {
    syncURL();
    const q = S.q.trim();
    S.selected = -1;
    if (!q) {
      renderIdle();
      return;
    }
    const seq = ++S.seq;
    const params = new URLSearchParams({
      q,
      root: S.root,
      kind: S.kind,
      limit: "60",
    });
    try {
      const res = await api("/api/search?" + params.toString());
      if (seq !== S.seq) return; // a newer keystroke won
      S.results = res.results || [];
      renderResults(res);
    } catch (e) {
      if (seq !== S.seq) return;
      setLog("! " + e.message, true);
    }
  }

  function renderResults(res) {
    const host = $("results");
    host.textContent = "";

    if (!res.results.length) {
      renderNoHits(res);
      return;
    }

    const max = res.results[0].score || 1;
    res.results.forEach((r, i) => host.appendChild(hitRow(r, i, max)));

    const extra = res.expanded && res.expanded.length
      ? " · " + t("ollamaAlso", { words: res.expanded.join(", ") })
      : "";
    setLog(
      (res.count === 1
        ? t("hitOne", { ms: res.took_ms })
        : t("hits", { n: fmtNum(res.count), ms: res.took_ms })) +
        extra +
        (S.job && S.job.running
          ? t("partial", { n: fmtNum(S.job.files) })
          : "")
    );
  }

  function hitRow(r, i, max) {
    const row = el("article", "hit");
    row.tabIndex = 0;
    row.dataset.i = i;

    const thumb = el("div", "thumb");
    if (r.kind === "image") {
      const img = el("img");
      img.loading = "lazy";
      img.alt = "";
      img.src = "/api/thumb?path=" + encodeURIComponent(r.path);
      img.addEventListener("error", () => {
        thumb.textContent = "";
        thumb.appendChild(fileGlyph(r));
      });
      img.addEventListener("click", (ev) => {
        ev.stopPropagation();
        openLightbox(r);
      });
      thumb.appendChild(img);
    } else {
      thumb.appendChild(fileGlyph(r));
    }

    const body = el("div", "hit-body");
    const line = el("div", "hit-line");
    line.appendChild(el("span", "hit-name", r.name));
    line.appendChild(el("span", "badge", r.kind));
    line.appendChild(el("span", "hit-size", fmtSize(r.size)));
    if (r.width && r.height) {
      line.appendChild(el("span", "hit-size", r.width + "×" + r.height));
    }
    body.appendChild(line);
    body.appendChild(el("div", "hit-path", shortPath(r.dir)));

    if (r.caption) {
      body.appendChild(el("div", "hit-caption", r.caption));
    } else if (r.snippet) {
      body.appendChild(el("div", "hit-snippet", r.snippet));
    }

    if (r.matched && r.matched.length) {
      const why = el("div", "hit-why");
      r.matched.slice(0, 4).forEach((m, k) => {
        if (k) why.appendChild(el("span", "sep", "·"));
        why.appendChild(el("span", null, m));
      });
      body.appendChild(why);
    }

    const side = el("div", "hit-side");
    const bar = el("div", "bar");
    const fill = el("i");
    fill.style.width = Math.max(3, Math.round((r.score / max) * 100)) + "%";
    bar.appendChild(fill);
    side.appendChild(bar);

    const actions = el("div", "hit-actions");
    const bOpen = el("button", "mini", "open");
    bOpen.addEventListener("click", (ev) => {
      ev.stopPropagation();
      openFile(r.path);
    });
    const bDir = el("button", "mini", "folder");
    bDir.addEventListener("click", (ev) => {
      ev.stopPropagation();
      openFile(r.path, true);
    });
    actions.append(bOpen, bDir);
    side.appendChild(actions);

    row.append(thumb, body, side);
    row.addEventListener("click", () => openFile(r.path));
    row.addEventListener("keydown", (ev) => {
      if (ev.key === "Enter") openFile(r.path);
    });
    return row;
  }

  function fileGlyph(r) {
    const hasExt = r.name && r.name.includes(".");
    const label = hasExt
      ? r.name.split(".").pop().slice(0, 4).toUpperCase()
      : r.kind.slice(0, 4).toUpperCase();
    return el("div", "glyph", label || "FILE");
  }

  function renderIdle() {
    const host = $("results");
    host.textContent = "";

    const box = el("div", "idle");
    const art = el(
      "pre",
      "idle-art",
      "  no query yet.\n" +
        "  type what you remember — a name, a folder, a phrase."
    );
    box.appendChild(art);

    if (!S.roots.length) {
      box.appendChild(el("h2", null, "start with a folder"));
      box.appendChild(
        el("p", null, "Pick one folder, or add several. “all folders” searches everything you added.")
      );
      const b = el("button", "btn", "+ add a folder");
      b.addEventListener("click", () => openPicker(S.home));
      box.appendChild(b);
    } else {
      box.appendChild(el("h2", null, "what works"));
      const ul = el("ul");
      for (const text of [
        "file and folder names — “invoice march”, “screenshot 2026”",
        "text inside txt / md / json / source files",
        "dates, image orientation, extensions, sizes",
        S.ollama
          ? "plain-language queries, expanded by the local ollama model"
          : "plain-language queries (add ollama for translation and photo captions)",
      ]) {
        ul.appendChild(el("li", null, text));
      }
      box.appendChild(ul);
    }

    const stats = el("div", "stats");
    stats.textContent =
      fmtNum(S.total) +
      " files indexed in " +
      S.roots.length +
      (S.roots.length === 1 ? " folder" : " folders") +
      " · everything stays on this machine";
    box.appendChild(stats);

    host.appendChild(box);
  }

  function renderNoHits(res) {
    const host = $("results");
    const box = el("div", "empty");
    const indexing = !!(S.job && S.job.running && S.job.phase !== "describing");

    if (indexing) {
      box.appendChild(el("h2", null, t("stillIndexing")));
      box.appendChild(el("p", null, t("scanned", { n: fmtNum(S.job.files) })));
      box.appendChild(el("p", "small", t("noMatchesYet", { q: res.query })));
      host.appendChild(box);
      setLog(t("indexingPartial", { n: fmtNum(S.job.files) }));
      return;
    }

    box.appendChild(el("h2", null, t("noHits", { q: res.query })));
    box.appendChild(el("p", null, t("tryFewer")));
    box.appendChild(el("p", "small", t("onlyFolders")));

    if (res.expanded && res.expanded.length) {
      box.appendChild(
        el("p", "small", t("ollamaAlso", { words: res.expanded.join(", ") }))
      );
    }
    if (S.ollama && !S.vision) {
      box.appendChild(el("p", "small", t("visionHint")));
    }

    const b = el("button", "btn", t("reindexScope"));
    b.addEventListener("click", startScan);
    box.appendChild(b);
    host.appendChild(box);
    setLog(t("zeroHits", { ms: res.took_ms }));
  }

  /* ------------------------------------------------------------- actions */

  function openFile(path, reveal) {
    postJSON("/api/open", { path, reveal: !!reveal }).catch((e) =>
      setLog("! " + e.message, true)
    );
  }

  async function startScan() {
    const body = {
      root: S.root === "all" ? "" : S.root,
      enrich: $("enrich").checked,
    };
    try {
      applyState(await postJSON("/api/scan", body));
      setLog("scan queued.");
    } catch (e) {
      setLog("! " + e.message, true);
    }
  }

  async function stopScan() {
    try {
      await postJSON("/api/scan/cancel", {});
      setLog("stopping after the current folder…");
    } catch (e) {
      setLog("! " + e.message, true);
    }
  }

  /* -------------------------------------------------------------- picker */

  async function openPicker(path) {
    $("picker").hidden = false;
    $("pick-hint").textContent = "pick a folder to index";
    await browse(path || S.home);
  }

  function closePicker() {
    $("picker").hidden = true;
    $("q").focus();
  }

  async function browse(path) {
    try {
      const res = await api("/api/browse?path=" + encodeURIComponent(path || ""));
      S.pick = res.path;
      $("pick-path").value = res.path;

      const host = $("dirs");
      host.textContent = "";

      const up = el("button", "dir", "..");
      up.disabled = !res.parent;
      up.addEventListener("click", () => {
        if (res.parent) browse(res.parent);
      });
      host.appendChild(up);

      for (const d of res.dirs) {
        const b = el("button", "dir", d.name + "/");
        b.addEventListener("click", () => browse(d.path));
        host.appendChild(b);
      }
      if (!res.dirs.length) {
        host.appendChild(el("div", "dir empty", "no subfolders"));
      }
    } catch (e) {
      $("pick-hint").textContent = e.message;
    }
  }

  async function useFolder() {
    if (!S.pick) return;
    try {
      applyState(await postJSON("/api/roots", { path: S.pick }));
      $("picker").hidden = true;
      setLog("added " + shortPath(S.pick) + " — indexing starts now.");
      searchNow();
    } catch (e) {
      $("pick-hint").textContent = e.message;
    }
  }

  /* ------------------------------------------------------------ lightbox */

  function openLightbox(r) {
    $("lb-img").src = "/api/file?path=" + encodeURIComponent(r.path);
    $("lb-cap").textContent =
      r.name + (r.caption ? " — " + r.caption : " — " + fmtSize(r.size));
    $("lightbox").hidden = false;
  }

  function closeLightbox() {
    $("lightbox").hidden = true;
    $("lb-img").removeAttribute("src");
  }

  /* ------------------------------------------------------------ keyboard */

  function move(delta) {
    if (!S.results.length) return;
    S.selected = Math.max(0, Math.min(S.results.length - 1, S.selected + delta));
    const rows = document.querySelectorAll(".hit");
    rows.forEach((row, i) => row.classList.toggle("sel", i === S.selected));
    if (rows[S.selected]) rows[S.selected].scrollIntoView({ block: "nearest" });
  }

  function toggleHelp(force) {
    const panel = $("help");
    panel.hidden = force != null ? !force : !panel.hidden;
  }

  document.addEventListener("keydown", (ev) => {
    const typing =
      document.activeElement &&
      (document.activeElement.tagName === "INPUT" ||
        document.activeElement.tagName === "TEXTAREA");

    if (ev.key === "Escape") {
      if (!$("lightbox").hidden) return closeLightbox();
      if (!$("picker").hidden) return closePicker();
      if (!$("help").hidden) return toggleHelp(false);
      if (typing && S.q) {
        S.q = "";
        $("q").value = "";
        S.results = [];
        syncURL();
        renderIdle();
      }
      return;
    }

    if (typing) return;

    if (ev.key === "/") {
      ev.preventDefault();
      $("q").focus();
      $("q").select();
      return;
    }
    if (ev.key === "?") {
      ev.preventDefault();
      toggleHelp();
      return;
    }
    if (!$("help").hidden) return;

    if (ev.key === "j" || ev.key === "ArrowDown") {
      ev.preventDefault();
      move(1);
    } else if (ev.key === "k" || ev.key === "ArrowUp") {
      ev.preventDefault();
      move(-1);
    } else if (ev.key === "Enter" && S.selected >= 0) {
      openFile(S.results[S.selected].path);
    } else if (ev.key === "o" && S.selected >= 0) {
      openFile(S.results[S.selected].path);
    } else if (ev.key === "r" && S.selected >= 0) {
      openFile(S.results[S.selected].path, true);
    }
  });

  /* ---------------------------------------------------------------- wire */

  function load() {
    const params = new URLSearchParams(location.search);
    S.q = params.get("q") || "";
    S.root = params.get("root") || "all";
    S.kind = params.get("kind") || "";
    $("q").value = S.q;

    renderFilters();
    renderIdle();
    refreshState().then(() => {
      if (S.q) searchNow();
      else renderIdle();
      $("q").focus();
    });

    $("q").addEventListener("input", () => {
      S.q = $("q").value;
      if (!S.q.trim()) {
        S.results = [];
        syncURL();
        renderIdle();
        return;
      }
      scheduleSearch();
    });

    $("q").addEventListener("keydown", (ev) => {
      if (ev.key === "Enter") {
        clearTimeout(S.timer);
        searchNow();
      }
    });

    $("clear").addEventListener("click", () => {
      S.q = "";
      $("q").value = "";
      S.results = [];
      syncURL();
      renderIdle();
      $("q").focus();
    });

    $("reindex").addEventListener("click", startScan);
    $("stop").addEventListener("click", stopScan);
    $("help-btn").addEventListener("click", () => toggleHelp());
    $("help-close").addEventListener("click", () => toggleHelp(false));

    $("pick-close").addEventListener("click", closePicker);
    $("pick-go").addEventListener("click", () => browse($("pick-path").value));
    $("pick-use").addEventListener("click", useFolder);
    $("pick-path").addEventListener("keydown", (ev) => {
      if (ev.key === "Enter") browse($("pick-path").value);
    });
    $("up").addEventListener("click", () => {
      const parts = splitPath(S.pick);
      parts.pop();
      browse("/" + parts.join("/"));
    });
    $("picker").addEventListener("click", (ev) => {
      if (ev.target === $("picker")) closePicker();
    });

    $("lb-close").addEventListener("click", closeLightbox);
    $("lightbox").addEventListener("click", (ev) => {
      if (ev.target === $("lightbox")) closeLightbox();
    });

    // poll state; the payload is tiny and this keeps the footer honest
    S.poll = setInterval(() => {
      if (!document.hidden) refreshState();
    }, 1600);
  }

  load();
})();
