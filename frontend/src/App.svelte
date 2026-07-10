<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import {
    LibraryService,
    OPDSService,
    CalibreService,
    SettingsService,
    type BookCard,
    type BookDetail,
    type SidebarItem,
    type AppSettings,
    type OPDSStatus,
    type CalibreStatus,
    type CalibreSendProgress,
    type ImportProgress,
    type ImportSummary,
  } from "./lib/api";
  import { type View, viewTitle } from "./lib/types";
  import Sidebar from "./lib/Sidebar.svelte";
  import BookGrid from "./lib/BookGrid.svelte";
  import GroupGrid from "./lib/GroupGrid.svelte";
  import BookDetailView from "./lib/BookDetail.svelte";
  import SettingsModal from "./lib/SettingsModal.svelte";

  let view = $state<View>({ kind: "all" });
  let browseMode = $state<"books" | "author" | "series" | "tag">("books");
  let parentBrowseMode = $state<"author" | "series" | "tag" | null>(null);
  let sort = $state<"title" | "author" | "added">("title");
  let viewMode = $state<"grid" | "list">("grid");
  let query = $state("");
  let books = $state<BookCard[]>([]);
  let loading = $state(true);
  let selectedIds = $state<number[]>([]);
  let batchSeries = $state("");
  let batchSeriesStart = $state("");

  let total = $state(0);
  let authors = $state<SidebarItem[]>([]);
  let series = $state<SidebarItem[]>([]);
  let tags = $state<SidebarItem[]>([]);
  let authorGroups = $state<SidebarItem[]>([]);
  let seriesGroups = $state<SidebarItem[]>([]);
  let tagGroups = $state<SidebarItem[]>([]);

  let detail = $state<BookDetail | null>(null);
  let settings = $state<AppSettings | null>(null);
  let opdsStatus = $state<OPDSStatus | null>(null);
  let calibre = $state<CalibreStatus | null>(null);
  let sending = $state(false);
  let settingsOpen = $state(false);

  let importing = $state(false);
  let progress = $state<ImportProgress | null>(null);
  let toast = $state("");
  let dragOver = $state(false);

  async function loadBooks() {
    loading = true;
    try {
      const q = query.trim();
      let res: BookCard[] | null;
      if (q) res = await LibraryService.Search(q);
      else if (view.kind === "author") res = view.id === 0 ? await LibraryService.BooksWithoutAuthor() : await LibraryService.BooksByAuthor(view.id);
      else if (view.kind === "series") res = view.id === 0 ? await LibraryService.BooksWithoutSeries() : await LibraryService.BooksBySeries(view.id);
      else if (view.kind === "tag") res = view.id === 0 ? await LibraryService.BooksWithoutTag() : await LibraryService.BooksByTag(view.id);
      else res = await LibraryService.Books(sort);
      books = res ?? [];
      const visible = new Set(books.map((b) => b.id));
      selectedIds = selectedIds.filter((id) => visible.has(id));
    } finally {
      loading = false;
    }
  }

  async function loadSidebar() {
    const [t, a, s, g, ag, sg, tg] = await Promise.all([
      LibraryService.Stats(),
      LibraryService.Authors(),
      LibraryService.SeriesList(),
      LibraryService.Tags(),
      LibraryService.AuthorGroups(),
      LibraryService.SeriesGroups(),
      LibraryService.TagGroups(),
    ]);
    total = t.books;
    authors = a ?? [];
    series = s ?? [];
    tags = g ?? [];
    authorGroups = ag ?? [];
    seriesGroups = sg ?? [];
    tagGroups = tg ?? [];
  }

  function selectView(v: View) {
    view = v;
    browseMode = "books";
    parentBrowseMode = null;
    query = "";
    clearSelection();
    loadBooks();
  }

  let searchTimer: ReturnType<typeof setTimeout>;
  function onSearch(e: Event) {
    query = (e.target as HTMLInputElement).value;
    clearTimeout(searchTimer);
    if (browseMode === "books") {
      searchTimer = setTimeout(loadBooks, 200);
    }
  }

  function setBrowseMode(mode: "books" | "author" | "series" | "tag") {
    browseMode = mode;
    parentBrowseMode = null;
    clearSelection();
    if (mode !== "books") {
      view = { kind: "all" };
      query = "";
    }
    if (mode === "books") {
      loadBooks();
    }
  }

  function openGroup(kind: "author" | "series" | "tag", item: SidebarItem) {
    view = { kind, id: item.id, name: item.name };
    browseMode = "books";
    parentBrowseMode = kind;
    query = "";
    clearSelection();
    loadBooks();
  }

  function backToGroups() {
    if (!parentBrowseMode) return;
    setBrowseMode(parentBrowseMode);
  }

  function setSort(s: "title" | "author" | "added") {
    sort = s;
    if (!query.trim()) loadBooks();
  }

  async function openBook(id: number) {
    try {
      detail = await LibraryService.Book(id);
    } catch (e) {
      console.error(e);
    }
  }

  async function removeBook(book: BookDetail) {
    console.info("[Reliure] RemoveBook requested", { id: book.id, title: book.title });
    try {
      const res = await LibraryService.RemoveBook(book.id);
      console.info("[Reliure] RemoveBook completed", res);
      detail = null;
      toast =
        res.trashedFiles > 0
          ? `Livre retiré · ${res.trashedFiles} fichier${res.trashedFiles === 1 ? "" : "s"} déplacé${res.trashedFiles === 1 ? "" : "s"} dans la corbeille`
          : "Livre retiré de l’index";
      setTimeout(() => (toast = ""), 6000);
      await Promise.all([loadSidebar(), loadBooks()]);
    } catch (e) {
      console.error("[Reliure] RemoveBook failed", e);
      toast = `Suppression impossible · ${errorMessage(e)}`;
      setTimeout(() => (toast = ""), 6000);
    }
  }

  async function saveBook(update: Parameters<typeof LibraryService.UpdateBook>[0]) {
    try {
      detail = await LibraryService.UpdateBook(update);
      toast = "Métadonnées enregistrées";
      setTimeout(() => (toast = ""), 4000);
      await Promise.all([loadSidebar(), loadBooks()]);
    } catch (e) {
      console.error("[Reliure] UpdateBook failed", e);
      toast = `Enregistrement impossible · ${errorMessage(e)}`;
      setTimeout(() => (toast = ""), 6000);
      throw e;
    }
  }

  async function setTitleSort(sort: string) {
    if (!detail) return;
    try {
      detail = await LibraryService.SetTitleSort(detail.id, sort);
      await Promise.all([loadSidebar(), loadBooks()]);
    } catch (e) {
      toast = `Enregistrement impossible · ${errorMessage(e)}`;
      setTimeout(() => (toast = ""), 6000);
    }
  }
  async function setAuthorSort(authorId: number, sort: string) {
    if (!detail) return;
    try {
      detail = await LibraryService.SetAuthorSort(detail.id, authorId, sort);
      await Promise.all([loadSidebar(), loadBooks()]);
    } catch (e) {
      toast = `Enregistrement impossible · ${errorMessage(e)}`;
      setTimeout(() => (toast = ""), 6000);
    }
  }

  function toggleSelect(id: number) {
    selectedIds = selectedIds.includes(id)
      ? selectedIds.filter((x) => x !== id)
      : [...selectedIds, id];
  }

  function clearSelection() {
    selectedIds = [];
    batchSeries = "";
    batchSeriesStart = "";
  }

  async function applyBatchSeries() {
    if (selectedIds.length === 0) return;
    try {
      const orderedIds = books.map((b) => b.id).filter((id) => selectedIds.includes(id));
      const res = await LibraryService.BatchSetSeries({
        ids: orderedIds,
        series: batchSeries,
        seriesIndexStart: batchSeriesStart,
      });
      toast = `${res.updated} livre${res.updated === 1 ? "" : "s"} mis à jour`;
      setTimeout(() => (toast = ""), 4000);
      clearSelection();
      await Promise.all([loadSidebar(), loadBooks()]);
    } catch (e) {
      console.error("[Reliure] BatchSetSeries failed", e);
      toast = `Édition par lot impossible · ${errorMessage(e)}`;
      setTimeout(() => (toast = ""), 6000);
    }
  }

  function errorMessage(e: unknown): string {
    if (e instanceof Error) return e.message;
    if (typeof e === "string") return e;
    try {
      return JSON.stringify(e);
    } catch {
      return "erreur inconnue";
    }
  }

  function doImport() {
    // Progress + completion arrive via events; no need to await the result.
    LibraryService.ChooseAndImport().catch((e) => console.error(e));
  }

  async function openSettings() {
    const [nextSettings, nextOPDS] = await Promise.all([SettingsService.Get(), OPDSService.Status()]);
    settings = nextSettings;
    opdsStatus = nextOPDS;
    settingsOpen = true;
  }
  async function setMode(m: "copy" | "reference") {
    settings = await SettingsService.SetImportMode(m);
  }
  async function chooseFolder() {
    settings = await SettingsService.ChooseLibraryFolder();
  }
  async function setRemotePathTemplate(tmpl: string) {
    settings = await SettingsService.SetRemotePathTemplate(tmpl);
  }
  async function setWriteMetadataToFile(enabled: boolean) {
    settings = await SettingsService.SetWriteMetadataToFile(enabled);
  }

  // applyTheme reflects the choice onto the document: "system" removes the
  // attribute so the OS preference (via CSS) governs; light/dark pin it. The
  // value is mirrored to localStorage so the next launch applies it before the
  // Go settings round-trip (no flash).
  function applyTheme(theme: string | undefined) {
    const t = theme === "light" || theme === "dark" ? theme : "system";
    if (t === "system") delete document.documentElement.dataset.theme;
    else document.documentElement.dataset.theme = t;
    try {
      localStorage.setItem("theme", t);
    } catch {}
  }
  async function setTheme(theme: "system" | "light" | "dark") {
    applyTheme(theme);
    settings = await SettingsService.SetTheme(theme);
  }
  async function regenerateCovers() {
    try {
      const res = await LibraryService.RegenerateCovers();
      toast = res.updated
        ? `${res.updated} vignette${res.updated === 1 ? "" : "s"} générée${res.updated === 1 ? "" : "s"}`
        : "Aucune vignette à générer";
      setTimeout(() => (toast = ""), 5000);
      await loadBooks();
    } catch (e) {
      toast = `Régénération impossible · ${errorMessage(e)}`;
      setTimeout(() => (toast = ""), 6000);
    }
  }
  async function setOPDSEnabled(enabled: boolean) {
    opdsStatus = await OPDSService.SetEnabled(enabled);
    settings = await SettingsService.Get();
  }
  async function setOPDSPort(port: number) {
    opdsStatus = await OPDSService.SetPort(port);
    settings = await SettingsService.Get();
  }
  async function setCalibreEnabled(enabled: boolean) {
    calibre = await CalibreService.SetEnabled(enabled);
  }

  async function sendToDevice() {
    if (sending || selectedIds.length === 0) return;
    const orderedIds = books.map((b) => b.id).filter((id) => selectedIds.includes(id));
    sending = true;
    try {
      const res = await CalibreService.SendBooks(orderedIds);
      toast =
        `${res.sent} envoyé${res.sent === 1 ? "" : "s"} vers la liseuse` +
        (res.failed ? ` · ${res.failed} échec${res.failed === 1 ? "" : "s"}` : "");
      setTimeout(() => (toast = ""), 6000);
      clearSelection();
    } catch (e) {
      toast = `Envoi impossible · ${errorMessage(e)}`;
      setTimeout(() => (toast = ""), 6000);
    } finally {
      sending = false;
    }
  }

  onMount(() => {
    loadSidebar();
    loadBooks();
    OPDSService.Status().then((s) => (opdsStatus = s)).catch(() => {});
    CalibreService.Status().then((s) => (calibre = s)).catch(() => {});
    // Sync the theme from persisted settings (source of truth).
    SettingsService.Get().then((s) => {
      settings = s;
      applyTheme(s.theme);
    }).catch(() => {});

    const offCalibre = Events.On("calibre:status", (e: { data: CalibreStatus }) => {
      calibre = e.data;
    });
    const offCalibreProgress = Events.On("calibre:progress", (e: { data: CalibreSendProgress }) => {
      const p = e.data;
      toast = `Envoi ${p.done}/${p.total} · ${p.title}${p.ok ? "" : " (échec)"}`;
    });

    const offProgress = Events.On("import:progress", (e: { data: ImportProgress }) => {
      importing = true;
      progress = e.data;
    });
    const offDone = Events.On("import:done", (e: { data: ImportSummary }) => {
      importing = false;
      progress = null;
      const s = e.data;
      toast =
        `${s.imported} importé${s.imported === 1 ? "" : "s"}` +
        (s.attached ? ` · ${s.attached} format${s.attached === 1 ? "" : "s"} ajouté${s.attached === 1 ? "" : "s"}` : "") +
        (s.duplicates ? ` · ${s.duplicates} doublon${s.duplicates === 1 ? "" : "s"}` : "") +
        (s.failed ? ` · ${s.failed} échec${s.failed === 1 ? "" : "s"}` : "");
      setTimeout(() => (toast = ""), 6000);
      loadSidebar();
      loadBooks();
    });
    return () => {
      offCalibre();
      offCalibreProgress();
      offProgress();
      offDone();
    };
  });

  const pct = $derived(
    progress && progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0,
  );
  const groupItems = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const source = browseMode === "author" ? authorGroups : browseMode === "series" ? seriesGroups : browseMode === "tag" ? tagGroups : [];
    if (!q) return source;
    return source.filter((item) => item.name.toLowerCase().includes(q));
  });
  const visibleCount = $derived(browseMode === "books" ? books.length : groupItems.length);
