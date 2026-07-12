<script lang="ts">
  import { LibraryService } from "./api";
  import type { BookDetail, BookUpdate, OnlineCandidate, ApplyMetadataInput } from "./api";
  import { t } from "./i18n";

  let {
    book,
    onClose,
    onApply,
  }: {
    book: BookDetail;
    onClose: () => void;
    onApply: (input: ApplyMetadataInput) => Promise<void> | void;
  } = $props();

  // Search hints, prefilled once from the book the modal opened on (it is
  // recreated per open, so the initial snapshot is what we want). Authors are
  // semicolon-joined so a "Nom, Prénom" author stays one value (comma is
  // ambiguous for people whose display form is "Last, First").
  // svelte-ignore state_referenced_locally
  const initial = book;
  let qTitle = $state(initial.title ?? "");
  let qAuthors = $state((initial.authors ?? []).map((a) => a.name).join(" ; "));
  let qIsbn = $state(initial.isbn ?? "");
  let qLang = $state(initial.language ?? "");

  let searching = $state(false);
  let searched = $state(false);
  let error = $state("");
  let candidates = $state<OnlineCandidate[]>([]);
  let selected = $state<OnlineCandidate | null>(null);
  let applying = $state(false);

  // Field definitions: label + how to read the current book value and the
  // candidate value. Authors/tags are multi-value; authors split on ";" only.
  type FieldKey =
    | "title" | "authors" | "series" | "seriesIndex"
    | "description" | "language" | "isbn" | "published" | "tags";

  const fields: { key: FieldKey; label: string; multiline?: boolean; authors?: boolean }[] = [
    { key: "title", label: t("metadata.field.title") },
    { key: "authors", label: t("metadata.field.authors"), authors: true },
    { key: "series", label: t("metadata.field.series") },
    { key: "seriesIndex", label: t("quick.seriesIndex") },
    { key: "language", label: t("metadata.field.language") },
    { key: "published", label: t("metadata.field.published") },
    { key: "isbn", label: "ISBN" },
    { key: "tags", label: t("metadata.field.tags") },
    { key: "description", label: t("metadata.field.description"), multiline: true },
  ];

  function currentValue(key: FieldKey): string {
    switch (key) {
      case "title": return book.title ?? "";
      case "authors": return (book.authors ?? []).map((a) => a.name).join(" ; ");
      case "series": return book.series ?? "";
      case "seriesIndex": return book.seriesIndex ? String(book.seriesIndex) : "";
      case "description": return book.description ?? "";
      case "language": return book.language ?? "";
      case "isbn": return book.isbn ?? "";
      case "published": return book.published ?? "";
      case "tags": return (book.tags ?? []).join(", ");
    }
  }

  function candidateValue(c: OnlineCandidate, key: FieldKey): string {
    switch (key) {
      case "title": return c.title ?? "";
      case "authors": return (c.authors ?? []).join(" ; ");
      case "series": return c.series ?? "";
      case "seriesIndex": return c.seriesIndex ?? "";
      case "description": return c.description ?? "";
      case "language": return c.language ?? "";
      case "isbn": return c.isbn ?? "";
      case "published": return c.published ?? "";
      case "tags": return (c.tags ?? []).join(", ");
    }
  }

  // The editable merge state, rebuilt each time a candidate is picked. Each
  // field carries whether to apply it and its (editable) value; the cover is a
  // separate toggle since it is applied as a URL, not a text field.
  let merge = $state<Record<FieldKey, { apply: boolean; value: string }>>(blankMerge());
  let coverApply = $state(false);

  function blankMerge(): Record<FieldKey, { apply: boolean; value: string }> {
    const m = {} as Record<FieldKey, { apply: boolean; value: string }>;
    for (const f of fields) m[f.key] = { apply: false, value: "" };
    return m;
  }

  function selectCandidate(c: OnlineCandidate) {
    selected = c;
    const m = blankMerge();
    for (const f of fields) {
      const val = candidateValue(c, f.key);
      // Default to applying a field only when it brings something new.
      m[f.key] = { apply: val !== "" && val !== currentValue(f.key), value: val };
    }
    merge = m;
    coverApply = !!c.coverUrl;
  }

  async function search() {
    if (searching) return;
    searching = true;
    error = "";
    try {
      const res = await LibraryService.SearchOnlineMetadata(
        book.id, qTitle, qAuthors, qIsbn, qLang,
      );
      candidates = res?.candidates ?? [];
      searched = true;
      selected = null;
      if (candidates.length > 0) selectCandidate(candidates[0]);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      candidates = [];
    } finally {
      searching = false;
    }
  }

  // Flip "Nom, Prénom" → "Prénom Nom" for each author in the field, so the user
  // can normalise a source that returns the sort form.
  function flipAuthors() {
    merge.authors.value = merge.authors.value
      .split(";")
      .map((a) => a.trim())
      .filter(Boolean)
      .map((a) => {
        const i = a.indexOf(",");
        return i > 0 ? `${a.slice(i + 1).trim()} ${a.slice(0, i).trim()}`.trim() : a;
      })
      .join(" ; ");
  }

  function splitAuthors(s: string): string[] {
    return s.split(/[;\n]/).map((x) => x.trim()).filter(Boolean);
  }
  function splitTags(s: string): string[] {
    return s.split(/[,;\n]/).map((x) => x.trim()).filter(Boolean);
  }

  function pick(key: FieldKey): string {
    return merge[key].apply ? merge[key].value.trim() : currentValue(key);
  }

  async function apply() {
    if (applying) return;
    applying = true;
    try {
      const update: BookUpdate = {
        id: book.id,
        title: pick("title") || book.title,
        titleSort: book.titleSort,
        authors: splitAuthors(pick("authors")),
        series: pick("series"),
        seriesIndex: pick("seriesIndex"),
        description: pick("description"),
        language: pick("language"),
        isbn: pick("isbn"),
        published: pick("published"),
        tags: splitTags(pick("tags")),
        remotePathOverrideEnabled: book.remotePathOverrideEnabled,
        remotePathOverride: book.remotePathOverride,
      };
      await onApply({
        book: update,
        coverUrl: coverApply && selected?.coverUrl ? selected.coverUrl : "",
      });
    } finally {
      applying = false;
    }
  }

  const sourceLabels: Record<string, string> = {
    googlebooks: "Google Books",
    openlibrary: "OpenLibrary",
    bnf: "BnF",
  };

  const nothingSelected = $derived(!fields.some((f) => merge[f.key].apply) && !coverApply);

  // Run an initial search on open.
  $effect(() => {
    if (!searched && !searching) search();
  });
