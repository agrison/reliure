<script lang="ts">
  import type { AppSettings } from "./api";

  let {
    settings,
    onSetMode,
    onChooseFolder,
    onSetRemotePathTemplate,
    onClose,
  }: {
    settings: AppSettings;
    onSetMode: (mode: "copy" | "reference") => void;
    onChooseFolder: () => void;
    onSetRemotePathTemplate: (tmpl: string) => void;
    onClose: () => void;
  } = $props();

  let remotePathTemplate = $state("");

  $effect(() => {
    remotePathTemplate = settings.remotePathTemplate;
  });

  function saveTemplate() {
    if (remotePathTemplate !== settings.remotePathTemplate) {
      onSetRemotePathTemplate(remotePathTemplate);
    }
  }
</script>

<div class="scrim" onclick={onClose} role="presentation"></div>

<div class="modal" role="dialog" aria-label="Réglages">
  <header>
    <h2>Réglages</h2>
    <button class="close" onclick={onClose} aria-label="Fermer">✕</button>
  </header>

  <h3>Mode d'import</h3>
  <div class="seg" role="radiogroup" aria-label="Mode d'import">
    <button
      class="seg-btn"
      class:active={settings.importMode === "copy"}
      aria-pressed={settings.importMode === "copy"}
      onclick={() => onSetMode("copy")}
    >
      Copier dans la bibliothèque
    </button>
    <button
      class="seg-btn"
      class:active={settings.importMode === "reference"}
      aria-pressed={settings.importMode === "reference"}
      onclick={() => onSetMode("reference")}
    >
      Indexer sur place
    </button>
  </div>

  {#if settings.importMode === "copy"}
    <p class="hint">Les livres sont copiés dans un dossier géré par Reliure.</p>
    <div class="folder">
      <span class="path ellipsis" title={settings.libraryDir}>{settings.libraryDir}</span>
      <button class="link" onclick={onChooseFolder}>Modifier…</button>
    </div>
  {:else}
    <p class="hint">
      Les fichiers restent à leur emplacement d'origine ; Reliure ne fait que les
      référencer, sans copie.
    </p>
  {/if}

  <h3 class="mt">Chemin KOReader</h3>
  <p class="hint">
    Modèle utilisé lors des envois futurs. Variables disponibles : {`{authors}`}, {`{series}`},
    {`{series_index}`}, {`{title}`}, {`{tags}`}, {`{language}`}.
  </p>
  <input
    class="template"
    bind:value={remotePathTemplate}
    onblur={saveTemplate}
    onkeydown={(e) => {
      if (e.key === "Enter") saveTemplate();
    }}
  />
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
  }
  .modal {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: min(460px, 92vw);
    padding: 1.5rem 1.75rem 1.75rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 16px;
    box-shadow: 0 30px 70px rgba(0, 0, 0, 0.55);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1.25rem;
  }
  h2 {
    margin: 0;
    font-size: 1.2rem;
  }
  .close {
    width: 2rem;
    height: 2rem;
    border-radius: 50%;
    border: none;
    background: var(--surface-hi);
    color: var(--muted);
    cursor: pointer;
  }
  .close:hover {
    color: var(--text);
  }
  h3 {
    margin: 0 0 0.6rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--faint);
  }
  .mt {
    margin-top: 1.35rem;
  }
  .seg {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 4px;
    padding: 4px;
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid var(--border);
    border-radius: 10px;
  }
  .seg-btn {
    padding: 0.55rem 0.5rem;
    font: inherit;
    font-size: 0.85rem;
    color: var(--muted);
    background: transparent;
    border: none;
    border-radius: 7px;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }
  .seg-btn:hover:not(.active) {
    color: var(--text);
  }
  .seg-btn.active {
    color: var(--text);
    background: rgba(255, 255, 255, 0.08);
    box-shadow: inset 0 0 0 1px var(--border);
  }
  .hint {
    margin: 0.8rem 0 0;
    color: var(--muted);
    font-size: 0.82rem;
    line-height: 1.45;
  }
  .folder {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 0.7rem;
    padding: 0.55rem 0.7rem;
    background: rgba(0, 0, 0, 0.22);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 0.8rem;
  }
  .path {
    color: var(--muted);
    flex: 1;
    min-width: 0;
    user-select: text;
  }
  .link {
    flex: none;
    font: inherit;
    font-size: 0.8rem;
    color: var(--accent);
    background: none;
    border: none;
    cursor: pointer;
    padding: 0;
  }
  .link:hover {
    text-decoration: underline;
  }
  .template {
    width: 100%;
    margin-top: 0.7rem;
    padding: 0.55rem 0.65rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 0.82rem;
    outline: none;
  }
  .template:focus {
    border-color: var(--border-hi);
    background: var(--surface-hi);
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