</script>

<svelte:window
  ondragover={(e) => {
    e.preventDefault();
    dragOver = true;
  }}
  ondragleave={(e) => {
    if (e.relatedTarget === null) dragOver = false;
  }}
  ondrop={() => (dragOver = false)}
/>

<div class="app" data-file-drop-target>
  <Sidebar
    {total}
    {authors}
    {series}
    {tags}
    opds={opdsStatus}
    {calibre}
    active={view}
    onSelect={selectView}
    onOpenSettings={openSettings}
  />

  <main class="main">
    <header class="toolbar">
      <div class="title">
        {#if parentBrowseMode}
          <button class="back" onclick={backToGroups} aria-label="Retour aux groupes">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M15 18l-6-6 6-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            Retour
          </button>
        {/if}
        <h1>{browseMode === "books" ? viewTitle(view) : browseMode === "author" ? "Auteurs" : browseMode === "series" ? "Séries" : "Tags"}</h1>
        <span class="n">{visibleCount}</span>
      </div>

      <div class="search">
        <svg viewBox="0 0 24 24" aria-hidden="true"
          ><circle cx="11" cy="11" r="7" fill="none" stroke="currentColor" stroke-width="2" /><path
            d="M21 21l-4.3-4.3" stroke="currentColor" stroke-width="2" stroke-linecap="round"
          /></svg
        >
        <input
          type="search"
          placeholder="Rechercher…"
          value={query}
          oninput={onSearch}
          aria-label="Rechercher"
        />
      </div>

      <div class="sort">
        <select value={browseMode} onchange={(e) => setBrowseMode((e.target as HTMLSelectElement).value as any)} aria-label="Vue">
          <option value="books">Livres</option>
          <option value="author">Auteurs</option>
          <option value="series">Séries</option>
          <option value="tag">Tags</option>
        </select>
      </div>

      {#if browseMode === "books" && view.kind === "all" && !query.trim()}
        <div class="sort">
          <select value={sort} onchange={(e) => setSort((e.target as HTMLSelectElement).value as any)}>
            <option value="title">Titre</option>
            <option value="author">Auteur</option>
            <option value="added">Ajout récent</option>
          </select>
        </div>
      {/if}

      {#if browseMode === "books"}
        <div class="viewtoggle" role="group" aria-label="Affichage">
          <button class:active={viewMode === "grid"} onclick={() => (viewMode = "grid")} aria-label="Grille">
            <svg viewBox="0 0 24 24" width="16" height="16"><path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z" fill="currentColor"/></svg>
          </button>
          <button class:active={viewMode === "list"} onclick={() => (viewMode = "list")} aria-label="Liste">
            <svg viewBox="0 0 24 24" width="16" height="16"><path d="M4 6h16M4 12h16M4 18h16" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
          </button>
        </div>
      {/if}

      <button class="import" onclick={doImport} disabled={importing}>
        {importing ? "Import…" : "Importer"}
      </button>
    </header>

    {#if importing && progress}
      <div class="progress">
        <div class="fill" style="width:{pct}%"></div>
        <span class="ptext ellipsis">{progress.title || "…"} · {progress.done}/{progress.total}</span>
      </div>
    {/if}

    {#if selectedIds.length}
      <div class="batchbar">
        <span class="sel">{selectedIds.length} sélectionné{selectedIds.length === 1 ? "" : "s"}</span>
        <input bind:value={batchSeries} placeholder="Série" aria-label="Série" />
        <input class="small" bind:value={batchSeriesStart} placeholder="Tome départ" aria-label="Tome de départ" />
        <button onclick={applyBatchSeries}>Assigner</button>
        {#if calibre?.connected}
          <button class="send" onclick={sendToDevice} disabled={sending}>
            {sending ? "Envoi…" : `Envoyer vers ${calibre.device || "la liseuse"}`}
          </button>
        {/if}
        <button class="ghost" onclick={clearSelection}>Annuler</button>
      </div>
    {/if}

    <div class="content">
      {#if browseMode !== "books"}
        {#if groupItems.length === 0}
          <div class="empty">
            <p>{query.trim() ? `Aucun groupe pour « ${query} ».` : "Aucun groupe à afficher."}</p>
          </div>
        {:else if browseMode === "author"}
          <GroupGrid items={groupItems} kind="author" onOpen={(item) => openGroup("author", item)} />
        {:else if browseMode === "series"}
          <GroupGrid items={groupItems} kind="series" onOpen={(item) => openGroup("series", item)} />
        {:else if browseMode === "tag"}
          <GroupGrid items={groupItems} kind="tag" onOpen={(item) => openGroup("tag", item)} />
        {/if}
      {:else if loading && books.length === 0}
        <p class="state">Chargement…</p>
      {:else if books.length === 0}
        <div class="empty">
          {#if query.trim()}
            <p>Aucun résultat pour « {query} ».</p>
          {:else if view.kind === "all"}
            <p>Votre bibliothèque est vide.</p>
            <p class="sub">Importez des fichiers ou un dossier — ou glissez des EPUB ici.</p>
            <button class="import big" onclick={doImport}>Importer des livres…</button>
          {:else}
            <p>Aucun livre ici.</p>
          {/if}
        </div>
      {:else}
        <BookGrid {books} mode={viewMode} {selectedIds} onOpen={openBook} onToggleSelect={toggleSelect} />
      {/if}
    </div>
  </main>
</div>

{#if detail}
  <BookDetailView
    book={detail}
    onClose={() => (detail = null)}
    onRemove={removeBook}
    onSave={saveBook}
    onSetTitleSort={setTitleSort}
    onSetAuthorSort={setAuthorSort}
  />
{/if}

{#if settingsOpen && settings && opdsStatus}
  <SettingsModal
    {settings}
    {opdsStatus}
    {calibre}
    onSetMode={setMode}
    onChooseFolder={chooseFolder}
    onSetRemotePathTemplate={setRemotePathTemplate}
    onSetOPDSEnabled={setOPDSEnabled}
    onSetOPDSPort={setOPDSPort}
    onSetCalibreEnabled={setCalibreEnabled}
    onSetWriteMetadataToFile={setWriteMetadataToFile}
    onRegenerateCovers={regenerateCovers}
    onSetTheme={setTheme}
    onClose={() => (settingsOpen = false)}
  />
{/if}

{#if toast}
  <div class="toast">{toast}</div>
{/if}

{#if dragOver}
  <div class="dropzone file-drop-target-active">
    <div class="dropcard">Déposez vos livres pour les importer</div>
  </div>
{/if}

<style>
  .app {
    display: flex;
    height: 100vh;
    min-height: 0;
  }
  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.9rem 1.5rem;
    border-bottom: 1px solid var(--border);
    /* Leave room for the macOS traffic lights over the translucent titlebar. */
    padding-top: 1.4rem;
  }
  .title {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-width: 0;
  }
  .back {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    flex: 0 0 auto;
    padding: 0.35rem 0.55rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--muted);
    font: inherit;
    font-size: 0.8rem;
    cursor: pointer;
  }
  .back:hover {
    color: var(--text);
    background: var(--surface-hi);
    border-color: var(--border-hi);
  }
  .back svg {
    width: 15px;
    height: 15px;
  }
  .title h1 {
    margin: 0;
    font-size: 1.15rem;
    letter-spacing: -0.01em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .title .n {
    color: var(--faint);
    font-size: 0.85rem;
    font-variant-numeric: tabular-nums;
  }

  .search {
    position: relative;
    margin-left: auto;
    width: min(320px, 34vw);
  }
  .search svg {
    position: absolute;
    left: 0.6rem;
    top: 50%;
    transform: translateY(-50%);
    width: 15px;
    height: 15px;
    color: var(--faint);
  }
  .search input {
    width: 100%;
    padding: 0.5rem 0.7rem 0.5rem 2rem;
    font: inherit;
    font-size: 0.88rem;
    color: var(--text);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 9px;
    outline: none;
  }
  .search input:focus {
    border-color: var(--border-hi);
    background: var(--surface-hi);
  }

  .sort select {
    font: inherit;
    font-size: 0.85rem;
    color: var(--text);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 9px;
    padding: 0.45rem 0.6rem;
    outline: none;
  }

  .viewtoggle {
    display: flex;
    gap: 2px;
    padding: 3px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 9px;
  }
  .viewtoggle button {
    display: grid;
    place-items: center;
    width: 30px;
    height: 26px;
    border: none;
    border-radius: 6px;
    background: none;
    color: var(--muted);
    cursor: pointer;
  }
  .viewtoggle button.active {
    background: var(--surface-hi);
    color: var(--text);
  }

  .import {
    font: inherit;
    font-weight: 600;
    font-size: 0.85rem;
    color: var(--accent-ink);
    background: var(--accent);
    border: none;
    border-radius: 9px;
    padding: 0.5rem 1rem;
    cursor: pointer;
    transition: filter 0.15s, opacity 0.15s;
  }
  .import:hover:not(:disabled) {
    filter: brightness(1.08);
  }
  .import:disabled {
    opacity: 0.55;
    cursor: default;
  }

  .progress {
    position: relative;
    height: 26px;
    display: flex;
    align-items: center;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    overflow: hidden;
  }

  .batchbar {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.65rem 1.5rem;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
  }
  .batchbar .sel {
    color: var(--muted);
    font-size: 0.82rem;
    white-space: nowrap;
  }
  .batchbar input {
    min-width: 0;
    width: min(260px, 24vw);
    padding: 0.45rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel);
    color: var(--text);
    font: inherit;
    font-size: 0.84rem;
    outline: none;
  }
  .batchbar input.small {
    width: 120px;
  }
  .batchbar input:focus {
    border-color: var(--border-hi);
    background: var(--surface-hi);
  }
  .batchbar button {
    padding: 0.45rem 0.75rem;
    border: none;
    border-radius: 8px;
    background: var(--accent);
    color: var(--accent-ink);
    font: inherit;
    font-size: 0.82rem;
    font-weight: 650;
    cursor: pointer;
  }
  .batchbar button.ghost {
    border: 1px solid var(--border);
    background: transparent;
    color: var(--muted);
    font-weight: 500;
  }
  .batchbar button.send {
    background: transparent;
    color: var(--ok);
    border: 1px solid color-mix(in srgb, var(--ok) 45%, var(--border));
  }
  .batchbar button.send:hover:not(:disabled) {
    background: color-mix(in srgb, var(--ok) 14%, transparent);
  }
  .batchbar button.send:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .progress .fill {
    position: absolute;
    inset: 0 auto 0 0;
    background: color-mix(in srgb, var(--accent) 28%, transparent);
    transition: width 0.2s ease;
  }
  .ptext {
    position: relative;
    padding: 0 1.5rem;
    font-size: 0.76rem;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }

  .content {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
  .state {
    color: var(--muted);
    padding: 2rem 1.5rem;
  }
  .empty {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.4rem;
    color: var(--muted);
    text-align: center;
  }
  .empty .sub {
    color: var(--faint);
    font-size: 0.9rem;
  }
  .empty .big {
    margin-top: 1rem;
    padding: 0.7rem 1.3rem;
  }

  .toast {
    position: fixed;
    bottom: 1.5rem;
    left: 50%;
    z-index: 60;
    transform: translateX(-50%);
    padding: 0.7rem 1.2rem;
    background: var(--panel);
    border: 1px solid var(--border-hi);
    border-radius: 10px;
    box-shadow: 0 12px 30px rgba(0, 0, 0, 0.5);
    font-size: 0.85rem;
    animation: rise 0.2s ease;
  }
  @keyframes rise {
    from {
      transform: translate(-50%, 8px);
      opacity: 0;
    }
  }

  .dropzone {
    position: fixed;
    inset: 0;
    z-index: 70;
    display: grid;
    place-items: center;
    background: rgba(10, 12, 20, 0.55);
    backdrop-filter: blur(2px);
    pointer-events: none;
  }
  .dropcard {
    padding: 2rem 3rem;
    border: 2px dashed var(--accent);
    border-radius: 16px;
    color: var(--text);
    font-size: 1.05rem;
    background: rgba(0, 0, 0, 0.3);
  }

  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
