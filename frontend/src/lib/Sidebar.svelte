<script lang="ts">
  import type { SidebarItem, OPDSStatus, CalibreStatus, ReadingStatusCounts, SmartShelfSummary } from "./api";
  import { t } from "./i18n";
  import type { View, ReadingStatus } from "./types";

  let {
    total,
    authors,
    series,
    tags,
    shelves,
    opds,
    calibre,
    reading,
    annotationCount,
    showDiscover,
    showSmartShelves,
    active,
    onSelect,
    onOpenSettings,
  }: {
    total: number;
    authors: SidebarItem[];
    series: SidebarItem[];
    tags: SidebarItem[];
    shelves: SmartShelfSummary[];
    opds: OPDSStatus | null;
    calibre: CalibreStatus | null;
    reading: ReadingStatusCounts | null;
    annotationCount: number;
    showDiscover: boolean;
    showSmartShelves: boolean;
    active: View;
    onSelect: (v: View) => void;
    onOpenSettings: () => void;
  } = $props();

  // Reading-status filters shown only when they contain at least one book, so an
  // unsynced library stays clean (and "Abandonnés" only appears when relevant).
  const readingFilters = $derived(
    (
      [
        { status: "reading", label: t("nav.reading"), count: reading?.reading ?? 0 },
        { status: "complete", label: t("nav.complete"), count: reading?.complete ?? 0 },
        { status: "abandoned", label: t("nav.abandoned"), count: reading?.abandoned ?? 0 },
      ] as { status: ReadingStatus; label: string; count: number }[]
    ).filter((f) => f.count > 0),
  );

  // "http://192.168.1.10:8080/" → "192.168.1.10:8080" for a compact status line.
  function shortURL(url: string): string {
    return url.replace(/^https?:\/\//, "").replace(/\/$/, "");
  }

  // Which groups are expanded. Authors open by default.
  let open = $state({ author: true, series: false, tag: false });

  type Group = { key: "author" | "series" | "tag"; label: string; items: SidebarItem[] };
  const groups = $derived<Group[]>([
    { key: "author", label: t("nav.authors"), items: authors },
    { key: "series", label: t("nav.series"), items: series },
    { key: "tag", label: t("nav.tags"), items: tags },
  ]);

  function isActive(kind: "author" | "series" | "tag", id: number): boolean {
    return active.kind === kind && active.id === id;
  }
</script>

<aside class="sidebar">
  <div class="brand">
    <img src="/reliure-logo.png" alt="" aria-hidden="true" />
    <span>{t("app.name")}</span>
  </div>

  <button class="root tool" class:active={active.kind === "dashboard"} onclick={() => onSelect({ kind: "dashboard" })}>
    <svg viewBox="0 0 24 24" aria-hidden="true" width="15" height="15">
      <path d="M4 13h6V4H4zM14 20h6V4h-6zM4 20h6v-4H4z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>
    </svg>
    <span>{t("nav.dashboard")}</span>
  </button>

  <button class="root" class:active={active.kind === "all"} onclick={() => onSelect({ kind: "all" })}>
    <span>{t("nav.allBooks")}</span>
    <span class="count">{total}</span>
  </button>

  {#if readingFilters.length}
    <div class="statusrow">
      {#each readingFilters as f (f.status)}
        <button
          class="status"
          class:active={active.kind === "reading" && active.status === f.status}
          onclick={() => onSelect({ kind: "reading", status: f.status })}
        >
          <span class="dot {f.status}"></span>
          <span class="ellipsis">{f.label}</span>
          <span class="count">{f.count}</span>
        </button>
      {/each}
    </div>
  {/if}

  <button class="root tool" class:active={active.kind === "quickedit"} onclick={() => onSelect({ kind: "quickedit" })}>
    <span>{t("nav.quickEdit")}</span>
  </button>

  {#if showSmartShelves}
    <button class="root tool" class:active={active.kind === "shelves"} onclick={() => onSelect({ kind: "shelves" })}>
      <svg viewBox="0 0 24 24" aria-hidden="true" width="15" height="15">
        <path d="M4 6h16M4 12h16M4 18h16" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
        <path d="M7 4v16M17 4v16" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" opacity=".55"/>
      </svg>
      <span>{t("nav.shelves")}</span>
      <span class="count">{shelves.length}</span>
    </button>
  {/if}

  {#if showSmartShelves && shelves.length}
    <div class="statusrow shelves">
      {#each shelves as shelf (shelf.id)}
        <button
          class="status"
          class:active={active.kind === "shelf" && active.id === shelf.id}
          onclick={() => onSelect({ kind: "shelf", id: shelf.id, name: shelf.name })}
          title={shelf.name}
        >
          <span class="dot shelf"></span>
          <span class="ellipsis">{shelf.name}</span>
          <span class="count">{shelf.count}</span>
        </button>
      {/each}
    </div>
  {/if}

  {#if showDiscover}
    <button class="root tool" class:active={active.kind === "gutenberg"} onclick={() => onSelect({ kind: "gutenberg" })}>
      <svg viewBox="0 0 24 24" aria-hidden="true" width="15" height="15">
        <path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H11v16H6.5A2.5 2.5 0 0 0 4 21.5zM20 5.5A2.5 2.5 0 0 0 17.5 3H13v16h4.5a2.5 2.5 0 0 1 2.5 2.5z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round" />
      </svg>
      <span>{t("nav.discover")}</span>
    </button>
  {/if}

  {#if annotationCount > 0}
    <button class="root tool" class:active={active.kind === "annotations"} onclick={() => onSelect({ kind: "annotations" })}>
      <svg viewBox="0 0 24 24" aria-hidden="true" width="15" height="15">
        <path d="M4 19.5V5a1 1 0 0 1 1-1h14a1 1 0 0 1 1 1v9l-6 6H5a1 1 0 0 1-1-1z M14 20v-5a1 1 0 0 1 1-1h5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>
        <path d="M8 9h8M8 12.5h5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
      </svg>
      <span>{t("nav.annotations")}</span>
      <span class="count">{annotationCount}</span>
    </button>
  {/if}

  <nav class="groups">
    {#each groups as g (g.key)}
      <div class="group">
        <button class="group-head" onclick={() => (open[g.key] = !open[g.key])}>
          <svg class="chev" class:open={open[g.key]} viewBox="0 0 24 24" aria-hidden="true"
            ><path d="M9 6l6 6-6 6" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round" /></svg
          >
          <span>{g.label}</span>
          <span class="count">{g.items.length}</span>
        </button>
        {#if open[g.key]}
          <ul>
            {#each g.items as it (it.id)}
              <li>
                <button
                  class="item"
                  class:active={isActive(g.key, it.id)}
                  onclick={() => onSelect({ kind: g.key, id: it.id, name: it.name })}
                  title={it.name}
                >
                  <span class="ellipsis">{it.name}</span>
                  <span class="count">{it.count}</span>
                </button>
              </li>
            {/each}
            {#if g.items.length === 0}
              <li class="empty">{t("common.none")}</li>
            {/if}
          </ul>
        {/if}
      </div>
    {/each}
  </nav>

  {#if opds && (opds.enabled || opds.running)}
    <button
      class="opds"
      class:on={opds.running}
      onclick={onOpenSettings}
      title={opds.running ? opds.url : opds.error || t("nav.opds.stopped")}
    >
      <span class="dot"></span>
      <span class="lbl">{t("nav.opds.status", undefined, { status: opds.running ? t("status.online.lower") : t("status.stopped.lower") })}</span>
      {#if opds.running && opds.url}
        <span class="addr ellipsis">{shortURL(opds.url)}</span>
      {/if}
    </button>
  {/if}

  {#if calibre && calibre.running}
    <button
      class="opds"
      class:on={calibre.connected}
      onclick={onOpenSettings}
      title={calibre.connected ? t("nav.reader.connectedTitle", undefined, { device: calibre.device }) : t("nav.reader.waitingTitle")}
    >
      <span class="dot"></span>
      <span class="lbl">{t("nav.reader.status", undefined, { status: calibre.connected ? t("status.connected.lower") : t("status.waiting.lower") })}</span>
      {#if calibre.connected && calibre.device}
        <span class="addr ellipsis">{calibre.device}</span>
      {:else if calibre.address}
        <span class="addr ellipsis">{calibre.address}</span>
      {/if}
    </button>
  {/if}

  <button class="settings" class:active={active.kind === "settings"} onclick={onOpenSettings}>
    <svg viewBox="0 0 24 24" aria-hidden="true" width="15" height="15"
      ><path
        d="M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
        fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"
        stroke-linejoin="round" /></svg
    >
    {t("nav.settings")}
  </button>
</aside>

<style>
  .sidebar {
    width: 244px;
    flex: none;
    height: 100%;
    display: flex;
    flex-direction: column;
    padding: 2.35rem 0.75rem 0.75rem;
    background: var(--panel);
    border-right: 1px solid var(--border);
    overflow: hidden;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    font-weight: 700;
    font-size: 1.05rem;
    letter-spacing: 0;
    padding: 0.5rem 0.6rem 0.9rem;
  }
  .brand img {
    width: 28px;
    height: 28px;
    flex: none;
    object-fit: contain;
  }

  button {
    font: inherit;
    color: inherit;
    background: none;
    border: none;
    cursor: pointer;
  }

  .root,
  .item,
  .group-head {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.45rem 0.6rem;
    border-radius: 8px;
    text-align: left;
    color: var(--muted);
    transition: background 0.12s, color 0.12s;
  }
  .root {
    font-weight: 600;
    color: var(--text);
    margin-bottom: 0.4rem;
  }
  .root:hover,
  .item:hover,
  .group-head:hover {
    background: var(--surface);
    color: var(--text);
  }
  .root.active,
  .item.active {
    background: color-mix(in srgb, var(--accent) 20%, transparent);
    color: var(--text);
  }

  .statusrow {
    display: flex;
    flex-direction: column;
    gap: 1px;
    margin-bottom: 0.4rem;
  }
  .status {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 0.6rem;
    border-radius: 8px;
    text-align: left;
    color: var(--muted);
    font-size: 0.86rem;
  }
  .status:hover {
    background: var(--surface);
    color: var(--text);
  }
  .status.active {
    background: color-mix(in srgb, var(--accent) 20%, transparent);
    color: var(--text);
  }
  .status .dot {
    flex: none;
    width: 0.55rem;
    height: 0.55rem;
    border-radius: 50%;
    background: var(--faint);
  }
  .status .dot.reading {
    background: var(--accent);
  }
  .status .dot.complete {
    background: var(--ok);
  }
  .status .dot.abandoned {
    background: var(--faint);
  }
  .status .dot.shelf {
    background: color-mix(in srgb, var(--accent) 55%, var(--ok));
  }
  .statusrow.shelves {
    margin-top: -0.25rem;
  }

  .groups {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding-right: 0.15rem;
  }
  .group {
    margin-top: 0.35rem;
  }
  .group-head {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--faint);
  }
  .chev {
    width: 14px;
    height: 14px;
    transition: transform 0.15s;
  }
  .chev.open {
    transform: rotate(90deg);
  }

  ul {
    list-style: none;
    margin: 0.15rem 0 0.3rem;
    padding: 0 0 0 0.4rem;
  }
  .item {
    font-size: 0.86rem;
  }
  .empty {
    color: var(--faint);
    font-size: 0.8rem;
    padding: 0.3rem 0.7rem;
  }

  .count {
    margin-left: auto;
    font-size: 0.72rem;
    color: var(--faint);
    font-variant-numeric: tabular-nums;
  }
  .item.active .count,
  .root.active .count {
    color: var(--muted);
  }

  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .opds {
    flex: none;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.5rem 0.6rem;
    margin-top: 0.65rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--muted);
    text-align: left;
    font-size: 0.8rem;
  }
  .opds:hover {
    background: var(--surface-hi);
    border-color: var(--border-hi);
  }
  .opds .dot {
    flex: none;
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--faint);
  }
  .opds.on .dot {
    background: var(--ok);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--ok) 20%, transparent);
  }
  .opds.on .lbl {
    color: var(--text);
  }
  .opds .addr {
    margin-left: auto;
    color: var(--faint);
    font-variant-numeric: tabular-nums;
    max-width: 105px;
  }

  .settings {
    flex: none;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.55rem 0.6rem;
    margin-top: 0.5rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--muted);
    text-align: left;
  }
  .settings:hover {
    background: var(--surface-hi);
    border-color: var(--border-hi);
    color: var(--text);
  }
  .settings.active {
    background: color-mix(in srgb, var(--accent) 20%, transparent);
    border-color: color-mix(in srgb, var(--accent) 38%, var(--border));
    color: var(--text);
  }
  .settings svg,
  .root.tool svg {
    flex: none;
  }
</style>
