<script lang="ts">
  import { LibraryService, type ContentOccurrencePage, type ContentSnippet, type SearchScope } from "./api";
  import { t } from "./i18n";

  let {
    query,
    scope,
    title,
    onBack,
    onOpen,
  }: {
    query: string;
    scope: SearchScope;
    title: string;
    onBack: () => void;
    onOpen: (id: number) => void;
  } = $props();

  let loading = $state(true);
  let page = $state(1);
  let result = $state<ContentOccurrencePage | null>(null);
  const perPage = 12;

  async function load(next = page) {
    loading = true;
    try {
      page = next;
      result = await LibraryService.ContentOccurrences(query, scope, page, perPage);
    } finally {
      loading = false;
    }
  }

  function highlighted(raw: string): string {
    return escapeHTML(raw)
      .replaceAll("[[[", '<mark class="hit">')
      .replaceAll("]]]", "</mark>");
  }

  function escapeHTML(s: string): string {
    return s
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }

  function pages(r: ContentOccurrencePage | null): string {
    if (!r || r.totalPages <= 0) return "0 / 0";
    return `${r.page} / ${r.totalPages}`;
  }

  function seriesLine(h: ContentSnippet): string {
    if (!h.series) return "";
    return h.seriesIndex ? `${h.series} · ${t("content.results.volume", undefined, { index: h.seriesIndex })}` : h.series;
  }

  $effect(() => {
    query;
    scope;
    load(1);
  });
</script>

<section class="occurrences">
  <header>
    <button class="back" onclick={onBack}>{t("common.back")}</button>
    <div>
      <h1>{t("content.occurrences.title")}</h1>
      <p>{title} · « {query} »</p>
    </div>
  </header>

  <div class="pager">
    <button onclick={() => load(1)} disabled={loading || !result || result.page <= 1}>{t("content.occurrences.first")}</button>
    <button onclick={() => load(Math.max(1, page - 1))} disabled={loading || !result || result.page <= 1}>{t("content.occurrences.previous")}</button>
    <span>{pages(result)}</span>
    <button onclick={() => load(Math.min(result?.totalPages || page, page + 1))} disabled={loading || !result || result.page >= result.totalPages}>{t("content.occurrences.next")}</button>
    <button onclick={() => load(result?.totalPages || 1)} disabled={loading || !result || result.page >= result.totalPages}>{t("content.occurrences.last")}</button>
  </div>

  {#if loading && !result}
    <p class="state">{t("common.loading")}</p>
  {:else if !result || result.items.length === 0}
    <p class="state">{t("content.occurrences.empty")}</p>
  {:else}
    <ol class="list">
      {#each result.items as h, i (`${h.bookId}-${h.page}-${i}`)}
        <li>
          <button class="book" onclick={() => onOpen(h.bookId)}>
            <span class="cover">
              {#if h.cover}<img src={h.cover} alt="" loading="lazy" />{:else}{h.title.slice(0, 1)}{/if}
            </span>
            <span>
              <strong>{h.title}</strong>
              <small>{h.authors || t("common.none")}</small>
              {#if seriesLine(h)}<small class="series">{seriesLine(h)}</small>{/if}
            </span>
          </button>
          <span class="page">{t("common.page.short", undefined, { page: h.page })}</span>
          <p>{@html highlighted(h.snippet)}</p>
        </li>
      {/each}
    </ol>
  {/if}
</section>

<style>
  .occurrences {
    padding: 1.5rem;
    max-width: 980px;
    margin: 0 auto;
  }
  header {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  h1,
  p {
    margin: 0;
  }
  h1 {
    font-size: 1.05rem;
  }
  header p,
  .state {
    color: var(--muted);
    font-size: 0.88rem;
    margin-top: 0.25rem;
  }
  .back,
  .pager button {
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 0.84rem;
    cursor: pointer;
  }
  .back {
    padding: 0.45rem 0.65rem;
  }
  .pager {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    margin: 1rem 0;
  }
  .pager button {
    padding: 0.42rem 0.6rem;
  }
  .pager button:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .pager span {
    color: var(--muted);
    font-size: 0.84rem;
    font-variant-numeric: tabular-nums;
    padding: 0 0.4rem;
  }
  .list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 0.95rem;
  }
  .list li {
    border-top: 1px solid var(--border);
    padding-top: 0.95rem;
    display: grid;
    grid-template-columns: minmax(180px, 260px) 3.6rem minmax(0, 1fr);
    gap: 0.9rem;
    align-items: start;
  }
  .book {
    display: flex;
    gap: 0.65rem;
    align-items: center;
    min-width: 0;
    padding: 0;
    border: none;
    background: none;
    color: inherit;
    text-align: left;
    font: inherit;
    cursor: pointer;
  }
  .cover {
    width: 32px;
    aspect-ratio: 2 / 3;
    border-radius: 5px;
    overflow: hidden;
    background: var(--surface);
    display: grid;
    place-items: center;
    flex: none;
    color: var(--muted);
    font-weight: 700;
  }
  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  strong,
  small {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  strong {
    font-size: 0.88rem;
  }
  small {
    margin-top: 0.1rem;
    color: var(--muted);
    font-size: 0.78rem;
  }
  small.series {
    color: var(--accent);
  }
  .page {
    color: var(--faint);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
    padding-top: 0.12rem;
  }
  .list p {
    font-size: 0.92rem;
    line-height: 1.6;
    color: var(--text);
  }
  :global(.occurrences mark.hit) {
    padding: 0.05em 0.18em;
    border-radius: 0.2em;
    background: #fff0a6 !important;
    box-shadow: inset 0 -0.48em 0 rgba(255, 222, 93, 0.58);
    color: #211800;
  }
  @media (max-width: 760px) {
    .occurrences {
      padding: 1rem;
    }
    .list li {
      grid-template-columns: 1fr;
      gap: 0.35rem;
    }
  }
</style>
