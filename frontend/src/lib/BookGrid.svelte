<script lang="ts">
  import type { BookCard, DeviceBookState, ReadingCard } from "./api";
  import { t } from "./i18n";

  let {
    books,
    mode,
    selectedIds,
    deviceStates,
    readingStates,
    onOpen,
    onToggleSelect,
  }: {
    books: BookCard[];
    mode: "grid" | "list";
    selectedIds: number[];
    deviceStates: Record<number, DeviceBookState>;
    readingStates: Record<number, ReadingCard>;
    onOpen: (id: number) => void;
    onToggleSelect: (id: number) => void;
  } = $props();

  // Progress to show on a card: a completed book reads as full, whatever the
  // last recorded percentage.
  function progress(state: ReadingCard | undefined): number {
    if (!state) return 0;
    if (state.status === "complete") return 1;
    return state.percent ?? 0;
  }

  function progressTitle(state: ReadingCard | undefined): string {
    if (!state) return "";
    const pct = Math.round(progress(state) * 100);
    if (state.pages > 0) {
      const page = Math.max(1, Math.round(progress(state) * state.pages));
      return t("book.progressPage", undefined, { pct, page, pages: state.pages });
    }
    return t("book.progressRead", undefined, { pct });
  }

  const selected = $derived(new Set(selectedIds));

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

  function deviceLabel(state: DeviceBookState | undefined): string {
    if (!state || state.status === "unknown") return "";
    return state.status === "present" ? t("book.onReader") : t("book.absent");
  }
</script>

