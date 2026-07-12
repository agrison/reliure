<script lang="ts">
  import { StatsService } from "./api";
  import { t } from "./i18n";
  import type { Dashboard, NameCount } from "./api";
  import type { ReadingStatus } from "./types";

  let {
    onOpenBook,
    onSelectStatus,
  }: {
    onOpenBook: (id: number) => void;
    onSelectStatus: (status: ReadingStatus) => void;
  } = $props();

  let loading = $state(true);
  let d = $state<Dashboard | null>(null);

  async function load() {
    loading = true;
    try {
      d = await StatsService.Dashboard();
    } catch (e) {
      console.error("[Reliure] Dashboard failed", e);
      d = null;
    } finally {
      loading = false;
    }
  }

  function humanSize(n: number): string {
    if (!n) return "0";
    const u = ["o", "Ko", "Mo", "Go", "To"];
    let i = 0;
    let v = n;
    while (v >= 1024 && i < u.length - 1) {
      v /= 1024;
      i++;
    }
    return `${v.toFixed(i ? 1 : 0)} ${u[i]}`;
  }
  function fr(n: number): string {
    return n.toLocaleString("fr-FR");
  }
  function maxCount(list: NameCount[] | null): number {
    return (list ?? []).reduce((m, x) => Math.max(m, x.count), 0) || 1;
  }
  function pct(part: number, total: number): number {
    return total > 0 ? (part / total) * 100 : 0;
  }

  function langLabel(code: string): string {
    const key = `language.${code}` as Parameters<typeof t>[0];
    return code === "Autres" ? t("language.other") : t(key) || code.toUpperCase();
  }
  function monthLabel(m: string): string {
    const [y, mo] = m.split("-");
    const month = Number(mo);
    return `${month >= 1 && month <= 12 ? t(`month.${month}` as Parameters<typeof t>[0]) : mo} ${y.slice(2)}`;
  }

  load();
</script>

