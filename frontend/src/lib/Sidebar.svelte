<script lang="ts">
  import type { SidebarItem, OPDSStatus } from "./api";
  import type { View } from "./types";

  let {
    total,
    authors,
    series,
    tags,
    opds,
    active,
    onSelect,
    onOpenSettings,
  }: {
    total: number;
    authors: SidebarItem[];
    series: SidebarItem[];
    tags: SidebarItem[];
    opds: OPDSStatus | null;
    active: View;
    onSelect: (v: View) => void;
    onOpenSettings: () => void;
  } = $props();

  // "http://192.168.1.10:8080/" → "192.168.1.10:8080" for a compact status line.
  function shortURL(url: string): string {
    return url.replace(/^https?:\/\//, "").replace(/\/$/, "");
  }

  // Which groups are expanded. Authors open by default.
  let open = $state({ author: true, series: false, tag: false });

  type Group = { key: "author" | "series" | "tag"; label: string; items: SidebarItem[] };
  const groups = $derived<Group[]>([
    { key: "author", label: "Auteurs", items: authors },
    { key: "series", label: "Séries", items: series },
    { key: "tag", label: "Tags", items: tags },
  ]);

  function isActive(kind: "author" | "series" | "tag", id: number): boolean {
    return active.kind === kind && active.id === id;
  }
</script>

<aside class="sidebar">
  <div class="brand">Reliure</div>

  <button class="root" class:active={active.kind === "all"} onclick={() => onSelect({ kind: "all" })}>
    <span>Tous les livres</span>
    <span class="count">{total}</span>
  </button>

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
              <li class="empty">—</li>
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
      title={opds.running ? opds.url : opds.error || "Serveur OPDS arrêté"}
    >
      <span class="dot"></span>
      <span class="lbl">OPDS {opds.running ? "en ligne" : "arrêté"}</span>
      {#if opds.running && opds.url}
        <span class="addr ellipsis">{shortURL(opds.url)}</span>
      {/if}
    </button>
  {/if}

  <button class="settings" onclick={onOpenSettings}>
    <svg viewBox="0 0 24 24" aria-hidden="true" width="15" height="15"
      ><path
        d="M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
        fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"
        stroke-linejoin="round" /></svg
    >
    Réglages
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
    font-weight: 700;
    font-size: 1.05rem;
    letter-spacing: -0.01em;
    padding: 0.5rem 0.6rem 0.9rem;
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
  .settings svg {
    flex: none;
  }
</style>