{#if mode === "grid"}
  <div class="grid">
    {#each books as b (b.id)}
      <div class="cell" class:selected={selected.has(b.id)}>
        <button
          class="select"
          class:active={selected.has(b.id)}
          onclick={() => onToggleSelect(b.id)}
          aria-label={selected.has(b.id) ? t("book.deselect") : t("book.select")}
        >
          {selected.has(b.id) ? "✓" : ""}
        </button>
        <button class="open" onclick={() => onOpen(b.id)} title={b.title}>
        <div class="cover">
          {#if b.cover}
            <img src={b.cover} alt="" loading="lazy" />
          {:else}
            <div class="ph" style="--h:{hue(b.title)}">{initials(b.title)}</div>
          {/if}
          {#if deviceLabel(deviceStates[b.id])}
            <span
              class="device"
              class:present={deviceStates[b.id]?.status === "present"}
              title={deviceStates[b.id]?.remotePath || deviceLabel(deviceStates[b.id])}
            >
              {deviceLabel(deviceStates[b.id])}
            </span>
          {/if}
          {#if readingStates[b.id]?.annotations}
            <span class="notes" title={t("book.noteTitle", undefined, { count: readingStates[b.id].annotations })}>✎ {readingStates[b.id].annotations}</span>
          {/if}
          {#if progress(readingStates[b.id]) > 0}
            <span class="progress" title={progressTitle(readingStates[b.id])}>
              <span
                class="pfill"
                class:done={readingStates[b.id]?.status === "complete"}
                style="width:{Math.round(progress(readingStates[b.id]) * 100)}%"
              ></span>
            </span>
          {/if}
        </div>
        <div class="cap">
          <div class="t ellipsis">{b.title}</div>
          <div class="a ellipsis">{b.authors || t("common.none")}</div>
        </div>
        </button>
      </div>
    {/each}
  </div>
{:else}
  <div class="list">
    {#each books as b (b.id)}
      <div class="row" class:selected={selected.has(b.id)}>
        <button
          class="select"
          class:active={selected.has(b.id)}
          onclick={() => onToggleSelect(b.id)}
          aria-label={selected.has(b.id) ? t("book.deselect") : t("book.select")}
        >
          {selected.has(b.id) ? "✓" : ""}
        </button>
        <button class="rowopen" onclick={() => onOpen(b.id)}>
        <div class="thumb">
          {#if b.cover}
            <img src={b.cover} alt="" loading="lazy" />
          {:else}
            <div class="ph sm" style="--h:{hue(b.title)}">{initials(b.title)}</div>
          {/if}
        </div>
        <div class="meta">
          <div class="t ellipsis">{b.title}</div>
          <div class="a ellipsis">{b.authors || t("common.none")}</div>
        </div>
        <!-- Fixed slots so badges stay column-aligned across rows regardless of
             which ones a book has (series / read / on-reader). -->
        <div class="col series ellipsis">
          {#if b.series}{b.series}{b.seriesIndex ? ` #${b.seriesIndex}` : ""}{/if}
        </div>
        <div class="col right">
          {#if readingStates[b.id]?.status === "complete"}
            <span class="chip read">{t("book.read")}</span>
          {:else if progress(readingStates[b.id]) > 0}
            <span class="chip pct" title={progressTitle(readingStates[b.id])}>{Math.round(progress(readingStates[b.id]) * 100)} %</span>
          {/if}
        </div>
        <div class="col right">
          {#if deviceStates[b.id]?.status === "present"}
            <span class="chip reader" title={deviceStates[b.id]?.remotePath || t("book.onReaderChip")}>{t("book.onReaderChip")}</span>
          {/if}
        </div>
        <div class="col formats">
          {#each b.formats as f}<span class="tag">{f}</span>{/each}
        </div>
        </button>
      </div>
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
    position: relative;
    min-width: 0;
    padding: 3px;
    border-radius: 10px;
  }
  .cell.selected {
    background: color-mix(in srgb, var(--accent) 16%, transparent);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 45%, transparent);
  }
  .open {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
    width: 100%;
    min-width: 0;
    overflow: hidden;
    padding: 0;
    border: none;
    background: none;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
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
  .open:hover .cover {
    transform: translateY(-4px);
    box-shadow: 0 14px 30px rgba(0, 0, 0, 0.5);
  }
  .select {
    position: absolute;
    z-index: 2;
    top: 0.45rem;
    left: 0.45rem;
    width: 24px;
    height: 24px;
    display: grid;
    place-items: center;
    border: 1px solid var(--border-hi);
    border-radius: 999px;
    background: rgba(0, 0, 0, 0.55);
    color: var(--text);
    font: inherit;
    font-size: 0.74rem;
    cursor: pointer;
  }
  .select.active {
    background: var(--accent);
    color: var(--accent-ink);
    border-color: var(--accent);
  }
  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .device {
    position: absolute;
    right: 0.45rem;
    bottom: 0.45rem;
    max-width: calc(100% - 0.9rem);
    padding: 0.22rem 0.42rem;
    border-radius: 999px;
    background: rgba(0, 0, 0, 0.62);
    color: rgba(255, 255, 255, 0.78);
    border: 1px solid rgba(255, 255, 255, 0.16);
    font-size: 0.66rem;
    font-weight: 650;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    pointer-events: none;
  }
  .device.present {
    background: color-mix(in srgb, var(--accent) 82%, #0b1410);
    color: var(--accent-ink);
    border-color: color-mix(in srgb, var(--accent) 72%, transparent);
  }
  .notes {
    position: absolute;
    top: 0.45rem;
    right: 0.45rem;
    padding: 0.14rem 0.4rem;
    border-radius: 999px;
    background: rgba(0, 0, 0, 0.62);
    color: rgba(255, 255, 255, 0.85);
    border: 1px solid rgba(255, 255, 255, 0.16);
    font-size: 0.64rem;
    font-weight: 650;
    pointer-events: none;
  }
  .progress {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    height: 5px;
    background: rgba(0, 0, 0, 0.45);
  }
  .pfill {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  .pfill.done {
    background: var(--ok);
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
  .cap {
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
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
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr);
    align-items: center;
    gap: 0.5rem;
    border-bottom: 1px solid var(--border);
  }
  .row.selected {
    background: color-mix(in srgb, var(--accent) 12%, transparent);
  }
  .row .select {
    position: static;
    margin-left: 0.2rem;
  }
  .rowopen {
    font: inherit;
    color: inherit;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    display: grid;
    /* thumb | title+author | series | read | on-reader | formats.
       Title and series share the slack (title keeps priority) so the series +
       volume gets more room on a wide window; the badge columns stay fixed so
       they line up across rows whether or not a given book fills them. */
    grid-template-columns: 40px minmax(0, 1.7fr) minmax(0, 1fr) 58px 118px 96px;
    align-items: center;
    gap: 0.7rem;
    padding: 0.55rem 0.6rem;
    min-width: 0;
  }
  .col {
    min-width: 0;
  }
  .col.right {
    display: flex;
    justify-content: flex-end;
  }
  .rowopen:hover {
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
  }
  .formats {
    display: flex;
    gap: 0.3rem;
    justify-content: flex-end;
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
  /* Coloured status chips, EPUB-tag style: green "Lu", blue "Sur la liseuse". */
  .chip {
    font-size: 0.62rem;
    font-weight: 650;
    letter-spacing: 0.02em;
    border-radius: 5px;
    padding: 0.12rem 0.4rem;
    border: 1px solid var(--border);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100%;
  }
  .chip.read {
    color: var(--ok);
    border-color: color-mix(in srgb, var(--ok) 42%, var(--border));
    background: color-mix(in srgb, var(--ok) 13%, transparent);
  }
  .chip.reader {
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 42%, var(--border));
    background: color-mix(in srgb, var(--accent) 13%, transparent);
  }
  .chip.pct {
    color: var(--muted);
    background: var(--surface-hi);
    font-variant-numeric: tabular-nums;
  }

  .ellipsis {
    display: block;
    max-width: 100%;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    overflow-wrap: anywhere;
  }
</style>
