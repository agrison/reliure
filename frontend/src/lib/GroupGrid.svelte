<script lang="ts">
  import type { SidebarItem } from "./api";

  let {
    items,
    kind,
    onOpen,
  }: {
    items: SidebarItem[];
    kind: "author" | "series" | "tag";
    onOpen: (item: SidebarItem) => void;
  } = $props();

  const labels = {
    author: "Auteur",
    series: "Série",
    tag: "Tag",
  };

  function initials(name: string): string {
    return name
      .split(/\s+/)
      .slice(0, 2)
      .map((w) => w[0] ?? "")
      .join("")
      .toUpperCase();
  }

  function hue(name: string): number {
    let h = 0;
    for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) % 360;
    return h;
  }
</script>

<div class="groups">
  {#each items as item (item.id)}
    <button class="group" onclick={() => onOpen(item)} title={item.name}>
      <div class="mark" style="--h:{hue(item.name)}">{initials(item.name)}</div>
      <div class="body">
        <div class="kind">{labels[kind]}</div>
        <div class="name ellipsis">{item.name}</div>
        <div class="count">{item.count} livre{item.count === 1 ? "" : "s"}</div>
      </div>
    </button>
  {/each}
</div>

<style>
  .groups {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 0.8rem;
    padding: 1.5rem;
  }
  .group {
    display: grid;
    grid-template-columns: 54px minmax(0, 1fr);
    align-items: center;
    gap: 0.85rem;
    min-width: 0;
    padding: 0.85rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .group:hover {
    background: var(--surface-hi);
    border-color: var(--border-hi);
  }
  .mark {
    width: 54px;
    height: 72px;
    display: grid;
    place-items: center;
    border-radius: 6px;
    color: #fff;
    font-weight: 750;
    background: linear-gradient(145deg, hsl(var(--h) 42% 42%), hsl(calc(var(--h) + 50) 38% 24%));
    box-shadow: 0 8px 18px rgba(0, 0, 0, 0.28);
  }
  .body {
    min-width: 0;
  }
  .kind {
    color: var(--faint);
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .name {
    margin-top: 0.2rem;
    font-size: 0.94rem;
    font-weight: 650;
  }
  .count {
    margin-top: 0.35rem;
    color: var(--muted);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