</script>

<div class="scrim" onclick={onClose} role="presentation"></div>

<div class="modal" role="dialog" aria-label={t("metadata.online.title")}>
  <header class="head">
    <h2>{t("metadata.online.title")}</h2>
    <button class="close" onclick={onClose} aria-label={t("common.close")}>✕</button>
  </header>

  <div class="searchbar">
    <input bind:value={qTitle} placeholder={t("metadata.field.title")} aria-label={t("metadata.field.title")} onkeydown={(e) => e.key === "Enter" && search()} />
    <input bind:value={qAuthors} placeholder={t("metadata.online.author")} aria-label={t("metadata.online.author")} onkeydown={(e) => e.key === "Enter" && search()} />
    <input class="isbn" bind:value={qIsbn} placeholder="ISBN" aria-label="ISBN" onkeydown={(e) => e.key === "Enter" && search()} />
    <input class="lang" bind:value={qLang} placeholder={t("metadata.online.language")} aria-label={t("metadata.online.preferredLanguage")} title={t("metadata.online.preferredLanguageTitle")} onkeydown={(e) => e.key === "Enter" && search()} />
    <button class="go" onclick={search} disabled={searching}>{searching ? "..." : t("common.search")}</button>
  </div>

  <div class="body">
    <div class="results">
      {#if searching}
        <p class="hint">{t("metadata.online.searching")}</p>
      {:else if error}
        <p class="err">{error}</p>
      {:else if candidates.length === 0}
        <p class="hint">{searched ? t("metadata.online.noResult") : ""}</p>
      {:else}
        {#each candidates as c (c.id)}
          <button
            class="cand"
            class:sel={selected?.id === c.id}
            onclick={() => selectCandidate(c)}
          >
            <div class="thumb">
              {#if c.coverUrl}<img src={c.coverUrl} alt="" loading="lazy" />{:else}<span class="noimg">{t("common.none")}</span>{/if}
            </div>
            <div class="cinfo">
              <span class="ctitle">{c.title}</span>
              {#if c.authors?.length}<span class="cauth">{c.authors.join(", ")}</span>{/if}
              <span class="cmeta">
                {#if c.language}<span class="badge">{c.language}</span>{/if}
                {#if c.published}<span>{c.published}</span>{/if}
                {#if c.publisher}<span class="ellipsis">· {c.publisher}</span>{/if}
              </span>
              <span class="src">{sourceLabels[c.source] ?? c.source}</span>
            </div>
          </button>
        {/each}
      {/if}
    </div>

    <div class="merge">
      {#if selected}
        <p class="mergehint">{t("metadata.online.mergeHint")}</p>

        <div class="coverrow">
          <label class="chk">
            <input type="checkbox" bind:checked={coverApply} disabled={!selected.coverUrl} />
            <span>{t("metadata.online.cover")}</span>
          </label>
          <div class="coverprev">
            {#if selected.coverUrl}
              <img src={selected.coverUrl} alt="" />
            {:else}<span class="noimg small">{t("metadata.online.noCover")}</span>{/if}
          </div>
        </div>

        {#each fields as f (f.key)}
          {@const cur = currentValue(f.key)}
          <div class="frow" class:on={merge[f.key].apply}>
            <label class="chk">
              <input type="checkbox" bind:checked={merge[f.key].apply} />
              <span>{f.label}</span>
            </label>
            <div class="vals">
              {#if f.multiline}
                <textarea bind:value={merge[f.key].value} rows="4" disabled={!merge[f.key].apply}></textarea>
              {:else}
                <div class="inline">
                  <input bind:value={merge[f.key].value} disabled={!merge[f.key].apply} />
                  {#if f.authors && merge[f.key].value.includes(",")}
                    <button class="flip" title={t("metadata.flipName")} onclick={flipAuthors} disabled={!merge[f.key].apply}>⇄</button>
                  {/if}
                </div>
              {/if}
              {#if cur}<span class="cur" title={cur}>{t("metadata.online.current", undefined, { value: cur })}</span>{/if}
            </div>
          </div>
        {/each}
      {:else if !searching}
        <p class="hint centered">{t("metadata.online.selectEdition")}</p>
      {/if}
    </div>
  </div>

  <footer class="foot">
    <span class="foothint">
      {#if selected}{t("metadata.online.source", undefined, { source: sourceLabels[selected.source] ?? selected.source })}{/if}
    </span>
    <div class="footbtns">
      <button class="ghost" onclick={onClose}>{t("common.cancel")}</button>
      <button class="primary" onclick={apply} disabled={!selected || applying || nothingSelected}>
        {applying ? t("metadata.online.applying") : t("metadata.online.apply")}
      </button>
    </div>
  </footer>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 50;
    background: rgba(0, 0, 0, 0.55);
    animation: fade 0.15s ease;
  }
  @keyframes fade { from { opacity: 0; } }
  .modal {
    position: fixed;
    z-index: 51;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: min(940px, 94vw);
    height: min(760px, 90vh);
    display: flex;
    flex-direction: column;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 14px;
    box-shadow: 0 30px 80px rgba(0, 0, 0, 0.55);
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border);
  }
  .head h2 {
    margin: 0;
    font-size: 1.05rem;
  }
  .close {
    width: 2rem;
    height: 2rem;
    border-radius: 50%;
    border: none;
    background: var(--surface-hi);
    color: var(--muted);
    cursor: pointer;
  }
  .close:hover { color: var(--text); }

  .searchbar {
    display: flex;
    gap: 0.5rem;
    padding: 0.85rem 1.25rem;
    border-bottom: 1px solid var(--border);
  }
  .searchbar input {
    min-width: 0;
    flex: 1;
    padding: 0.5rem 0.65rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 0.85rem;
    outline: none;
  }
  .searchbar input.isbn { flex: 0 0 130px; }
  .searchbar input.lang { flex: 0 0 80px; }
  .searchbar input:focus { border-color: var(--border-hi); background: var(--surface-hi); }
  .go {
    flex: none;
    padding: 0.5rem 0.9rem;
    border: none;
    border-radius: 8px;
    background: var(--accent);
    color: var(--accent-ink);
    font: inherit;
    font-weight: 650;
    font-size: 0.84rem;
    cursor: pointer;
  }
  .go:disabled { opacity: 0.6; cursor: default; }

  .body {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 300px minmax(0, 1fr);
  }
  .results {
    overflow-y: auto;
    border-right: 1px solid var(--border);
    padding: 0.5rem;
  }
  .cand {
    display: flex;
    gap: 0.6rem;
    width: 100%;
    text-align: left;
    padding: 0.5rem;
    margin-bottom: 0.35rem;
    border: 1px solid transparent;
    border-radius: 9px;
    background: none;
    color: inherit;
    cursor: pointer;
  }
  .cand:hover { background: var(--surface); }
  .cand.sel {
    background: var(--surface-hi);
    border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
  }
  .thumb {
    flex: 0 0 46px;
    width: 46px;
    height: 68px;
    border-radius: 5px;
    overflow: hidden;
    background: var(--inset);
    display: grid;
    place-items: center;
  }
  .thumb img { width: 100%; height: 100%; object-fit: cover; }
  .noimg { color: var(--faint); font-size: 0.8rem; }
  .noimg.small { font-size: 0.7rem; }
  .cinfo {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .ctitle {
    font-size: 0.84rem;
    font-weight: 600;
    line-height: 1.2;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
  }
  .cauth {
    font-size: 0.76rem;
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cmeta {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    align-items: center;
    font-size: 0.7rem;
    color: var(--faint);
    min-width: 0;
  }
  .cmeta .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 130px;
  }
  .badge {
    text-transform: uppercase;
    font-size: 0.62rem;
    letter-spacing: 0.04em;
    padding: 0.05rem 0.3rem;
    border-radius: 4px;
    background: var(--surface-hi);
    color: var(--muted);
  }
  .src {
    font-size: 0.66rem;
    color: var(--accent);
    margin-top: 0.1rem;
  }

  .merge {
    overflow-y: auto;
    padding: 0.9rem 1.1rem;
  }
  .mergehint, .hint, .err {
    font-size: 0.8rem;
    color: var(--muted);
  }
  .hint.centered { text-align: center; margin-top: 3rem; }
  .err { color: var(--danger); }
  .mergehint { margin: 0 0 0.9rem; }

  .coverrow {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.6rem 0 0.9rem;
    border-bottom: 1px solid var(--border);
    margin-bottom: 0.7rem;
  }
  .coverprev {
    width: 52px;
    height: 76px;
    border-radius: 5px;
    overflow: hidden;
    background: var(--inset);
    display: grid;
    place-items: center;
  }
  .coverprev img { width: 100%; height: 100%; object-fit: cover; }

  .frow {
    display: grid;
    grid-template-columns: 120px minmax(0, 1fr);
    gap: 0.7rem;
    align-items: start;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--border);
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    color: var(--muted);
    font-size: 0.82rem;
    cursor: pointer;
    padding-top: 0.3rem;
  }
  .chk input {
    width: 15px;
    height: 15px;
    accent-color: var(--accent);
    flex: none;
  }
  .frow.on .chk { color: var(--text); }
  .vals { min-width: 0; display: grid; gap: 0.25rem; }
  .inline { display: flex; gap: 0.35rem; }
  .vals input, .vals textarea {
    width: 100%;
    min-width: 0;
    padding: 0.42rem 0.55rem;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 0.82rem;
    outline: none;
    user-select: text;
  }
  .vals textarea { resize: vertical; line-height: 1.45; }
  .vals input:focus, .vals textarea:focus { border-color: var(--border-hi); background: var(--surface-hi); }
  .vals input:disabled, .vals textarea:disabled { opacity: 0.5; }
  .flip {
    flex: none;
    width: 32px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: var(--surface);
    color: var(--muted);
    cursor: pointer;
    font-size: 0.9rem;
  }
  .flip:hover:not(:disabled) { color: var(--text); background: var(--surface-hi); }
  .flip:disabled { opacity: 0.4; cursor: default; }
  .cur {
    font-size: 0.72rem;
    color: var(--faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.85rem 1.25rem;
    border-top: 1px solid var(--border);
  }
  .foothint { font-size: 0.76rem; color: var(--faint); }
  .footbtns { display: flex; gap: 0.6rem; }
  .footbtns button {
    padding: 0.55rem 1rem;
    border-radius: 8px;
    font: inherit;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .ghost {
    border: 1px solid var(--border);
    background: transparent;
    color: var(--muted);
  }
  .ghost:hover { color: var(--text); background: var(--surface); }
  .primary {
    border: none;
    background: var(--accent);
    color: var(--accent-ink);
    font-weight: 650;
  }
  .primary:disabled { opacity: 0.55; cursor: default; }
</style>