<div class="dash">
  {#if loading && !d}
    <p class="msg">{t("common.loading")}</p>
  {:else if !d}
    <p class="msg">{t("dashboard.unavailable")}</p>
  {:else}
    <div class="tiles">
      <div class="tile"><span class="tnum">{fr(d.books)}</span><span class="tlbl">{t("dashboard.books")}</span></div>
      <div class="tile"><span class="tnum">{humanSize(d.totalSize)}</span><span class="tlbl">{fr(d.files)} {t(d.files === 1 ? "common.file" : "common.files")}</span></div>
      <div class="tile"><span class="tnum">{fr(d.authors)}</span><span class="tlbl">{t("dashboard.authors")}</span></div>
      <div class="tile"><span class="tnum">{fr(d.series)}</span><span class="tlbl">{t("dashboard.series")}</span></div>
      <div class="tile"><span class="tnum">{fr(d.tags)}</span><span class="tlbl">{t("dashboard.tags")}</span></div>
      {#if d.onDevice > 0}
        <div class="tile"><span class="tnum">{fr(d.onDevice)}</span><span class="tlbl">{t("dashboard.onReader")}</span></div>
      {/if}
      {#if d.annotations > 0}
        <div class="tile"><span class="tnum">{fr(d.annotations)}</span><span class="tlbl">{t("dashboard.annotations")}</span></div>
      {/if}
      {#if d.content?.enabled}
        <div class="tile"><span class="tnum">{fr(d.content.indexedBooks)}</span><span class="tlbl">{t("dashboard.indexedBooks")}</span></div>
      {/if}
    </div>

    <div class="cards">
      {#if d.content?.enabled}
        <section class="card wide">
          <h3>{t("dashboard.content.title")}</h3>
          <div class="contentstats">
            <span><b>{fr(d.content.indexedBooks)}</b> {t("dashboard.content.indexed")}</span>
            <span><b>{fr(d.content.pendingBooks)}</b> {t("dashboard.content.pending")}</span>
            <span><b>{fr(d.content.emptyBooks)}</b> {t("dashboard.content.empty")}</span>
            <span><b>{fr(d.content.failedBooks)}</b> {t("dashboard.content.failed")}</span>
            <span><b>{fr(d.content.indexedChars)}</b> {t("dashboard.content.chars")}</span>
          </div>
          {#if d.books > 0}
            <div class="indexbar" role="img" aria-label={t("dashboard.content.coverage")}>
              <div class="indexed" style="width:{pct(d.content.indexedBooks, d.books)}%"></div>
            </div>
          {/if}
        </section>
      {/if}

      <!-- Reading breakdown: a labelled status bar (identity never color-alone). -->
      <section class="card wide">
        <h3>{t("dashboard.reading.title")}</h3>
        {#if d.books > 0}
          {@const r = d.reading}
          <div class="segbar" role="img" aria-label={t("dashboard.reading.distribution")}>
            {#if r.complete}<div class="seg complete" style="flex:{r.complete}" title="{t('nav.complete')} : {r.complete}"></div>{/if}
            {#if r.reading}<div class="seg reading" style="flex:{r.reading}" title="{t('nav.reading')} : {r.reading}"></div>{/if}
            {#if r.abandoned}<div class="seg abandoned" style="flex:{r.abandoned}" title="{t('nav.abandoned')} : {r.abandoned}"></div>{/if}
            {#if r.unread}<div class="seg unread" style="flex:{r.unread}" title="{t('dashboard.reading.unread')} : {r.unread}"></div>{/if}
          </div>
          <div class="legend">
            <button class="lg" onclick={() => onSelectStatus("complete")} disabled={!r.complete}><span class="dot complete"></span>{t("nav.complete")}<b>{fr(r.complete)}</b></button>
            <button class="lg" onclick={() => onSelectStatus("reading")} disabled={!r.reading}><span class="dot reading"></span>{t("nav.reading")}<b>{fr(r.reading)}</b></button>
            <button class="lg" onclick={() => onSelectStatus("abandoned")} disabled={!r.abandoned}><span class="dot abandoned"></span>{t("nav.abandoned")}<b>{fr(r.abandoned)}</b></button>
            <span class="lg static"><span class="dot unread"></span>{t("dashboard.reading.unread")}<b>{fr(r.unread)}</b></span>
          </div>
        {:else}
          <p class="empty">{t("dashboard.noBook")}</p>
        {/if}
      </section>

      <section class="card">
        <h3>{t("dashboard.formats")}</h3>
        {#each d.formats ?? [] as f (f.name)}
          {@const m = maxCount(d.formats)}
          <div class="bar">
            <span class="blabel">{f.name.toUpperCase()}</span>
            <div class="btrack"><div class="bfill" style="width:{pct(f.count, m)}%"></div></div>
            <span class="bval">{fr(f.count)}</span>
          </div>
        {/each}
        {#if !(d.formats ?? []).length}<p class="empty">{t("common.none")}</p>{/if}
      </section>

      <section class="card">
        <h3>{t("dashboard.languages")}</h3>
        {#each d.languages ?? [] as l (l.name)}
          {@const m = maxCount(d.languages)}
          <div class="bar">
            <span class="blabel ellipsis">{langLabel(l.name)}</span>
            <div class="btrack"><div class="bfill" style="width:{pct(l.count, m)}%"></div></div>
            <span class="bval">{fr(l.count)}</span>
          </div>
        {/each}
        {#if !(d.languages ?? []).length}<p class="empty">{t("common.none")}</p>{/if}
      </section>

      <section class="card">
        <h3>{t("dashboard.topAuthors")}</h3>
        {#each d.topAuthors ?? [] as a (a.name)}
          {@const m = maxCount(d.topAuthors)}
          <div class="bar">
            <span class="blabel ellipsis" title={a.name}>{a.name}</span>
            <div class="btrack"><div class="bfill" style="width:{pct(a.count, m)}%"></div></div>
            <span class="bval">{fr(a.count)}</span>
          </div>
        {/each}
        {#if !(d.topAuthors ?? []).length}<p class="empty">{t("common.none")}</p>{/if}
      </section>

      {#if (d.topTags ?? []).length}
        <section class="card">
          <h3>{t("dashboard.topTags")}</h3>
          {#each d.topTags as t (t.name)}
            {@const m = maxCount(d.topTags)}
            <div class="bar">
              <span class="blabel ellipsis" title={t.name}>{t.name}</span>
              <div class="btrack"><div class="bfill" style="width:{pct(t.count, m)}%"></div></div>
              <span class="bval">{fr(t.count)}</span>
            </div>
          {/each}
        </section>
      {/if}

      {#if (d.addedByMonth ?? []).length > 1}
        {@const mm = maxCount(d.addedByMonth)}
        <section class="card wide">
          <h3>{t("dashboard.addedByMonth")}</h3>
          <div class="cols">
            {#each d.addedByMonth as m (m.name)}
              <div class="colgroup" title="{monthLabel(m.name)} : {m.count}">
                <div class="coltrack"><div class="colfill" style="height:{pct(m.count, mm)}%"></div></div>
                <span class="collabel">{monthLabel(m.name)}</span>
              </div>
            {/each}
          </div>
        </section>
      {/if}
    </div>

    {#if (d.recent ?? []).length}
      <section class="recent">
        <h3>{t("dashboard.recent")}</h3>
        <div class="rrow">
          {#each d.recent as b (b.id)}
            <button class="rbook" onclick={() => onOpenBook(b.id)} title={b.title}>
              <div class="rcover">
                {#if b.cover}<img src={b.cover} alt="" loading="lazy" />{:else}<span class="rph">{b.title.slice(0, 1)}</span>{/if}
              </div>
              <span class="rtitle ellipsis">{b.title}</span>
            </button>
          {/each}
        </div>
      </section>
    {/if}
  {/if}
</div>

<style>
  .dash {
    padding: 1.25rem 1.5rem 3rem;
    /* Local status hues that hold up in light and dark. */
    --amber: light-dark(#c9821f, #e0a84a);
    --neutral: light-dark(rgba(0, 0, 0, 0.16), rgba(255, 255, 255, 0.16));
  }
  .msg {
    color: var(--muted);
    padding: 1.5rem 0;
  }

  .tiles {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 0.9rem;
    margin-bottom: 1.2rem;
  }
  .tile {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    padding: 1rem 1.1rem;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--panel);
  }
  .tnum {
    font-size: 1.6rem;
    font-weight: 700;
    letter-spacing: -0.01em;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }
  .tlbl {
    font-size: 0.78rem;
    color: var(--muted);
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 0.9rem;
  }
  .card {
    padding: 1rem 1.1rem 1.1rem;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--panel);
    min-width: 0;
  }
  .card.wide {
    grid-column: 1 / -1;
  }
  .card h3,
  .recent h3 {
    margin: 0 0 0.85rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--faint);
  }
  .empty {
    color: var(--faint);
    font-size: 0.85rem;
    margin: 0;
  }
  .contentstats {
    display: flex;
    flex-wrap: wrap;
    gap: 0.65rem 1rem;
    color: var(--muted);
    font-size: 0.86rem;
  }
  .contentstats b {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }
  .indexbar {
    height: 8px;
    margin-top: 0.85rem;
    border-radius: 999px;
    background: var(--inset);
    overflow: hidden;
  }
  .indexbar .indexed {
    height: 100%;
    background: var(--accent);
  }

  /* Horizontal magnitude bars — single hue (identity is the label). */
  .bar {
    display: grid;
    grid-template-columns: minmax(56px, 34%) 1fr auto;
    align-items: center;
    gap: 0.7rem;
    padding: 0.28rem 0;
  }
  .blabel {
    font-size: 0.82rem;
    color: var(--text);
    min-width: 0;
  }
  .btrack {
    height: 9px;
    background: var(--inset);
    border-radius: 999px;
    overflow: hidden;
  }
  .bfill {
    height: 100%;
    background: var(--accent);
    border-radius: 999px;
    min-width: 3px;
  }
  .bval {
    font-size: 0.8rem;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }

  /* Reading status bar. */
  .segbar {
    display: flex;
    gap: 2px; /* surface gap between fills */
    height: 14px;
    border-radius: 999px;
    overflow: hidden;
    background: var(--inset);
  }
  .seg {
    min-width: 3px;
  }
  .seg.complete { background: var(--ok); }
  .seg.reading { background: var(--accent); }
  .seg.abandoned { background: var(--amber); }
  .seg.unread { background: var(--neutral); }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1.1rem;
    margin-top: 0.85rem;
  }
  .lg {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    border: none;
    background: none;
    color: var(--muted);
    font: inherit;
    font-size: 0.82rem;
    cursor: pointer;
    padding: 0;
  }
  .lg:hover:not(:disabled):not(.static) {
    color: var(--text);
  }
  .lg:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .lg.static {
    cursor: default;
  }
  .lg b {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }
  .dot {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    flex: none;
  }
  .dot.complete { background: var(--ok); }
  .dot.reading { background: var(--accent); }
  .dot.abandoned { background: var(--amber); }
  .dot.unread { background: var(--neutral); }

  /* Monthly columns. */
  .cols {
    display: flex;
    align-items: flex-end;
    gap: 0.4rem;
    height: 120px;
    overflow-x: auto;
  }
  .colgroup {
    flex: 1;
    min-width: 26px;
    display: flex;
    flex-direction: column;
    align-items: center;
    height: 100%;
  }
  .coltrack {
    flex: 1;
    width: 100%;
    max-width: 30px;
    display: flex;
    align-items: flex-end;
    justify-content: center;
  }
  .colfill {
    width: 100%;
    background: var(--accent);
    border-radius: 4px 4px 0 0;
    min-height: 2px;
  }
  .collabel {
    margin-top: 0.4rem;
    font-size: 0.66rem;
    color: var(--faint);
    white-space: nowrap;
  }

  .recent {
    margin-top: 1.5rem;
  }
  .rrow {
    display: flex;
    gap: 0.9rem;
    overflow-x: auto;
    padding-bottom: 0.4rem;
  }
  .rbook {
    flex: 0 0 84px;
    width: 84px;
    min-width: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    border: none;
    background: none;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
    padding: 0;
  }
  .rcover {
    width: 84px;
    max-width: 100%;
    flex: none;
    aspect-ratio: 2 / 3;
    border-radius: 7px;
    overflow: hidden;
    background: var(--surface);
    box-shadow: 0 5px 14px rgba(0, 0, 0, 0.3);
    display: grid;
    place-items: center;
  }
  .rcover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .rph {
    font-size: 1.4rem;
    font-weight: 700;
    color: var(--muted);
  }
  .rbook:hover .rcover {
    box-shadow: 0 10px 22px rgba(0, 0, 0, 0.45);
  }
  .rtitle {
    display: block;
    width: 100%;
    min-width: 0;
    font-size: 0.74rem;
    color: var(--muted);
    line-height: 1.25;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
