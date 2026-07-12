<script lang="ts">
  import type { SmartShelfDetail, SmartShelfInput, SmartShelfRule, SmartShelfSummary } from "./api";
  import { plural, t } from "./i18n";

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
    { value: "tag", label: t("shelves.field.tag") },
    { value: "author", label: t("shelves.field.author") },
    { value: "series", label: t("shelves.field.series") },
    { value: "language", label: t("shelves.field.language") },
    { value: "format", label: t("shelves.field.format") },
    { value: "reading_status", label: t("shelves.field.reading") },
    { value: "on_device", label: t("shelves.field.onDevice") },
    { value: "added_within_days", label: t("shelves.field.recentAdded") },
    { value: "title", label: t("shelves.field.title") },
  ];
  const operators = [
    { value: "is", label: t("shelves.operator.is") },
    { value: "is_not", label: t("shelves.operator.isNot") },
    { value: "contains", label: t("shelves.operator.contains") },
    { value: "not_contains", label: t("shelves.operator.notContains") },
    { value: "exists", label: t("shelves.operator.exists") },
    { value: "not_exists", label: t("shelves.operator.notExists") },
  ];
  const valueHints: Record<string, string> = {
    reading_status: "unread, reading, complete, abandoned",
    on_device: "present, absent, unknown",
    added_within_days: "30",
    format: "epub, pdf",
    language: "fr, en...",
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
      error = t("shelves.nameRequired");
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
      <h2>{t("shelves.title")}</h2>
      <button class="secondary" onclick={reset}>{t("shelves.newShort")}</button>
    </div>
    {#if loading}
      <p class="muted">{t("shelves.loading")}</p>
    {:else if shelves.length === 0}
      <p class="muted">{t("shelves.empty")}</p>
    {:else}
      <div class="cards">
        {#each shelves as shelf (shelf.id)}
          <article class="card">
            <button class="open" onclick={() => onOpen(shelf)}>
              <span class="name ellipsis">{shelf.name}</span>
              <span class="count">{t("shelves.count", undefined, { count: shelf.count, s: plural(shelf.count) })}</span>
            </button>
            <div class="actions">
              <button onclick={() => edit(shelf)}>{t("shelves.rename")}</button>
              <button class="danger" onclick={() => del(shelf.id)}>{t("common.delete")}</button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>

  <section class="editor">
    <h2>{draft.id ? t("shelves.edit") : t("shelves.new")}</h2>
    <label>
      <span>{t("shelves.name")}</span>
      <input bind:value={draft.name} placeholder={t("shelves.namePlaceholder")} />
    </label>
    <label>
      <span>{t("shelves.match")}</span>
      <select bind:value={draft.match}>
        <option value="all">{t("shelves.allRules")}</option>
        <option value="any">{t("shelves.anyRule")}</option>
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
          <input bind:value={rule.value} placeholder={valueHints[rule.field] ?? t("shelves.value")} />
          <button class="icon" onclick={() => removeRule(i)} aria-label={t("shelves.removeRule")}>×</button>
        </div>
      {/each}
    </div>
    <button class="secondary" onclick={addRule}>{t("shelves.addRule")}</button>
    {#if error}<p class="error">{error}</p>{/if}
    <button class="primary" onclick={save} disabled={saving}>{saving ? t("common.saving") : t("common.save")}</button>
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
    background-color: var(--inset);
    color: var(--text);
    font: inherit;
    font-size: 0.85rem;
    outline: none;
  }
  select {
    padding-right: 2rem;
  }
  input:focus,
  select:focus {
    border-color: var(--border-hi);
    background-color: var(--surface-hi);
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
