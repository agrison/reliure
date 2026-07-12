<script lang="ts">
  import { LibraryService } from "./api";
  import type { DiscoverResult, DiscoverBook } from "./api";
  import { plural, t } from "./i18n";

  // Self-contained "Découvrir" view: browse legal public-domain providers and
  // add EPUBs to the library. Imports go through the normal pipeline, so the global
  // import:done event (handled in App) refreshes the library and shows a toast;
  // here we track per-card state for immediate feedback.
  let query = $state("");
  let lang = $state("fr");
  let source = $state("all");
  let loading = $state(false);
  let slow = $state(false);
  let error = $state("");
  let result = $state<DiscoverResult | null>(null);
  let slowTimer: ReturnType<typeof setTimeout>;
  let adding = $state<Record<string, boolean>>({});
  let added = $state<Record<string, "added" | "duplicate" | "failed">>({});

  const languages = [
    { code: "fr", label: t("language.fr") },
    { code: "en", label: t("language.en") },
    { code: "es", label: t("language.es") },
    { code: "de", label: t("language.de") },
    { code: "it", label: t("language.it") },
    { code: "", label: t("discover.language.all") },
  ];
  const sources = [
    { code: "all", label: t("discover.source.all") },
    { code: "gutenberg", label: t("discover.source.gutenberg") },
    { code: "standardebooks", label: t("discover.source.standardebooks") },
  ];

  async function search(page = 1) {
    if (loading) return;
    loading = true;
    slow = false;
    error = "";
    // Warn only if it's actually dragging: the provider catalog may be cold.
    clearTimeout(slowTimer);
    slowTimer = setTimeout(() => (slow = true), 4000);
    try {
      result = await LibraryService.SearchDiscover(source, query.trim(), lang, page);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      result = null;
    } finally {
      clearTimeout(slowTimer);
      loading = false;
      slow = false;
    }
  }

  function bookKey(b: DiscoverBook): string {
    return `${b.source}:${b.id}`;
  }

  async function addBook(b: DiscoverBook) {
    const key = bookKey(b);
    if (adding[key] || added[key] === "added") return;
    adding = { ...adding, [key]: true };
    try {
      const res = await LibraryService.ImportDiscoverBook(b.source, b.id);
      const state = res.imported > 0 || res.attached > 0 ? "added" : res.duplicates > 0 ? "duplicate" : "failed";
      added = { ...added, [key]: state };
    } catch (e) {
      console.error("[Reliure] ImportDiscoverBook failed", e);
      added = { ...added, [key]: "failed" };
    } finally {
      const next = { ...adding };
      delete next[key];
      adding = next;
    }
  }

  function addLabel(b: DiscoverBook): string {
    const key = bookKey(b);
    if (adding[key]) return t("discover.adding");
    switch (added[key]) {
      case "added": return t("discover.status.added");
      case "duplicate": return t("discover.status.duplicate");
      case "failed": return t("discover.status.failed");
      default: return t("discover.add");
    }
  }

  function sourceLabel(b: DiscoverBook): string {
    return b.source === "standardebooks" ? t("discover.source.standardebooks") : t("discover.source.gutenberg");
  }

  function changeSource(next: string) {
    source = next;
    if (source === "standardebooks" && lang === "fr") {
      lang = "en";
    }
    search(1);
  }

  const totalLabel = $derived(
    result ? t("discover.total", undefined, { count: result.count.toLocaleString("fr-FR"), s: plural(result.count) }) : "",
  );

  // Initial browse: popular French books.
  $effect(() => {
    if (!result && !loading && !error) search(1);
  });
</script>

