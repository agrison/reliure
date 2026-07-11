<script lang="ts">
  import { KOReaderService } from "./api";
  import type { AnnotatedBook } from "./api";

  let { onOpen }: { onOpen: (id: number) => void } = $props();

  let loading = $state(true);
  let books = $state<AnnotatedBook[]>([]);

  async function load() {
    loading = true;
    try {
      books = (await KOReaderService.AnnotatedBooks()) ?? [];
    } catch (e) {
      console.error("[Reliure] AnnotatedBooks failed", e);
      books = [];
    } finally {
      loading = false;
    }
  }

  function initials(title: string): string {
    return title.split(/\s+/).slice(0, 2).map((w) => w[0] ?? "").join("").toUpperCase();
  }
  function hue(title: string): number {
    let h = 0;
    for (let i = 0; i < title.length; i++) h = (h * 31 + title.charCodeAt(i)) % 360;
    return h;
  }

  load();
</script>

<div class="annotations">
  {#if loading}
    <p class="msg">Chargement…</p>
  {:else if books.length === 0}
    <p class="msg">Aucune annotation synchronisée pour l’instant.</p>
  {:else}
    {#each books as b (b.bookId)}
      <section class="book">
        <button class="bookhead" onclick={() => onOpen(b.bookId)} title="Ouvrir la fiche">
          <div class="cover">
            {#if b.cover}
              <img src={b.cover} alt="" loading="lazy" />
            {:else}
              <div class="ph" style="--h:{hue(b.title)}">{initials(b.title)}</div>
            {/if}
          </div>
          <div class="binfo">
            <div class="btitle ellipsis">{b.title}</div>
            {#if b.authors}<div class="bauth ellipsis">{b.authors}</div>{/if}
            <div class="bcount">{(b.annotations ?? []).length} surlignage{(b.annotations ?? []).length === 1 ? "" : "s"} / note{(b.annotations ?? []).length === 1 ? "" : "s"}</div>
          </div>
        </button>

        <div class="list">
          {#each b.annotations ?? [] as a}
            <div class="anno">
              {#if a.chapter}<div class="achap">{a.chapter}</div>{/if}
              {#if a.text}<blockquote class="atext">{a.text}</blockquote>{/if}
              {#if a.note}<div class="anote">✎ {a.note}</div>{/if}
              {#if a.createdAt}<div class="adate">{a.createdAt}</div>{/if}
            </div>
          {/each}
        </div>
      </section>
    {/each}
  {/if}
</div>

<style>
  .annotations {
    padding: 1.25rem 1.5rem 3rem;
    max-width: 820px;
  }
  .msg {
    color: var(--muted);
    padding: 1.5rem 0;
  }
  .book {
    margin-bottom: 1.5rem;
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
    background: var(--panel);
  }
  .bookhead {
    display: flex;
    gap: 0.85rem;
    width: 100%;
    text-align: left;
    padding: 0.85rem 1rem;
    border: none;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
    color: inherit;
    font: inherit;
    cursor: pointer;
  }
  .bookhead:hover {
    background: var(--surface-hi);
  }
  .cover {
    flex: 0 0 42px;
    width: 42px;
    height: 62px;
    border-radius: 5px;
    overflow: hidden;
    background: var(--inset);
  }
  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .ph {
    width: 100%;
    height: 100%;
    display: grid;
    place-items: center;
    font-weight: 700;
    font-size: 0.85rem;
    color: #fff;
    background: linear-gradient(145deg, hsl(var(--h) 45% 42%), hsl(calc(var(--h) + 40) 45% 26%));
  }
  .binfo {
    min-width: 0;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 0.15rem;
  }
  .btitle {
    font-size: 0.95rem;
    font-weight: 600;
  }
  .bauth {
    font-size: 0.8rem;
    color: var(--muted);
  }
  .bcount {
    font-size: 0.72rem;
    color: var(--faint);
    margin-top: 0.1rem;
  }
  .list {
    padding: 0.85rem 1rem;
    display: grid;
    gap: 0.7rem;
  }
  .anno {
    padding-left: 0.2rem;
  }
  .achap {
    font-size: 0.72rem;
    color: var(--accent);
    margin-bottom: 0.3rem;
  }
  .atext {
    margin: 0;
    padding-left: 0.7rem;
    border-left: 2px solid var(--border-hi);
    font-size: 0.86rem;
    line-height: 1.5;
    color: var(--text);
    user-select: text;
  }
  .anote {
    margin-top: 0.4rem;
    font-size: 0.82rem;
    color: var(--muted);
    user-select: text;
  }
  .adate {
    margin-top: 0.35rem;
    font-size: 0.68rem;
    color: var(--faint);
    font-variant-numeric: tabular-nums;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
