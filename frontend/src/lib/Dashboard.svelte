<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
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

  // --- reading statistics (KOReader) ---
  const locale = () => document.documentElement.lang || "fr";
  function humanDuration(sec: number): string {
    const h = Math.floor(sec / 3600);
    const m = Math.round((sec % 3600) / 60);
    return h > 0 ? `${h} h ${m.toString().padStart(2, "0")}` : `${m} min`;
  }
  function maxSeconds(list: number[] | null | undefined): number {
    return (list ?? []).reduce((m, x) => Math.max(m, x), 0) || 1;
  }
  // Monday-first short weekday names, localized (2023-01-02 was a Monday).
  function weekdayShort(i: number): string {
    return new Intl.DateTimeFormat(locale(), { weekday: "short" }).format(new Date(2023, 0, 2 + i));
  }
  function fmtDate(d: Date): string {
    const p = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
  }
  function longDate(iso: string): string {
    const d = new Date(iso + "T00:00:00");
    return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString(locale(), { day: "numeric", month: "long", year: "numeric" });
  }
  // A GitHub-style heatmap: `weeks` columns of 7 days ending this week.
  type Cell = { key: string; sec: number };
  function buildHeatmap(byDay: { date: string; seconds: number }[] | null | undefined, weeks: number): Cell[][] {
    const map = new Map((byDay ?? []).map((d) => [d.date, d.seconds]));
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const monday = new Date(today);
    monday.setDate(today.getDate() - ((today.getDay() + 6) % 7));
    const start = new Date(monday);
    start.setDate(monday.getDate() - (weeks - 1) * 7);
    const cols: Cell[][] = [];
    for (let w = 0; w < weeks; w++) {
      const col: Cell[] = [];
      for (let dd = 0; dd < 7; dd++) {
        const date = new Date(start);
        date.setDate(start.getDate() + w * 7 + dd);
        const key = fmtDate(date);
        col.push({ key, sec: date > today ? -1 : map.get(key) ?? 0 });
      }
      cols.push(col);
    }
    return cols;
  }
  function heatLevel(sec: number, max: number): number {
    if (sec < 0) return -1; // future day
    if (sec === 0) return 0;
    const r = sec / max;
    return r < 0.25 ? 1 : r < 0.5 ? 2 : r < 0.75 ? 3 : 4;
  }

  // Heatmap geometry — kept in sync with the CSS (cell/gap/weekday-label width).
  const HM_CELL = 13, HM_GAP = 3, HM_LEFT = 31; // left = weekday-label col + its gap
  const HM_PITCH = HM_CELL + HM_GAP;
  function monthName(iso: string): string {
    return new Intl.DateTimeFormat(locale(), { month: "short" }).format(new Date(iso + "T00:00:00"));
  }
  // One label per column where the month changes (spaced out so they don't collide).
  function monthMarks(cols: Cell[][]): { col: number; label: string }[] {
    const marks: { col: number; label: string }[] = [];
    let prev = "";
    cols.forEach((col, i) => {
      const key = col[0]?.key;
      if (!key) return;
      const ym = key.slice(0, 7);
      if (ym !== prev) {
        if (marks.length === 0 || i - marks[marks.length - 1].col >= 3) {
          marks.push({ col: i, label: monthName(key) });
        }
        prev = ym;
      }
    });
    return marks;
  }

  const heatmap = $derived(buildHeatmap(d?.readingStats?.byDay, 26));
  const heatMonths = $derived(monthMarks(heatmap));

  // Click a heatmap day to see the books read that day (like KOReader's calendar).
  let selectedDay = $state("");
  const dayTotals = $derived(new Map((d?.readingStats?.byDay ?? []).map((x) => [x.date, x.seconds])));
  const activeDay = $derived(selectedDay || (d?.readingStats?.byDay?.at(-1)?.date ?? ""));
  function dayBooksFor(day: string) {
    return d?.readingStats?.dayBooks?.[day] ?? [];
  }

  // Reading timeline: books read per month, with a year selector (default = now).
  const monthBooksMap = $derived(d?.readingStats?.monthBooks ?? {});
  const availableYears = $derived(
    [...new Set(Object.keys(monthBooksMap).map((k) => k.slice(0, 4)))].sort().reverse(),
  );
  let selectedYear = $state("");
  const activeYear = $derived.by(() => {
    if (selectedYear) return selectedYear;
    const now = String(new Date().getFullYear());
    return availableYears.includes(now) ? now : availableYears[0] ?? now;
  });
  function monthFullName(m: number): string {
    const s = new Intl.DateTimeFormat(locale(), { month: "long" }).format(new Date(2023, m - 1, 1));
    return s.charAt(0).toUpperCase() + s.slice(1);
  }
  let expandedMonth = $state(""); // "YYYY-M" of the expanded month, "" = none
  function monthsForYear(year: string) {
    const out: { month: number; total: number; max: number; books: { title: string; seconds: number }[] }[] = [];
    for (let m = 1; m <= 12; m++) {
      const raw = monthBooksMap[`${year}-${m.toString().padStart(2, "0")}`];
      if (raw && raw.length) {
        const books = [...raw].sort((a, b) => b.seconds - a.seconds);
        out.push({
          month: m,
          total: books.reduce((s, b) => s + b.seconds, 0),
          max: books[0]?.seconds || 1,
          books,
        });
      }
    }
    return out;
  }
  const months = $derived(monthsForYear(activeYear));
  const yearMax = $derived(Math.max(1, ...months.map((m) => m.total)));

  // Collapse runs of hours with no reading into a single "00 h – 02 h" row, and
  // keep an individual row for each hour that has reading time.
  type HourRow = { label: string; seconds: number };
  function hourRows(byHour: number[] | null | undefined): HourRow[] {
    const hh = (h: number) => `${h.toString().padStart(2, "0")} h`;
    const arr = byHour ?? [];
    const rows: HourRow[] = [];
    let i = 0;
    while (i < 24) {
      if ((arr[i] ?? 0) > 0) {
        rows.push({ label: hh(i), seconds: arr[i] });
        i++;
      } else {
        let j = i;
        while (j < 24 && (arr[j] ?? 0) === 0) j++;
        rows.push({ label: j - 1 > i ? `${hh(i)} – ${hh(j - 1)}` : hh(i), seconds: 0 });
        i = j;
      }
    }
    return rows;
  }
  const heatMax = $derived(maxSeconds((d?.readingStats?.byDay ?? []).map((x) => x.seconds)));
  const weekdayMax = $derived(maxSeconds(d?.readingStats?.byWeekday));
  const hourMax = $derived(maxSeconds(d?.readingStats?.byHour));
  const topBookMax = $derived(maxSeconds((d?.readingStats?.topBooks ?? []).map((b) => b.seconds)));

  load();

  // Auto-refresh when reading stats are (re)fetched (e.g. on device connect).
  onMount(() => Events.On("reading:statsUpdated", () => load()));
