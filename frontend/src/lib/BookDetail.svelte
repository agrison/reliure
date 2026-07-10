<script lang="ts">
  import type { BookDetail } from "./api";

  let { book, onClose }: { book: BookDetail; onClose: () => void } = $props();

  function hue(title: string): number {
    let h = 0;
    for (let i = 0; i < title.length; i++) h = (h * 31 + title.charCodeAt(i)) % 360;
    return h;
  }
  function initials(title: string): string {
    return title.split(/\s+/).slice(0, 2).map((w) => w[0] ?? "").join("").toUpperCase();
  }
  function humanSize(n: number): string {
    if (!n) return "";
    const u = ["o", "Ko", "Mo", "Go"];
    let i = 0;
    let v = n;
    while (v >= 1024 && i < u.length - 1) {
      v /= 1024;
      i++;
    }
    return `${v.toFixed(i ? 1 : 0)} ${u[i]}`;
  }

  const roleLabels: Record<string, string> = {
    aut: "", trl: "trad.", edt: "éd.", ill: "ill.", ctb: "contrib.",
  };
</script>

<div class="scrim" onclick={onClose} role="presentation"></div>

<aside class="drawer">
  <button class="close" onclick={onClose} aria-label="Fermer">✕</button>

  <div class="hero">
    <div class="cover">
      {#if book.cover}
        <img src={book.cover} alt="" />
      {:else}
        <div class="ph" style="--h:{hue(book.title)}">{initials(book.title)}</div>
      {/if}
    </div>
  </div>

  <h2>{book.title}</h2>

  {#if book.authors?.length}
    <p class="authors">
      {#each book.authors as a, i}{i > 0 ? ", " : ""}{a.name}{roleLabels[a.role]
          ? ` (${roleLabels[a.role]})`
          : ""}{/each}
    </p>
  {/if}

  {#if book.series}
    <p class="series">
      {book.series}{book.seriesIndex ? ` — tome ${book.seriesIndex}` : ""}
    </p>
  {/if}

  {#if book.tags?.length}
    <div class="tags">
      {#each book.tags as t}<span class="chip">{t}</span>{/each}
    </div>
  {/if}

  {#if book.description}
    <p class="desc">{book.description}</p>
  {/if}

  <dl class="facts">
    {#if book.language}<div><dt>Langue</dt><dd>{book.language}</dd></div>{/if}
    {#if book.published}<div><dt>Publié</dt><dd>{book.published}</dd></div>{/if}
    {#if book.isbn}<div><dt>ISBN</dt><dd>{book.isbn}</dd></div>{/if}
  </dl>

  <div class="files">
    <h3>Fichiers</h3>
    {#each book.files as f}
      <div class="file">
        <span class="fmt">{f.format}</span>
        <span class="path ellipsis" title={f.path}>{f.path}</span>
        {#if f.size}<span class="sz">{humanSize(f.size)}</span>{/if}
      </div>
    {/each}
  </div>
</aside>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    animation: fade 0.15s ease;
  }
  .drawer {
    position: fixed;
    top: 0;
    right: 0;
    height: 100%;
    width: min(420px, 92vw);
    padding: 2rem 1.75rem;
    overflow-y: auto;
    background: var(--panel);
    border-left: 1px solid var(--border);
    box-shadow: -20px 0 50px rgba(0, 0, 0, 0.45);
    animation: slide 0.2s cubic-bezier(0.2, 0.7, 0.2, 1);
  }
  @keyframes fade {
    from { opacity: 0; }
  }
  @keyframes slide {
    from { transform: translateX(20px); opacity: 0.5; }
  }
  .close {
    position: absolute;
    top: 1rem;
    right: 1rem;
    width: 2rem;
    height: 2rem;
    border-radius: 50%;
    border: none;
    background: var(--surface-hi);
    color: var(--muted);
    cursor: pointer;
    font-size: 0.8rem;
  }
  .close:hover {
    color: var(--text);
  }

  .hero {
    display: flex;
    justify-content: center;
    margin: 0.5rem 0 1.5rem;
  }
  .cover {
    width: 190px;
    aspect-ratio: 2 / 3;
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
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
    font-size: 3rem;
    color: #fff;
    background: linear-gradient(145deg, hsl(var(--h) 45% 42%), hsl(calc(var(--h) + 40) 45% 26%));
  }

  h2 {
    margin: 0;
    font-size: 1.4rem;
    line-height: 1.25;
    letter-spacing: -0.01em;
  }
  .authors {
    margin: 0.4rem 0 0;
    color: var(--text);
  }
  .series {
    margin: 0.35rem 0 0;
    color: var(--accent);
    font-size: 0.9rem;
  }

  .tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin-top: 1rem;
  }
  .chip {
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--surface-hi);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.2rem 0.6rem;
  }

  .desc {
    margin: 1.25rem 0 0;
    color: var(--muted);
    font-size: 0.88rem;
    line-height: 1.55;
    user-select: text;
  }

  .facts {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(90px, 1fr));
    gap: 0.9rem;
    margin: 1.5rem 0 0;
    padding: 1rem 0;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
  }
  .facts dt {
    color: var(--faint);
    font-size: 0.66rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .facts dd {
    margin: 0.2rem 0 0;
    font-size: 0.86rem;
    user-select: text;
  }

  .files {
    margin-top: 1.5rem;
  }
  .files h3 {
    margin: 0 0 0.6rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--faint);
  }
  .file {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 0.6rem;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    margin-bottom: 0.4rem;
    font-size: 0.78rem;
  }
  .fmt {
    text-transform: uppercase;
    font-size: 0.64rem;
    letter-spacing: 0.04em;
    color: var(--accent);
    flex: none;
  }
  .path {
    color: var(--muted);
    flex: 1;
    min-width: 0;
    user-select: text;
  }
  .sz {
    color: var(--faint);
    flex: none;
    font-variant-numeric: tabular-nums;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
