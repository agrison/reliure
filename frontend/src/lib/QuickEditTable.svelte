<script lang="ts">
  import type { QuickEditRow, QuickEditSaveResult } from "./api";

  let {
    rows,
    saving,
    onSave,
    onReload,
  }: {
    rows: QuickEditRow[];
    saving: boolean;
    onSave: (rows: QuickEditRow[]) => Promise<QuickEditSaveResult>;
    onReload: () => void;
  } = $props();

  let draft = $state<QuickEditRow[]>([]);
  let dirty = $state<Set<number>>(new Set());
  let errors = $state<Record<number, string>>({});
  let filter = $state("");

  $effect(() => {
    draft = rows.map((r) => ({ ...r }));
    dirty = new Set();
    errors = {};
  });

  const visible = $derived.by(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return draft;
    return draft.filter((r) =>
      [r.id, r.title, r.authors, r.series, r.tags, r.language, r.isbn]
        .join(" ")
        .toLowerCase()
        .includes(q),
    );
  });
  const dirtyCount = $derived(dirty.size);

  function mark(id: number) {
    dirty = new Set(dirty).add(id);
    if (errors[id]) {
      const next = { ...errors };
      delete next[id];
      errors = next;
    }
  }

  function validateRow(row: QuickEditRow): string {
    if (!row.title.trim()) return "Titre obligatoire";
    const idx = row.seriesIndex.trim().replace(",", ".");
    if (idx && Number.isNaN(Number(idx))) return "Index série invalide";
    return "";
  }

  async function save() {
    const changed = draft.filter((r) => dirty.has(r.id));
    const nextErrors: Record<number, string> = {};
    for (const row of changed) {
      const err = validateRow(row);
      if (err) nextErrors[row.id] = err;
    }
    if (Object.keys(nextErrors).length) {
      errors = nextErrors;
      return;
    }
    const res = await onSave(changed);
    const stillDirty = new Set(dirty);
    const mergedErrors: Record<number, string> = {};
    for (const saved of res.rows ?? []) {
      stillDirty.delete(saved.id);
      const row = draft.find((r) => r.id === saved.id);
      if (row) row.updatedAt = saved.updatedAt;
    }
    for (const err of res.errors ?? []) {
      mergedErrors[err.id] = err.error;
    }
    dirty = stillDirty;
    errors = mergedErrors;
  }
</script>

