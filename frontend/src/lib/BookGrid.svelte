<script lang="ts">
  import type { BookCard } from "./api";

  let {
    books,
    mode,
    onOpen,
  }: {
    books: BookCard[];
    mode: "grid" | "list";
    onOpen: (id: number) => void;
  } = $props();

  function initials(title: string): string {
    return title
      .split(/\s+/)
      .slice(0, 2)
      .map((w) => w[0] ?? "")
      .join("")
      .toUpperCase();
  }

  // Deterministic hue from the title so placeholder covers feel intentional.
  function hue(title: string): number {
    let h = 0;
    for (let i = 0; i < title.length; i++) h = (h * 31 + title.charCodeAt(i)) % 360;
    return h;
  }
</script>

{#if mode === "grid"}
  <div class="grid">
    {#each books as b (b.id)}
      <button class="cell" onclick={() => onOpen(b.id)} title={b.title}>
        <div class="cover">
          {#if b.cover}
            <img src={b.cover} alt="" loading="lazy" />
          {:else}
            <div class="ph" style="--h:{hue(b.title)}">{initials(b.title)}</div>
          {/if}
        </div>
        <div class="cap">
          <div class="t ellipsis">{b.title}</div>
          <div class="a ellipsis">{b.authors || "—"}</div>
        </div>
      </button>
    {/each}
  </div>
{:else}
  <div class="list">
    {#each books as b (b.id)}
      <button class="row" onclick={() => onOpen(b.id)}>
        <div class="thumb">
          {#if b.cover}
            <img src={b.cover} alt="" loading="lazy" />
          {:else}
            <div class="ph sm" style="--h:{hue(b.title)}">{initials(b.title)}</div>
          {/if}
        </div>
        <div class="meta">
          <div class="t ellipsis">{b.title}</div>
          <div class="a ellipsis">{b.authors || "—"}</div>
        </div>
        {#if b.series}
          <div class="series ellipsis">
            {b.series}{b.seriesIndex ? ` #${b.seriesIndex}` : ""}
          </div>
        {/if}
        <div class="formats">
          {#each b.formats as f}<span class="tag">{f}</span>{/each}
        </div>
      </button>
    {/each}
  </div>
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(148px, 1fr));
    gap: 1.35rem 1.1rem;
    padding: 1.5rem;
  }
  .cell {
    font: inherit;
    color: inherit;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }
  .cover {
    position: relative;
    aspect-ratio: 2 / 3;
    border-radius: 8px;
    overflow: hidden;
    background: var(--surface);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.35);
    transition: transform 0.16s ease, box-shadow 0.16s ease;
  }
  .cell:hover .cover {
    transform: translateY(-4px);
    box-shadow: 0 14px 30px rgba(0, 0, 0, 0.5);
  }
  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .ph {
    width: 100%;
    height: 100%;
    display: grid;
    place-items: center;
    font-weight: 700;
    font-size: 1.6rem;
    color: #fff;
    background: linear-gradient(
      145deg,
      hsl(var(--h) 45% 42%),
      hsl(calc(var(--h) + 40) 45% 26%)
    );
  }
  .ph.sm {
    font-size: 0.85rem;
  }
  .cap .t {
    font-size: 0.84rem;
    font-weight: 550;
  }
  .cap .a {
    font-size: 0.76rem;
    color: var(--muted);
    margin-top: 0.1rem;
  }

  .list {
    display: flex;
    flex-direction: column;
    padding: 0.5rem 1rem 1.5rem;
  }
  .row {
    font: inherit;
    color: inherit;
    background: none;
    border: none;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    text-align: left;
    display: grid;
    grid-template-columns: 40px 1fr auto auto;
    align-items: center;
    gap: 1rem;
    padding: 0.55rem 0.6rem;
  }
  .row:hover {
    background: var(--surface);
  }
  .thumb {
    width: 40px;
    height: 58px;
    border-radius: 4px;
    overflow: hidden;
    background: var(--surface);
  }
  .thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .meta {
    min-width: 0;
  }
  .meta .t {
    font-size: 0.9rem;
    font-weight: 550;
  }
  .meta .a {
    font-size: 0.78rem;
    color: var(--muted);
  }
  .series {
    color: var(--muted);
    font-size: 0.8rem;
    max-width: 200px;
  }
  .formats {
    display: flex;
    gap: 0.3rem;
  }
  .tag {
    font-size: 0.64rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--muted);
    background: var(--surface-hi);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 0.12rem 0.35rem;
  }

  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