</script>

<div class="dash">
  {#if loading && !d}
    <p class="msg">{t("common.loading")}</p>
  {:else if !d}
    <p class="msg">{t("dashboard.unavailable")}</p>
  {:else}
    <h2 class="sectiontitle">{t("dashboard.section.library")}</h2>
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

    {#if d.readingStats}
      {@const rs = d.readingStats}
      <h2 class="sectiontitle stats">{t("dashboard.stats.title")}</h2>
      <div class="tiles">
        <div class="tile"><span class="tnum">{humanDuration(rs.totalSeconds)}</span><span class="tlbl">{t("dashboard.stats.totalTime")}</span></div>
        <div class="tile"><span class="tnum">{fr(rs.daysRead)}</span><span class="tlbl">{t("dashboard.stats.daysRead")}</span></div>
        <div class="tile"><span class="tnum">{fr(rs.totalPages)}</span><span class="tlbl">{t("dashboard.stats.pagesRead")}</span></div>
        <div class="tile"><span class="tnum">{fr(rs.books)}</span><span class="tlbl">{t("dashboard.stats.booksTracked")}</span></div>
        {#if rs.longestDay?.seconds}
          <div class="tile"><span class="tnum">{humanDuration(rs.longestDay.seconds)}</span><span class="tlbl">{t("dashboard.stats.longestDay")} · {longDate(rs.longestDay.date)}</span></div>
        {/if}
      </div>

      <div class="cards stats">
        <section class="card span2">
          <h3>{t("dashboard.stats.calendar")}</h3>
          <div class="hmscroll">
            <div class="hmcontent">
              <div class="hmmonths" style="height:14px">
                {#each heatMonths as m}
                  <span style="left:{HM_LEFT + m.col * HM_PITCH}px">{m.label}</span>
                {/each}
              </div>
              <div class="hmgrid">
                <div class="hmdays">
                  {#each [0, 1, 2, 3, 4, 5, 6] as w}
                    <span>{w === 0 || w === 2 || w === 4 ? weekdayShort(w) : ""}</span>
                  {/each}
                </div>
                <div class="heatmap" role="group" aria-label={t("dashboard.stats.calendar")}>
                  {#each heatmap as col}
                    <div class="hmcol">
                      {#each col as cell}
                        <button
                          class="hmcell lvl{heatLevel(cell.sec, heatMax)}"
                          class:sel={cell.sec >= 0 && cell.key === activeDay}
                          disabled={cell.sec < 0}
                          title={cell.sec >= 0 ? `${longDate(cell.key)} · ${humanDuration(cell.sec)}` : ""}
                          aria-label={cell.sec >= 0 ? longDate(cell.key) : ""}
                          onclick={() => (selectedDay = cell.key)}
                        ></button>
                      {/each}
                    </div>
                  {/each}
                </div>
              </div>
            </div>
          </div>
          {#if activeDay}
            <div class="daydetail">
              <div class="daydhead">
                <span>{longDate(activeDay)}</span>
                <b>{humanDuration(dayTotals.get(activeDay) ?? 0)}</b>
              </div>
              {#each dayBooksFor(activeDay) as bk}
                <div class="daybook">
                  <span class="ellipsis" title={bk.authors ? `${bk.title} — ${bk.authors}` : bk.title}>{bk.title}</span>
                  <span class="dbtime">{humanDuration(bk.seconds)}</span>
                </div>
              {/each}
              {#if !dayBooksFor(activeDay).length}
                <div class="daynone">{t("dashboard.stats.dayEmpty")}</div>
              {/if}
            </div>
          {/if}
        </section>

        <section class="card">
          <h3>{t("dashboard.stats.byWeekday")}</h3>
          {#each rs.byWeekday ?? [] as secs, i}
            <div class="bar">
              <span class="blabel">{weekdayShort(i)}</span>
              <div class="btrack"><div class="bfill" style="width:{pct(secs, weekdayMax)}%"></div></div>
              <span class="bval">{secs ? humanDuration(secs) : "—"}</span>
            </div>
          {/each}
        </section>

        <section class="card">
          <h3>{t("dashboard.stats.byHour")}</h3>
          {#each hourRows(rs.byHour) as row}
            <div class="bar compact">
              <span class="blabel hour">{row.label}</span>
              <div class="btrack"><div class="bfill" style="width:{pct(row.seconds, hourMax)}%"></div></div>
              <span class="bval">{row.seconds ? humanDuration(row.seconds) : ""}</span>
            </div>
          {/each}
        </section>

        {#if (rs.topBooks ?? []).length}
          <section class="card span2">
            <h3>{t("dashboard.stats.topBooks")}</h3>
            {#each rs.topBooks as bk}
              <div class="bar">
                <span class="blabel ellipsis" title={bk.authors ? `${bk.title} — ${bk.authors}` : bk.title}>{bk.title}</span>
                <div class="btrack"><div class="bfill" style="width:{pct(bk.seconds, topBookMax)}%"></div></div>
                <span class="bval">{humanDuration(bk.seconds)}{bk.lastRead ? " · " + longDate(bk.lastRead) : ""}</span>
              </div>
            {/each}
          </section>
        {/if}

        {#if availableYears.length}
          <section class="card span2">
            <div class="cardhead">
              <h3>{t("dashboard.stats.timeline")}</h3>
              <select class="yearsel" value={activeYear} onchange={(e) => (selectedYear = (e.target as HTMLSelectElement).value)} aria-label={t("dashboard.stats.timeline")}>
                {#each availableYears as y}<option value={y}>{y}</option>{/each}
              </select>
            </div>
            {#if months.length}
              <div class="timeline">
                {#each months as row}
                  {@const key = `${activeYear}-${row.month}`}
                  {@const open = expandedMonth === key}
                  <div class="mgroup">
                    <button class="mrow" onclick={() => (expandedMonth = open ? "" : key)} aria-expanded={open}>
                      <svg class="chev" class:open viewBox="0 0 24 24" aria-hidden="true"><path d="M9 6l6 6-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" /></svg>
                      <span class="mmonth">{monthFullName(row.month)}</span>
                      <div class="mtrack"><div class="mfill" style="width:{pct(row.total, yearMax)}%"></div></div>
                      <span class="mtime">{humanDuration(row.total)}</span>
                    </button>
                    {#if open}
                      <div class="mbooks">
                        {#each row.books as b}
                          <div class="mbook">
                            <span class="mbtitle ellipsis" title={b.title}>{b.title}</span>
                            <div class="bktrack"><div class="bkfill" style="width:{pct(b.seconds, row.max)}%"></div></div>
                            <span class="mbtime">{humanDuration(b.seconds)}</span>
                          </div>
                        {/each}
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            {:else}
              <p class="empty">{t("dashboard.stats.timelineEmpty")}</p>
            {/if}
          </section>
        {/if}
      </div>
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

  /* Section headers */
  .sectiontitle {
    margin: 0 0 1rem;
    font-size: 1rem;
    font-weight: 650;
    letter-spacing: -0.01em;
  }
  .sectiontitle.stats {
    margin-top: 2.2rem;
    padding-top: 1.4rem;
    border-top: 1px solid var(--border);
  }
  /* Reading-statistics cards take their natural height (the hour list is tall). */
  .cards.stats {
    align-items: start;
  }
  /* The calendar and top-books get two grid columns so the heatmap fits without
     scrolling; clamps to one column on narrow layouts. */
  .cards.stats .span2 {
    grid-column: span 2;
  }

  /* Reading statistics — calendar heatmap with month (top) and weekday (left) axes.
     Geometry (cell 13, gap 3, weekday col 26 + 5) is mirrored in JS (HM_* consts). */
  .hmscroll {
    overflow-x: auto;
    padding-bottom: 0.2rem;
  }
  .hmcontent {
    display: inline-block;
  }
  .hmmonths {
    position: relative;
  }
  .hmmonths span {
    position: absolute;
    top: 0;
    font-size: 0.64rem;
    color: var(--faint);
    white-space: nowrap;
  }
  .hmgrid {
    display: flex;
    gap: 5px;
  }
  .hmdays {
    display: flex;
    flex-direction: column;
    gap: 3px;
    width: 26px;
  }
  .hmdays span {
    height: 13px;
    line-height: 13px;
    font-size: 0.6rem;
    color: var(--faint);
    text-align: right;
    padding-right: 2px;
  }
  .heatmap {
    display: flex;
    gap: 3px;
  }
  .hmcol {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .hmcell {
    width: 13px;
    height: 13px;
    border-radius: 3px;
    background: var(--inset);
    border: none;
    padding: 0;
    appearance: none;
    cursor: pointer;
  }
  .hmcell:disabled {
    cursor: default;
  }
  .hmcell.sel {
    outline: 2px solid var(--text);
    outline-offset: 1px;
  }
  .daydetail {
    margin-top: 0.9rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--border);
  }
  .daydhead {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 0.6rem;
    font-size: 0.82rem;
    margin-bottom: 0.5rem;
  }
  .daydhead b {
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .daybook {
    display: flex;
    justify-content: space-between;
    gap: 0.7rem;
    font-size: 0.8rem;
    padding: 0.22rem 0;
    color: var(--muted);
    min-width: 0;
  }
  .daybook .dbtime {
    color: var(--text);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    flex: none;
  }
  .daynone {
    font-size: 0.8rem;
    color: var(--faint);
  }

  /* Reading timeline (books per month) */
  .cardhead {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.8rem;
    margin-bottom: 0.85rem;
  }
  .cardhead h3 {
    margin: 0;
  }
  .yearsel {
    font: inherit;
    font-size: 0.82rem;
    color: var(--text);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.25rem 0.5rem;
    outline: none;
    font-variant-numeric: tabular-nums;
  }
  .timeline {
    display: flex;
    flex-direction: column;
  }
  .mgroup {
    border-bottom: 1px solid var(--border);
  }
  .mgroup:last-child {
    border-bottom: none;
  }
  .mrow {
    display: grid;
    grid-template-columns: 14px 96px minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.7rem;
    width: 100%;
    padding: 0.5rem 0;
    background: none;
    border: none;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .chev {
    width: 12px;
    height: 12px;
    color: var(--faint);
    transition: transform 0.15s;
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .mmonth {
    font-weight: 600;
    font-size: 0.82rem;
  }
  .mtrack {
    height: 8px;
    background: var(--inset);
    border-radius: 999px;
    overflow: hidden;
  }
  .mfill {
    height: 100%;
    background: var(--accent);
    border-radius: 999px;
    min-width: 3px;
  }
  .mtime {
    font-size: 0.8rem;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .mbooks {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    padding: 0.1rem 0 0.65rem 1.55rem;
  }
  .mbook {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 96px auto;
    gap: 0.6rem;
    align-items: center;
  }
  .mbtitle {
    font-size: 0.78rem;
    color: var(--muted);
  }
  .bktrack {
    height: 6px;
    background: var(--inset);
    border-radius: 999px;
    overflow: hidden;
  }
  .bkfill {
    height: 100%;
    background: color-mix(in srgb, var(--accent) 70%, transparent);
    border-radius: 999px;
    min-width: 3px;
  }
  .mbtime {
    font-size: 0.76rem;
    color: var(--text);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .hmcell.lvl-1 {
    background: transparent; /* future day */
  }
  .hmcell.lvl1 {
    background: color-mix(in srgb, var(--accent) 28%, var(--inset));
  }
  .hmcell.lvl2 {
    background: color-mix(in srgb, var(--accent) 52%, var(--inset));
  }
  .hmcell.lvl3 {
    background: color-mix(in srgb, var(--accent) 76%, var(--inset));
  }
  .hmcell.lvl4 {
    background: var(--accent);
  }
  /* Compact rows for the hour list. A fixed label column keeps every bar aligned
     whether the label is a single hour ("03 h") or a collapsed range. */
  .bar.compact {
    grid-template-columns: 84px 1fr auto;
    padding: 0.12rem 0;
    gap: 0.6rem;
  }
  .blabel.hour {
    font-variant-numeric: tabular-nums;
    color: var(--muted);
    font-size: 0.78rem;
    white-space: nowrap;
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