<section class="quick">
  <div class="bar">
    <div class="search">
      <input bind:value={filter} placeholder="Filtrer les lignes…" aria-label="Filtrer les lignes" />
    </div>
    <span class="count">{visible.length}/{draft.length}</span>
    {#if dirtyCount}
      <span class="dirty">{dirtyCount} modifié{dirtyCount === 1 ? "" : "s"}</span>
    {/if}
    <button class="ghost" onclick={onReload} disabled={saving}>Recharger</button>
    <button class="save" onclick={save} disabled={saving || dirtyCount === 0}>
      {saving ? "Sauvegarde…" : "Sauvegarder"}
    </button>
  </div>

  <div class="tablewrap">
    <table>
      <thead>
        <tr>
          <th class="id">ID</th>
          <th>Titre</th>
          <th>Titre tri</th>
          <th>Auteurs</th>
          <th>Série</th>
          <th class="n">No</th>
          <th>Tags</th>
          <th class="short">Langue</th>
          <th class="date">Date</th>
          <th>ISBN</th>
          <th class="override">Chemin KOReader</th>
        </tr>
      </thead>
      <tbody>
        {#each visible as row (row.id)}
          <tr class:changed={dirty.has(row.id)} class:error={!!errors[row.id]}>
            <td class="id mono" title={errors[row.id] || ""}>{row.id}</td>
            <td><input bind:value={row.title} oninput={() => mark(row.id)} /></td>
            <td><input bind:value={row.titleSort} oninput={() => mark(row.id)} /></td>
            <td><input bind:value={row.authors} oninput={() => mark(row.id)} /></td>
            <td><input bind:value={row.series} oninput={() => mark(row.id)} /></td>
            <td><input class="num" bind:value={row.seriesIndex} oninput={() => mark(row.id)} /></td>
            <td><input bind:value={row.tags} oninput={() => mark(row.id)} /></td>
            <td><input class="short" bind:value={row.language} oninput={() => mark(row.id)} /></td>
            <td><input class="date" bind:value={row.published} oninput={() => mark(row.id)} /></td>
            <td><input bind:value={row.isbn} oninput={() => mark(row.id)} /></td>
            <td class="overridecell">
              <label class="check" title="Override KOReader">
                <input type="checkbox" bind:checked={row.remotePathOverrideEnabled} onchange={() => mark(row.id)} />
              </label>
              <input bind:value={row.remotePathOverride} oninput={() => mark(row.id)} disabled={!row.remotePathOverrideEnabled} />
            </td>
          </tr>
          {#if errors[row.id]}
            <tr class="errmsg">
              <td></td>
              <td colspan="10">{errors[row.id]}</td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
</section>

<style>
  .quick {
    height: 100%;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .bar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 0.7rem;
    padding: 0.7rem 1rem;
    border-bottom: 1px solid var(--border);
    background: var(--panel);
  }
  .search {
    flex: 1;
    min-width: 180px;
  }
  .search input {
    width: 100%;
    padding: 0.48rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 0.84rem;
    outline: none;
  }
  .count,
  .dirty {
    color: var(--muted);
    font-size: 0.82rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  button {
    padding: 0.45rem 0.75rem;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 0.82rem;
    cursor: pointer;
  }
  button.save {
    border: none;
    background: var(--accent);
    color: var(--accent-ink);
    font-weight: 650;
  }
  button:disabled {
    opacity: 0.55;
    cursor: default;
  }
  .tablewrap {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  table {
    width: max(1500px, 100%);
    border-collapse: separate;
    border-spacing: 0;
    table-layout: fixed;
    font-size: 0.82rem;
  }
  th {
    position: sticky;
    top: 0;
    z-index: 2;
    height: 34px;
    padding: 0 0.45rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border-hi);
    color: var(--faint);
    text-align: left;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  td {
    height: 36px;
    padding: 0;
    border-bottom: 1px solid var(--border);
    background: color-mix(in srgb, var(--surface) 54%, transparent);
  }
  tr.changed td {
    background: color-mix(in srgb, var(--accent) 10%, var(--surface));
  }
  tr.error td {
    background: color-mix(in srgb, #ff6b5f 14%, var(--surface));
  }
  th.id,
  td.id {
    position: sticky;
    left: 0;
    z-index: 3;
    width: 72px;
    padding: 0 0.55rem;
    background: var(--panel);
  }
  td.id {
    z-index: 1;
    color: var(--muted);
  }
  th.n {
    width: 70px;
  }
  th.short {
    width: 84px;
  }
  th.date {
    width: 120px;
  }
  th.override {
    width: 260px;
  }
  input {
    width: 100%;
    height: 100%;
    min-width: 0;
    padding: 0 0.45rem;
    border: none;
    border-left: 1px solid var(--border);
    background: transparent;
    color: var(--text);
    font: inherit;
    outline: none;
  }
  input:focus {
    background: var(--surface-hi);
    box-shadow: inset 0 0 0 1px var(--border-hi);
  }
  input.num,
  input.short,
  input.date,
  .mono {
    font-variant-numeric: tabular-nums;
  }
  .overridecell {
    display: grid;
    grid-template-columns: 32px minmax(0, 1fr);
    align-items: stretch;
  }
  .check {
    display: grid;
    place-items: center;
    border-left: 1px solid var(--border);
  }
  .check input {
    width: 15px;
    height: 15px;
    padding: 0;
    border: 0;
    accent-color: var(--accent);
  }
  input:disabled {
    color: var(--faint);
  }
  .errmsg td {
    height: 28px;
    padding: 0 0.6rem;
    background: color-mix(in srgb, #ff6b5f 12%, var(--panel));
    color: #ffb0a6;
    font-size: 0.78rem;
  }
</style>
