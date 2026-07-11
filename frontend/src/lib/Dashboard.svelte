<script lang="ts">
  import { StatsService } from "./api";
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

  const langNames: Record<string, string> = {
    fr: "Français", en: "Anglais", de: "Allemand", es: "Espagnol", it: "Italien",
    nl: "Néerlandais", pt: "Portugais", ru: "Russe", ja: "Japonais", la: "Latin", zh: "Chinois",
  };
  function langLabel(code: string): string {
    return langNames[code] ?? (code === "Autres" ? "Autres" : code.toUpperCase());
  }
  function monthLabel(m: string): string {
    const [y, mo] = m.split("-");
    const names = ["", "jan", "fév", "mar", "avr", "mai", "juin", "juil", "aoû", "sep", "oct", "nov", "déc"];
    return `${names[Number(mo)] ?? mo} ${y.slice(2)}`;
  }

  load();
</script>

<div class="dash">
  {#if loading && !d}
    <p class="msg">Chargement…</p>
  {:else if !d}
    <p class="msg">Statistiques indisponibles.</p>
  {:else}
    <div class="tiles">
      <div class="tile"><span class="tnum">{fr(d.books)}</span><span class="tlbl">Livres</span></div>
      <div class="tile"><span class="tnum">{humanSize(d.totalSize)}</span><span class="tlbl">{fr(d.files)} fichier{d.files === 1 ? "" : "s"}</span></div>
      <div class="tile"><span class="tnum">{fr(d.authors)}</span><span class="tlbl">Auteurs</span></div>
      <div class="tile"><span class="tnum">{fr(d.series)}</span><span class="tlbl">Séries</span></div>
      <div class="tile"><span class="tnum">{fr(d.tags)}</span><span class="tlbl">Tags</span></div>
      {#if d.onDevice > 0}
        <div class="tile"><span class="tnum">{fr(d.onDevice)}</span><span class="tlbl">Sur liseuse</span></div>
      {/if}
      {#if d.annotations > 0}
        <div class="tile"><span class="tnum">{fr(d.annotations)}</span><span class="tlbl">Surlignages</span></div>
      {/if}
    </div>

    <div class="cards">
      <!-- Reading breakdown: a labelled status bar (identity never color-alone). -->
      <section class="card wide">
        <h3>Lecture</h3>
        {#if d.books > 0}
          {@const r = d.reading}
          <div class="segbar" role="img" aria-label="Répartition par statut de lecture">
            {#if r.complete}<div class="seg complete" style="flex:{r.complete}" title="Terminés : {r.complete}"></div>{/if}
            {#if r.reading}<div class="seg reading" style="flex:{r.reading}" title="En cours : {r.reading}"></div>{/if}
            {#if r.abandoned}<div class="seg abandoned" style="flex:{r.abandoned}" title="Abandonnés : {r.abandoned}"></div>{/if}
            {#if r.unread}<div class="seg unread" style="flex:{r.unread}" title="Non lus : {r.unread}"></div>{/if}
          </div>
          <div class="legend">
            <button class="lg" onclick={() => onSelectStatus("complete")} disabled={!r.complete}><span class="dot complete"></span>Terminés<b>{fr(r.complete)}</b></button>
            <button class="lg" onclick={() => onSelectStatus("reading")} disabled={!r.reading}><span class="dot reading"></span>En cours<b>{fr(r.reading)}</b></button>
            <button class="lg" onclick={() => onSelectStatus("abandoned")} disabled={!r.abandoned}><span class="dot abandoned"></span>Abandonnés<b>{fr(r.abandoned)}</b></button>
            <span class="lg static"><span class="dot unread"></span>Non lus<b>{fr(r.unread)}</b></span>
          </div>
        {:else}
          <p class="empty">Aucun livre.</p>
        {/if}
      </section>

      <section class="card">
        <h3>Formats</h3>
        {#each d.formats ?? [] as f (f.name)}
          {@const m = maxCount(d.formats)}
          <div class="bar">
            <span class="blabel">{f.name.toUpperCase()}</span>
            <div class="btrack"><div class="bfill" style="width:{pct(f.count, m)}%"></div></div>
            <span class="bval">{fr(f.count)}</span>
          </div>
        {/each}
        {#if !(d.formats ?? []).length}<p class="empty">—</p>{/if}
      </section>

      <section class="card">
        <h3>Langues</h3>
        {#each d.languages ?? [] as l (l.name)}
          {@const m = maxCount(d.languages)}
          <div class="bar">
            <span class="blabel ellipsis">{langLabel(l.name)}</span>
            <div class="btrack"><div class="bfill" style="width:{pct(l.count, m)}%"></div></div>
            <span class="bval">{fr(l.count)}</span>
          </div>
        {/each}
        {#if !(d.languages ?? []).length}<p class="empty">—</p>{/if}
      </section>

      <section class="card">
        <h3>Auteurs les plus présents</h3>
        {#each d.topAuthors ?? [] as a (a.name)}
          {@const m = maxCount(d.topAuthors)}
          <div class="bar">
            <span class="blabel ellipsis" title={a.name}>{a.name}</span>
            <div class="btrack"><div class="bfill" style="width:{pct(a.count, m)}%"></div></div>
            <span class="bval">{fr(a.count)}</span>
          </div>
        {/each}
        {#if !(d.topAuthors ?? []).length}<p class="empty">—</p>{/if}
      </section>

      {#if (d.topTags ?? []).length}
        <section class="card">
          <h3>Tags les plus utilisés</h3>
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
          <h3>Ajouts par mois</h3>
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
        <h3>Ajouts récents</h3>
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
    font-size: 0.74rem;
    color: var(--muted);
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
