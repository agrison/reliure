<script lang="ts">
  import type { BookDetail, BookUpdate } from "./api";

  let {
    book,
    onClose,
    onRemove,
    onSave,
  }: {
    book: BookDetail;
    onClose: () => void;
    onRemove: (book: BookDetail) => Promise<void> | void;
    onSave: (update: BookUpdate) => Promise<void> | void;
  } = $props();
  let removing = $state(false);
  let confirming = $state(false);
  let editing = $state(false);
  let saving = $state(false);
  let form = $state({
    title: "",
    titleSort: "",
    authors: "",
    series: "",
    seriesIndex: "",
    tags: "",
    description: "",
    language: "",
    published: "",
    isbn: "",
    remotePathOverrideEnabled: false,
    remotePathOverride: "",
  });

  function hue(title: string): number {
    let h = 0;
    for (let i = 0; i < title.length; i++) h = (h * 31 + title.charCodeAt(i)) % 360;
    return h;
  }
  function initials(title: string): string {
    return title.split(/\s+/).slice(0, 2).map((w) => w[0] ?? "").join("").toUpperCase();
  }
  function humanSize(n: number): string {
    if (!n) return "";
    const u = ["o", "Ko", "Mo", "Go"];
    let i = 0;
    let v = n;
    while (v >= 1024 && i < u.length - 1) {
      v /= 1024;
      i++;
    }
    return `${v.toFixed(i ? 1 : 0)} ${u[i]}`;
  }
  function humanDate(s: string): string {
    if (!s) return "";
    const d = new Date(s);
    if (Number.isNaN(d.getTime())) return s;
    return new Intl.DateTimeFormat("fr-FR", {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(d);
  }
  function shortHash(s: string): string {
    return s ? `${s.slice(0, 12)}…` : "";
  }

  const roleLabels: Record<string, string> = {
    aut: "", trl: "trad.", edt: "éd.", ill: "ill.", ctb: "contrib.",
  };

  $effect(() => {
    if (!editing) form = editForm(book);
  });

  function editForm(b: BookDetail) {
    return {
      title: b.title,
      titleSort: b.titleSort,
      authors: (b.authors ?? []).map((a) => a.name).join(", "),
      series: b.series,
      seriesIndex: b.seriesIndex ? String(b.seriesIndex) : "",
      tags: (b.tags ?? []).join(", "),
      description: b.description,
      language: b.language,
      published: b.published,
      isbn: b.isbn,
      remotePathOverrideEnabled: b.remotePathOverrideEnabled,
      remotePathOverride: b.remotePathOverride,
    };
  }

  function splitList(s: string): string[] {
    return s.split(",").map((x) => x.trim()).filter(Boolean);
  }

  async function save() {
    if (saving) return;
    saving = true;
    try {
      await onSave({
        id: book.id,
        title: form.title,
        titleSort: form.titleSort,
        authors: splitList(form.authors),
        series: form.series,
        seriesIndex: form.seriesIndex,
        description: form.description,
        language: form.language,
        isbn: form.isbn,
        published: form.published,
        tags: splitList(form.tags),
        remotePathOverrideEnabled: form.remotePathOverrideEnabled,
        remotePathOverride: form.remotePathOverride,
      });
      editing = false;
    } finally {
      saving = false;
    }
  }

  async function remove() {
    if (removing) return;
    if (!confirming) {
      confirming = true;
      return;
    }
    removing = true;
    try {
      await onRemove(book);
    } finally {
      removing = false;
    }
  }
</script>

<div class="scrim" onclick={onClose} role="presentation"></div>

<aside class="drawer">
  <button class="close" onclick={onClose} aria-label="Fermer">✕</button>
  <button class="edit" onclick={() => (editing = !editing)} disabled={saving}>
    {editing ? "Annuler" : "Modifier"}
  </button>

  <div class="hero">
    <div class="cover">
      {#if book.cover}
        <img src={book.cover} alt="" />
      {:else}
        <div class="ph" style="--h:{hue(book.title)}">{initials(book.title)}</div>
      {/if}
    </div>
  </div>

  {#if editing}
    <form class="editform" onsubmit={(e) => { e.preventDefault(); save(); }}>
      <label>
        <span>Titre</span>
        <input bind:value={form.title} required />
      </label>
      <label>
        <span>Titre de tri</span>
        <input bind:value={form.titleSort} />
      </label>
      <label>
        <span>Auteurs</span>
        <input bind:value={form.authors} placeholder="Auteur, Autre auteur" />
      </label>
      <div class="twocol">
        <label>
          <span>Série</span>
          <input bind:value={form.series} />
        </label>
        <label>
          <span>Tome</span>
          <input bind:value={form.seriesIndex} inputmode="decimal" />
        </label>
      </div>
      <label>
        <span>Tags</span>
        <input bind:value={form.tags} placeholder="tag, autre tag" />
      </label>
      <div class="twocol">
        <label>
          <span>Langue</span>
          <input bind:value={form.language} />
        </label>
        <label>
          <span>Publié</span>
          <input bind:value={form.published} />
        </label>
      </div>
      <label>
        <span>ISBN</span>
        <input bind:value={form.isbn} />
      </label>
      <div class="override">
        <label class="check">
          <input type="checkbox" bind:checked={form.remotePathOverrideEnabled} />
          <span>Chemin KOReader personnalisé</span>
        </label>
        <input
          bind:value={form.remotePathOverride}
          disabled={!form.remotePathOverrideEnabled}
          placeholder={book.remotePath}
          aria-label="Chemin KOReader personnalisé"
        />
      </div>
      <label>
        <span>Description</span>
        <textarea bind:value={form.description} rows="7"></textarea>
      </label>
      <button class="save" type="submit" disabled={saving}>{saving ? "Enregistrement…" : "Enregistrer"}</button>
    </form>
  {:else}
    <h2>{book.title}</h2>

    {#if book.authors?.length}
      <p class="authors">
        {#each book.authors as a, i}{i > 0 ? ", " : ""}{a.name}{roleLabels[a.role]
            ? ` (${roleLabels[a.role]})`
            : ""}{/each}
      </p>
    {/if}

    {#if book.series}
      <p class="series">
        {book.series}{book.seriesIndex ? ` — tome ${book.seriesIndex}` : ""}
      </p>
    {/if}

    {#if book.tags?.length}
      <div class="tags">
        {#each book.tags as t}<span class="chip">{t}</span>{/each}
      </div>
    {/if}

    {#if book.description}
      <p class="desc">{book.description}</p>
    {/if}
  {/if}

  <dl class="facts">
    {#if book.language}<div><dt>Langue</dt><dd>{book.language}</dd></div>{/if}
    {#if book.published}<div><dt>Publié</dt><dd>{book.published}</dd></div>{/if}
    {#if book.isbn}<div><dt>ISBN</dt><dd>{book.isbn}</dd></div>{/if}
    {#if book.addedAt}<div><dt>Ajouté</dt><dd>{humanDate(book.addedAt)}</dd></div>{/if}
    {#if book.updatedAt}<div><dt>Modifié</dt><dd>{humanDate(book.updatedAt)}</dd></div>{/if}
  </dl>

  {#if book.titleSort || book.authors?.some((a) => a.sortName)}
    <div class="meta">
      <h3>Tri</h3>
      {#if book.titleSort}
        <div class="kv">
          <span>Titre</span>
          <strong>{book.titleSort}</strong>
        </div>
      {/if}
      {#each book.authors ?? [] as a}
        {#if a.sortName}
          <div class="kv">
            <span>{a.name}</span>
            <strong>{a.sortName}</strong>
          </div>
        {/if}
      {/each}
    </div>
  {/if}

  {#if book.remotePath}
    <div class="meta">
      <h3>KOReader</h3>
      <div class="kv wide">
        <span>{book.remotePathOverrideEnabled ? "Override" : "Chemin"}</span>
        <strong>{book.remotePath}</strong>
      </div>
    </div>
  {/if}

  <div class="files">
    <h3>Fichiers</h3>
    {#each book.files as f}
      <div class="file">
        <div class="filetop">
          <span class="fmt">{f.format}</span>
          <span class="path ellipsis" title={f.path}>{f.path}</span>
          {#if f.size}<span class="sz">{humanSize(f.size)}</span>{/if}
        </div>
        <div class="filemeta">
          {#if f.addedAt}<span>Ajouté {humanDate(f.addedAt)}</span>{/if}
          {#if f.sha256}<span title={f.sha256}>SHA-256 {shortHash(f.sha256)}</span>{/if}
        </div>
      </div>
    {/each}
  </div>

  <div class="actions">
    <button class="danger" onclick={remove} disabled={removing}>
      {removing ? "Retrait…" : confirming ? "Confirmer le retrait" : "Retirer de la bibliothèque"}
    </button>
    {#if confirming && !removing}
      <button class="secondary" onclick={() => (confirming = false)}>Annuler</button>
    {/if}
  </div>
</aside>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    animation: fade 0.15s ease;
  }
  .drawer {
    position: fixed;
    top: 0;
    right: 0;
    height: 100%;
    width: min(420px, 92vw);
    padding: 2rem 1.75rem;
    overflow-y: auto;
    background: var(--panel);
    border-left: 1px solid var(--border);
    box-shadow: -20px 0 50px rgba(0, 0, 0, 0.45);
    animation: slide 0.2s cubic-bezier(0.2, 0.7, 0.2, 1);
  }
  @keyframes fade {
    from { opacity: 0; }
  }
  @keyframes slide {
    from { transform: translateX(20px); opacity: 0.5; }
  }
  .close {
    position: absolute;
    top: 1rem;
    right: 1rem;
    width: 2rem;
    height: 2rem;
    border-radius: 50%;
    border: none;
    background: var(--surface-hi);
    color: var(--muted);
    cursor: pointer;
    font-size: 0.8rem;
  }
  .close:hover {
    color: var(--text);
  }
  .edit {
    position: absolute;
    top: 1rem;
    right: 3.5rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--muted);
    font: inherit;
    font-size: 0.78rem;
    padding: 0.42rem 0.7rem;
    cursor: pointer;
  }
  .edit:hover:not(:disabled) {
    color: var(--text);
    background: var(--surface-hi);
  }

  .hero {
    display: flex;
    justify-content: center;
    margin: 0.5rem 0 1.5rem;
  }
  .cover {
    width: 190px;
    aspect-ratio: 2 / 3;
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
  }
  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .ph {
    width: 100%;
    height: 100%;
    display: grid;
    place-items: center;
    font-weight: 700;
    font-size: 3rem;
    color: #fff;
    background: linear-gradient(145deg, hsl(var(--h) 45% 42%), hsl(calc(var(--h) + 40) 45% 26%));
  }

  h2 {
    margin: 0;
    font-size: 1.4rem;
    line-height: 1.25;
    letter-spacing: -0.01em;
  }
  .authors {
    margin: 0.4rem 0 0;
    color: var(--text);
  }
  .series {
    margin: 0.35rem 0 0;
    color: var(--accent);
    font-size: 0.9rem;
  }

  .editform {
    display: grid;
    gap: 0.75rem;
  }
  .editform label {
    display: grid;
    gap: 0.3rem;
  }
  .editform label span {
    color: var(--faint);
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .editform input,
  .editform textarea {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 0.86rem;
    padding: 0.52rem 0.62rem;
    outline: none;
  }
  .editform textarea {
    resize: vertical;
    line-height: 1.45;
  }
  .editform input:focus,
  .editform textarea:focus {
    border-color: var(--border-hi);
    background: var(--surface-hi);
  }
  .twocol {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 90px;
    gap: 0.65rem;
  }
  .override {
    display: grid;
    gap: 0.45rem;
    padding: 0.65rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
  }
  .override > input {
    background: var(--panel);
  }
  .override > input:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .check {
    display: flex !important;
    grid-template-columns: none !important;
    align-items: center;
    gap: 0.5rem !important;
    color: var(--muted);
    font-size: 0.84rem;
  }
  .check input {
    width: 15px;
    height: 15px;
    accent-color: var(--accent);
  }
  .check span {
    color: var(--muted) !important;
    font-size: 0.84rem !important;
    text-transform: none !important;
    letter-spacing: 0 !important;
  }
  .save {
    margin-top: 0.25rem;
    padding: 0.62rem 0.9rem;
    border: none;
    border-radius: 8px;
    background: var(--accent);
    color: var(--accent-ink);
    font: inherit;
    font-weight: 700;
    cursor: pointer;
  }
  .save:disabled {
    opacity: 0.6;
    cursor: default;
  }

  .tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin-top: 1rem;
  }
  .chip {
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--surface-hi);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.2rem 0.6rem;
  }

  .desc {
    margin: 1.25rem 0 0;
    color: var(--muted);
    font-size: 0.88rem;
    line-height: 1.55;
    user-select: text;
  }

  .facts {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(90px, 1fr));
    gap: 0.9rem;
    margin: 1.5rem 0 0;
    padding: 1rem 0;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
  }
  .facts dt {
    color: var(--faint);
    font-size: 0.66rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .facts dd {
    margin: 0.2rem 0 0;
    font-size: 0.86rem;
    user-select: text;
  }

  .files {
    margin-top: 1.5rem;
  }
  .files h3,
  .meta h3 {
    margin: 0 0 0.6rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--faint);
  }
  .file {
    padding: 0.5rem 0.6rem;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    margin-bottom: 0.4rem;
    font-size: 0.78rem;
  }
  .filetop {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-width: 0;
  }
  .filemeta {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 0.8rem;
    margin-top: 0.35rem;
    color: var(--faint);
    font-size: 0.7rem;
    font-variant-numeric: tabular-nums;
  }
  .fmt {
    text-transform: uppercase;
    font-size: 0.64rem;
    letter-spacing: 0.04em;
    color: var(--accent);
    flex: none;
  }
  .path {
    color: var(--muted);
    flex: 1;
    min-width: 0;
    user-select: text;
  }
  .sz {
    color: var(--faint);
    flex: none;
    font-variant-numeric: tabular-nums;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    margin-top: 1.5rem;
  }
  .kv {
    display: grid;
    grid-template-columns: minmax(80px, 0.45fr) minmax(0, 1fr);
    gap: 0.8rem;
    padding: 0.45rem 0;
    border-bottom: 1px solid var(--border);
    font-size: 0.78rem;
  }
  .kv.wide {
    grid-template-columns: 70px minmax(0, 1fr);
  }
  .kv span {
    min-width: 0;
    color: var(--faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .kv strong {
    min-width: 0;
    color: var(--muted);
    font-weight: 500;
    overflow-wrap: anywhere;
    user-select: text;
  }

  .actions {
    margin-top: 1.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border);
  }
  .danger {
    width: 100%;
    padding: 0.65rem 0.9rem;
    border: 1px solid color-mix(in srgb, #ff6b6b 45%, var(--border));
    border-radius: 8px;
    background: color-mix(in srgb, #ff6b6b 10%, var(--surface));
    color: #ffb3b3;
    font: inherit;
    font-size: 0.86rem;
    font-weight: 650;
    cursor: pointer;
  }
  .danger:hover:not(:disabled) {
    background: color-mix(in srgb, #ff6b6b 15%, var(--surface-hi));
  }
  .danger:disabled {
    cursor: default;
    opacity: 0.6;
  }
  .secondary {
    width: 100%;
    margin-top: 0.5rem;
    padding: 0.55rem 0.9rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--muted);
    font: inherit;
    font-size: 0.84rem;
    cursor: pointer;
  }
  .secondary:hover {
    color: var(--text);
    background: var(--surface-hi);
  }
</style>
