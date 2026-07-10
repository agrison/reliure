<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import {
    LibraryService,
    SettingsService,
    type BookCard,
    type BookDetail,
    type SidebarItem,
    type AppSettings,
    type ImportProgress,
    type ImportSummary,
  } from "./lib/api";
  import { type View, viewTitle } from "./lib/types";
  import Sidebar from "./lib/Sidebar.svelte";
  import BookGrid from "./lib/BookGrid.svelte";
  import BookDetailView from "./lib/BookDetail.svelte";
  import SettingsModal from "./lib/SettingsModal.svelte";

  let view = $state<View>({ kind: "all" });
  let sort = $state<"title" | "author" | "added">("title");
  let viewMode = $state<"grid" | "list">("grid");
  let query = $state("");
  let books = $state<BookCard[]>([]);
  let loading = $state(true);

  let total = $state(0);
  let authors = $state<SidebarItem[]>([]);
  let series = $state<SidebarItem[]>([]);
  let tags = $state<SidebarItem[]>([]);

  let detail = $state<BookDetail | null>(null);
  let settings = $state<AppSettings | null>(null);
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
      else if (view.kind === "author") res = await LibraryService.BooksByAuthor(view.id);
      else if (view.kind === "series") res = await LibraryService.BooksBySeries(view.id);
      else if (view.kind === "tag") res = await LibraryService.BooksByTag(view.id);
      else res = await LibraryService.Books(sort);
      books = res ?? [];
    } finally {
      loading = false;
    }
  }

  async function loadSidebar() {
    const [t, a, s, g] = await Promise.all([
      LibraryService.Stats(),
      LibraryService.Authors(),
      LibraryService.SeriesList(),
      LibraryService.Tags(),
    ]);
    total = t.books;
    authors = a ?? [];
    series = s ?? [];
    tags = g ?? [];
  }

  function selectView(v: View) {
    view = v;
    query = "";
    loadBooks();
  }

  let searchTimer: ReturnType<typeof setTimeout>;
  function onSearch(e: Event) {
    query = (e.target as HTMLInputElement).value;
    clearTimeout(searchTimer);
    searchTimer = setTimeout(loadBooks, 200);
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

  function doImport() {
    // Progress + completion arrive via events; no need to await the result.
    LibraryService.ChooseAndImport().catch((e) => console.error(e));
  }

  async function openSettings() {
    settings = await SettingsService.Get();
    settingsOpen = true;
  }
  async function setMode(m: "copy" | "reference") {
    settings = await SettingsService.SetImportMode(m);
  }
  async function chooseFolder() {
    settings = await SettingsService.ChooseLibraryFolder();
  }

  onMount(() => {
    loadSidebar();
    loadBooks();

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
      offProgress();
      offDone();
    };
  });

  const pct = $derived(
    progress && progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0,
  );
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

<div class="app">
  <Sidebar
    {total}
    {authors}
    {series}
    {tags}
    active={view}
    onSelect={selectView}
    onOpenSettings={openSettings}
  />

  <main class="main">
    <header class="toolbar">
      <div class="title">
        <h1>{viewTitle(view)}</h1>
        <span class="n">{books.length}</span>
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

      {#if view.kind === "all" && !query.trim()}
        <div class="sort">
          <select value={sort} onchange={(e) => setSort((e.target as HTMLSelectElement).value as any)}>
            <option value="title">Titre</option>
            <option value="author">Auteur</option>
            <option value="added">Ajout récent</option>
          </select>
        </div>
      {/if}

      <div class="viewtoggle" role="group" aria-label="Affichage">
        <button class:active={viewMode === "grid"} onclick={() => (viewMode = "grid")} aria-label="Grille">
          <svg viewBox="0 0 24 24" width="16" height="16"><path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z" fill="currentColor"/></svg>
        </button>
        <button class:active={viewMode === "list"} onclick={() => (viewMode = "list")} aria-label="Liste">
          <svg viewBox="0 0 24 24" width="16" height="16"><path d="M4 6h16M4 12h16M4 18h16" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        </button>
      </div>

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

    <div class="content">
      {#if loading && books.length === 0}
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
        <BookGrid {books} mode={viewMode} onOpen={openBook} />
      {/if}
    </div>
  </main>
</div>

{#if detail}
  <BookDetailView book={detail} onClose={() => (detail = null)} />
{/if}

{#if settingsOpen && settings}
  <SettingsModal
    {settings}
    onSetMode={setMode}
    onChooseFolder={chooseFolder}
    onClose={() => (settingsOpen = false)}
  />
{/if}

{#if toast}
  <div class="toast">{toast}</div>
{/if}

{#if dragOver}
  <div class="dropzone">
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
    align-items: baseline;
    gap: 0.6rem;
    min-width: 0;
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
