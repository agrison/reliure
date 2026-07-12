<script lang="ts">
  import type { ContentSnippet } from "./api";
  import { plural, t } from "./i18n";

  let {
    snippets,
    onOpen,
    onOpenOccurrences,
  }: {
    snippets: ContentSnippet[];
    onOpen: (id: number) => void;
    onOpenOccurrences: () => void;
  } = $props();

  const groups = $derived(groupSnippets(snippets));

  function groupSnippets(items: ContentSnippet[]) {
    const map = new Map<number, { book: ContentSnippet; hits: ContentSnippet[] }>();
    for (const hit of items ?? []) {
      let group = map.get(hit.bookId);
      if (!group) {
        group = { book: hit, hits: [] };
        map.set(hit.bookId, group);
      }
      group.hits.push(hit);
    }
    return [...map.values()];
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

  function seriesLine(h: ContentSnippet): string {
    if (!h.series) return "";
    return h.seriesIndex ? `${h.series} · ${t("content.results.volume", undefined, { index: h.seriesIndex })}` : h.series;
  }

  function moreLabel(count: number): string {
    return t("content.results.more", undefined, { count, s: plural(count) });
  }
</script>

{#if groups.length}
  <section class="content-results">
    <div class="head">
      <h2>{t("content.results.title")}</h2>
      <span>{t("content.results.books", undefined, { count: groups.length, s: plural(groups.length) })}</span>
    </div>

    {#each groups as g (g.book.bookId)}
      <article class="book">
        <button class="bookhead" onclick={() => onOpen(g.book.bookId)}>
          <div class="cover">
            {#if g.book.cover}
              <img src={g.book.cover} alt="" loading="lazy" />
            {:else}
              <span>{g.book.title.slice(0, 1)}</span>
            {/if}
          </div>
          <div class="meta">
            <h3>{g.book.title}</h3>
            <p>{g.book.authors || t("common.none")}</p>
            {#if seriesLine(g.book)}<p class="series">{seriesLine(g.book)}</p>{/if}
          </div>
        </button>

        <ol class="hits">
          {#each g.hits as h, i (`${h.bookId}-${h.page}-${i}`)}
            <li>
              <span class="page">{t("common.page.short", undefined, { page: h.page })}</span>
              <p>{@html highlighted(h.snippet)}</p>
            </li>
          {/each}
        </ol>
        {#if g.hits[g.hits.length - 1]?.more > 0}
          <button class="more" onclick={onOpenOccurrences}>
            {moreLabel(g.hits[g.hits.length - 1].more)}
          </button>
        {/if}
      </article>
    {/each}
  </section>
{/if}

<style>
  .content-results {
    padding: 0 1.5rem 2.2rem;
  }
  .head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 1rem;
    margin: 0.2rem 0 0.85rem;
  }
  h2,
  h3,
  p {
    margin: 0;
  }
  h2 {
    font-size: 0.82rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--faint);
  }
  .head span {
    color: var(--muted);
    font-size: 0.82rem;
  }
  .book {
    border-top: 1px solid var(--border);
    padding: 1rem 0;
  }
  .bookhead {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    width: 100%;
    padding: 0;
    border: none;
    background: none;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .cover {
    width: 40px;
    aspect-ratio: 2 / 3;
    border-radius: 5px;
    overflow: hidden;
    background: var(--surface);
    flex: none;
    display: grid;
    place-items: center;
    color: var(--muted);
    font-weight: 700;
  }
  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .meta {
    min-width: 0;
  }
  h3 {
    font-size: 0.95rem;
    letter-spacing: 0;
  }
  .meta p {
    margin-top: 0.15rem;
    color: var(--muted);
    font-size: 0.82rem;
  }
  .meta p.series {
    color: var(--accent);
  }
  .hits {
    list-style: none;
    margin: 0.8rem 0 0;
    padding: 0;
    display: grid;
    gap: 0.55rem;
  }
  .hits li {
    display: grid;
    grid-template-columns: 3.8rem minmax(0, 1fr);
    gap: 0.75rem;
    align-items: baseline;
  }
  .page {
    color: var(--faint);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
  }
  .hits p {
    color: var(--text);
    font-size: 0.9rem;
    line-height: 1.55;
  }
  :global(.content-results mark.hit) {
    padding: 0.05em 0.18em;
    border-radius: 0.2em;
    background: #fff0a6 !important;
    box-shadow: inset 0 -0.48em 0 rgba(255, 222, 93, 0.58);
    color: #211800;
  }
  .more {
    display: inline-flex;
    margin-top: 0.65rem;
    margin-left: 4.55rem;
    padding: 0;
    border: none;
    background: none;
    color: var(--accent);
    cursor: pointer;
    font: inherit;
    font-size: 0.82rem;
  }
  .more:hover {
    text-decoration: underline;
  }
  @media (max-width: 720px) {
    .content-results {
      padding-inline: 1rem;
    }
    .hits li {
      grid-template-columns: 1fr;
      gap: 0.2rem;
    }
    .more {
      margin-left: 0;
    }
  }
</style>
