<script lang="ts">
  import type { SmartShelfDetail, SmartShelfInput, SmartShelfRule, SmartShelfSummary } from "./api";

  let {
    shelves,
    loading,
    onOpen,
    onLoad,
    onSave,
    onDelete,
  }: {
    shelves: SmartShelfSummary[];
    loading: boolean;
    onOpen: (shelf: SmartShelfSummary) => void;
    onLoad: (id: number) => Promise<SmartShelfDetail>;
    onSave: (input: SmartShelfInput) => Promise<SmartShelfDetail>;
    onDelete: (id: number) => Promise<void>;
  } = $props();

  const fields = [
    { value: "tag", label: "Tag" },
    { value: "author", label: "Auteur" },
    { value: "series", label: "Série" },
    { value: "language", label: "Langue" },
    { value: "format", label: "Format" },
    { value: "reading_status", label: "Lecture" },
    { value: "on_device", label: "Liseuse" },
    { value: "added_within_days", label: "Ajout récent" },
    { value: "title", label: "Titre" },
  ];
  const operators = [
    { value: "is", label: "est" },
    { value: "is_not", label: "n’est pas" },
    { value: "contains", label: "contient" },
    { value: "not_contains", label: "ne contient pas" },
    { value: "exists", label: "existe" },
    { value: "not_exists", label: "n’existe pas" },
  ];
  const valueHints: Record<string, string> = {
    reading_status: "unread, reading, complete, abandoned",
    on_device: "present, absent, unknown",
    added_within_days: "30",
    format: "epub, pdf",
    language: "fr, en…",
  };

  let draft = $state<SmartShelfInput>({
    id: 0,
    name: "",
    match: "all",
    rules: [{ field: "tag", operator: "is", value: "" }],
    position: 0,
  });
  let saving = $state(false);
  let error = $state("");

  function reset() {
    draft = { id: 0, name: "", match: "all", rules: [{ field: "tag", operator: "is", value: "" }], position: 0 };
    error = "";
  }
  async function edit(shelf: SmartShelfSummary) {
    error = "";
    const detail = await onLoad(shelf.id);
    draft = { id: detail.id, name: detail.name, match: detail.match, rules: detail.rules ?? [], position: detail.position };
  }
  function addRule() {
    draft.rules = [...(draft.rules ?? []), { field: "tag", operator: "is", value: "" }];
  }
  function removeRule(i: number) {
    draft.rules = (draft.rules ?? []).filter((_, idx) => idx !== i);
  }
  async function save() {
    if (!draft.name.trim()) {
      error = "Nom obligatoire";
      return;
    }
    saving = true;
    error = "";
    try {
      const saved = await onSave({ ...draft, rules: draft.rules ?? [] });
      draft = { id: saved.id, name: saved.name, match: saved.match, rules: saved.rules ?? [], position: saved.position };
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }
  async function del(id: number) {
    await onDelete(id);
    if (draft.id === id) reset();
  }
</script>

<div class="shelves-page">
  <section class="list">
    <div class="head">
      <h2>Étagères intelligentes</h2>
      <button class="secondary" onclick={reset}>Nouvelle</button>
    </div>
    {#if loading}
      <p class="muted">Chargement…</p>
    {:else if shelves.length === 0}
      <p class="muted">Aucune étagère pour l’instant.</p>
    {:else}
      <div class="cards">
        {#each shelves as shelf (shelf.id)}
          <article class="card">
            <button class="open" onclick={() => onOpen(shelf)}>
              <span class="name ellipsis">{shelf.name}</span>
              <span class="count">{shelf.count} livre{shelf.count === 1 ? "" : "s"}</span>
            </button>
            <div class="actions">
              <button onclick={() => edit(shelf)}>Renommer</button>
              <button class="danger" onclick={() => del(shelf.id)}>Supprimer</button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>

  <section class="editor">
    <h2>{draft.id ? "Modifier l’étagère" : "Nouvelle étagère"}</h2>
    <label>
      <span>Nom</span>
      <input bind:value={draft.name} placeholder="SF non lus" />
    </label>
    <label>
      <span>Correspondance</span>
      <select bind:value={draft.match}>
        <option value="all">Toutes les règles</option>
        <option value="any">Au moins une règle</option>
      </select>
    </label>

    <div class="rules">
      {#each draft.rules ?? [] as rule, i}
        <div class="rule">
          <select bind:value={rule.field}>
            {#each fields as f}
              <option value={f.value}>{f.label}</option>
            {/each}
          </select>
          <select bind:value={rule.operator} disabled={rule.field === "added_within_days"}>
            {#each operators as op}
              <option value={op.value}>{op.label}</option>
            {/each}
          </select>
          <input bind:value={rule.value} placeholder={valueHints[rule.field] ?? "Valeur"} />
          <button class="icon" onclick={() => removeRule(i)} aria-label="Retirer la règle">×</button>
        </div>
      {/each}
    </div>
    <button class="secondary" onclick={addRule}>Ajouter une règle</button>
    {#if error}<p class="error">{error}</p>{/if}
    <button class="primary" onclick={save} disabled={saving}>{saving ? "Enregistrement…" : "Enregistrer"}</button>
  </section>
</div>

<style>
  .shelves-page {
    display: grid;
    grid-template-columns: minmax(280px, 0.9fr) minmax(360px, 1.1fr);
    gap: 1rem;
    max-width: 1160px;
    margin: 0 auto;
    padding: 1.5rem;
  }
  h2 {
    margin: 0;
    font-size: 1rem;
    letter-spacing: 0;
  }
  .list,
  .editor,
  .card {
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel);
  }
  .list,
  .editor {
    padding: 1rem;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 0.85rem;
  }
  .cards {
    display: grid;
    gap: 0.65rem;
  }
  .card {
    padding: 0.75rem;
  }
  .open {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    border: none;
    background: none;
    color: var(--text);
    text-align: left;
    font: inherit;
    cursor: pointer;
  }
  .name {
    font-weight: 650;
  }
  .count,
  .muted {
    color: var(--muted);
    font-size: 0.82rem;
  }
  .actions {
    display: flex;
    gap: 0.4rem;
    margin-top: 0.65rem;
  }
  label {
    display: grid;
    gap: 0.35rem;
    margin-top: 0.9rem;
    color: var(--muted);
    font-size: 0.78rem;
  }
  input,
  select {
    min-width: 0;
    padding: 0.5rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--inset);
    color: var(--text);
    font: inherit;
    font-size: 0.85rem;
    outline: none;
  }
  input:focus,
  select:focus {
    border-color: var(--border-hi);
    background: var(--surface-hi);
  }
  .rules {
    display: grid;
    gap: 0.55rem;
    margin: 1rem 0 0.7rem;
  }
  .rule {
    display: grid;
    grid-template-columns: 1fr 1fr 1.2fr 32px;
    gap: 0.45rem;
  }
  button {
    font: inherit;
    cursor: pointer;
  }
  .primary,
  .secondary,
  .actions button {
    border-radius: 8px;
    padding: 0.5rem 0.75rem;
  }
  .primary {
    width: 100%;
    margin-top: 1rem;
    border: none;
    background: var(--accent);
    color: var(--accent-ink);
    font-weight: 700;
  }
  .secondary,
  .actions button {
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text);
  }
  .danger {
    color: var(--danger) !important;
  }
  .icon {
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--muted);
  }
  .error {
    color: var(--danger);
    font-size: 0.82rem;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  @media (max-width: 920px) {
    .shelves-page {
      grid-template-columns: 1fr;
    }
  }
</style>