<div class="discover">
  <div class="bar">
    <div class="searchwrap">
      <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" fill="none" stroke="currentColor" stroke-width="2" /><path d="M21 21l-4.3-4.3" stroke="currentColor" stroke-width="2" stroke-linecap="round" /></svg>
      <input
        placeholder={t("discover.searchPlaceholder")}
        bind:value={query}
        onkeydown={(e) => e.key === "Enter" && search(1)}
      />
    </div>
    <select bind:value={lang} onchange={() => search(1)} aria-label={t("settings.language.label")}>
      {#each languages as l (l.code)}<option value={l.code}>{l.label}</option>{/each}
    </select>
    <select value={source} onchange={(e) => changeSource((e.target as HTMLSelectElement).value)} aria-label={t("discover.source.label")}>
      {#each sources as s (s.code)}<option value={s.code}>{s.label}</option>{/each}
    </select>
    <button class="go" onclick={() => search(1)} disabled={loading}>{loading ? "..." : t("common.search")}</button>
    <span class="src">{t("discover.publicDomain")}</span>
  </div>

  {#if slow}
    <p class="slow">{t("discover.loadingCatalog")}</p>
  {/if}

  {#if error}
    <p class="msg err">{error}</p>
  {:else if loading && !result}
    <p class="msg">{t("common.loading")}</p>
  {:else if result}
    {@const r = result}
    <div class="meta">
      <span>{totalLabel}</span>
      {#if r.count > 0}
        <div class="pager">
          <button onclick={() => search(r.page - 1)} disabled={!r.hasPrevious || loading}>‹</button>
          <span>{t("discover.page", undefined, { page: r.page })}</span>
          <button onclick={() => search(r.page + 1)} disabled={!r.hasNext || loading}>›</button>
        </div>
      {/if}
    </div>

    {#if (r.books ?? []).length === 0}
      <p class="msg">{t("discover.empty")}</p>
    {:else}
      <div class="grid">
        {#each r.books ?? [] as b (bookKey(b))}
          <article class="card">
            <div class="cover">
              <img src={b.cover} alt="" loading="lazy" onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = "none")} />
              <span class="noimg">📖</span>
            </div>
            <div class="info">
              <h3 title={b.title}>{b.title}</h3>
              {#if b.authors?.length}<p class="auth" title={b.authors.join(", ")}>{b.authors.join(", ")}</p>{/if}
              <div class="tags">
                <span class="badge source-badge">{sourceLabel(b)}</span>
                {#each b.languages ?? [] as l}<span class="badge">{l}</span>{/each}
                {#if b.subjects?.length}<span class="subj ellipsis" title={b.subjects.join(" · ")}>{b.subjects[0]}</span>{/if}
              </div>
              <button
                class="add"
                class:done={added[bookKey(b)] === "added"}
                class:dupe={added[bookKey(b)] === "duplicate"}
                class:fail={added[bookKey(b)] === "failed"}
                onclick={() => addBook(b)}
                disabled={adding[bookKey(b)] || added[bookKey(b)] === "added" || !b.hasEpub}
                title={b.hasEpub ? "" : t("discover.noEpub")}
              >
                {b.hasEpub ? addLabel(b) : t("discover.noEpub.short")}
              </button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .discover {
    padding: 1.25rem 1.5rem 3rem;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
    margin-bottom: 1rem;
  }
  .searchwrap {
    position: relative;
    flex: 1;
    min-width: 220px;
    max-width: 420px;
  }
  .searchwrap svg {
    position: absolute;
    left: 0.6rem;
    top: 50%;
    transform: translateY(-50%);
    width: 15px;
    height: 15px;
    color: var(--faint);
  }
  .searchwrap input {
    width: 100%;
    padding: 0.55rem 0.7rem 0.55rem 2rem;
    font: inherit;
    font-size: 0.9rem;
    color: var(--text);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 9px;
    outline: none;
  }
  .searchwrap input:focus {
    border-color: var(--border-hi);
    background: var(--surface-hi);
  }
  select {
    min-width: 8rem;
  }
  .go {
    padding: 0.55rem 1rem;
    border: none;
    border-radius: 9px;
    background: var(--accent);
    color: var(--accent-ink);
    font: inherit;
    font-weight: 650;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .go:disabled { opacity: 0.6; cursor: default; }
  .src {
    margin-left: auto;
    font-size: 0.76rem;
    color: var(--faint);
  }

  .meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.85rem;
    color: var(--muted);
    font-size: 0.82rem;
  }
  .pager {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .pager button {
    width: 28px;
    height: 28px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: var(--surface);
    color: var(--text);
    font-size: 1rem;
    line-height: 1;
    cursor: pointer;
  }
  .pager button:disabled { opacity: 0.4; cursor: default; }

  .msg {
    color: var(--muted);
    padding: 1.5rem 0;
  }
  .msg.err { color: var(--danger); }
  .slow {
    margin: -0.3rem 0 0.9rem;
    padding: 0.5rem 0.7rem;
    border: 1px solid color-mix(in srgb, var(--accent) 30%, var(--border));
    border-radius: 8px;
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    color: var(--muted);
    font-size: 0.78rem;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 0.9rem;
  }
  .card {
    display: flex;
    gap: 0.75rem;
    padding: 0.75rem;
    border: 1px solid var(--border);
    border-radius: 11px;
    background: var(--panel);
  }
  .cover {
    position: relative;
    flex: 0 0 68px;
    width: 68px;
    height: 100px;
    border-radius: 6px;
    overflow: hidden;
    background: var(--inset);
    display: grid;
    place-items: center;
  }
  .cover img {
    position: relative;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  /* Sits behind the cover image; revealed when the image 404s (no cover). */
  .noimg {
    position: absolute;
    font-size: 1.5rem;
    opacity: 0.5;
  }
  .info {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    flex: 1;
  }
  .info h3 {
    margin: 0;
    font-size: 0.9rem;
    line-height: 1.25;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
  }
  .auth {
    margin: 0;
    font-size: 0.78rem;
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tags {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.35rem;
    margin-top: auto;
  }
  .badge {
    text-transform: uppercase;
    font-size: 0.62rem;
    letter-spacing: 0.04em;
    padding: 0.08rem 0.34rem;
    border-radius: 4px;
    background: var(--surface-hi);
    color: var(--muted);
  }
  .source-badge {
    text-transform: none;
    letter-spacing: 0;
    color: var(--text);
  }
  .subj {
    font-size: 0.7rem;
    color: var(--faint);
    min-width: 0;
  }
  .add {
    margin-top: 0.35rem;
    padding: 0.42rem 0.6rem;
    border: 1px solid color-mix(in srgb, var(--accent) 45%, var(--border));
    border-radius: 8px;
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--accent);
    font: inherit;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
  }
  .add:hover:not(:disabled) { background: color-mix(in srgb, var(--accent) 20%, transparent); }
  .add:disabled { cursor: default; }
  .add.done {
    color: var(--ok);
    border-color: color-mix(in srgb, var(--ok) 45%, var(--border));
    background: color-mix(in srgb, var(--ok) 12%, transparent);
    opacity: 1;
  }
  .add.dupe {
    color: var(--muted);
    border-color: var(--border);
    background: var(--surface);
  }
  .add.fail {
    color: var(--danger);
    border-color: color-mix(in srgb, var(--danger) 45%, var(--border));
    background: color-mix(in srgb, var(--danger) 10%, transparent);
  }
</style>
