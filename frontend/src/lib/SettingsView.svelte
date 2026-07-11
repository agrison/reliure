<script lang="ts">
  import type { AppSettings, OPDSStatus, CalibreStatus } from "./api";

  let {
    settings,
    opdsStatus,
    calibre,
    onSetMode,
    onChooseFolder,
    onSetRemotePathTemplate,
    onSetOPDSEnabled,
    onSetOPDSPort,
    onSetCalibreEnabled,
    onSetWriteMetadataToFile,
    onSetFeatureDiscover,
    onSetFeatureSmartShelves,
    onRegenerateCovers,
    onSetTheme,
    onChooseKoreader,
    onSyncKoreader,
    onSyncKoreaderFromDevice,
    syncingKoreader,
  }: {
    settings: AppSettings;
    opdsStatus: OPDSStatus;
    calibre: CalibreStatus | null;
    onSetMode: (mode: "copy" | "reference") => void;
    onChooseFolder: () => void;
    onSetRemotePathTemplate: (tmpl: string) => void;
    onSetOPDSEnabled: (enabled: boolean) => void;
    onSetOPDSPort: (port: number) => void;
    onSetCalibreEnabled: (enabled: boolean) => void;
    onSetWriteMetadataToFile: (enabled: boolean) => void;
    onSetFeatureDiscover: (enabled: boolean) => void;
    onSetFeatureSmartShelves: (enabled: boolean) => void;
    onRegenerateCovers: () => Promise<void> | void;
    onSetTheme: (theme: "system" | "light" | "dark") => void;
    onChooseKoreader: () => Promise<void> | void;
    onSyncKoreader: () => Promise<void> | void;
    onSyncKoreaderFromDevice: () => Promise<void> | void;
    syncingKoreader: boolean;
  } = $props();

  const themes: { value: "system" | "light" | "dark"; label: string }[] = [
    { value: "system", label: "Auto" },
    { value: "light", label: "Clair" },
    { value: "dark", label: "Sombre" },
  ];

  let regenerating = $state(false);
  let remotePathTemplate = $state("");
  let opdsPort = $state("");

  $effect(() => {
    remotePathTemplate = settings.remotePathTemplate;
    opdsPort = String(opdsStatus.port);
  });

  async function regenerate() {
    if (regenerating) return;
    regenerating = true;
    try {
      await onRegenerateCovers();
    } finally {
      regenerating = false;
    }
  }

  function saveTemplate() {
    if (remotePathTemplate !== settings.remotePathTemplate) {
      onSetRemotePathTemplate(remotePathTemplate);
    }
  }

  function savePort() {
    const port = Number.parseInt(opdsPort, 10);
    if (Number.isInteger(port) && port > 0 && port <= 65535 && port !== opdsStatus.port) {
      onSetOPDSPort(port);
    } else {
      opdsPort = String(opdsStatus.port);
    }
  }
</script>

<div class="settings-page">
  <section class="section">
    <div class="section-head">
      <div>
        <h2>Réseau</h2>
        <p>Services utilisés par KOReader pour récupérer ou recevoir les livres.</p>
      </div>
    </div>

    <div class="grid">
      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>OPDS</h3>
            <p>Catalogue WiFi consultable depuis KOReader.</p>
          </div>
          <span class="status" class:on={opdsStatus.running}>{opdsStatus.running ? "En ligne" : "Arrêté"}</span>
        </div>

        <label class="toggle">
          <input
            type="checkbox"
            checked={opdsStatus.enabled}
            onchange={(e) => onSetOPDSEnabled((e.target as HTMLInputElement).checked)}
          />
          <span>Activer le catalogue</span>
        </label>

        <div class="field-row">
          <label for="opds-port">Port</label>
          <input
            id="opds-port"
            class="port"
            inputmode="numeric"
            bind:value={opdsPort}
            onblur={savePort}
            onkeydown={(e) => {
              if (e.key === "Enter") savePort();
            }}
          />
        </div>

        {#if opdsStatus.url}
          <div class="path-row">
            <span class="path ellipsis" title={opdsStatus.url}>{opdsStatus.url}</span>
          </div>
        {/if}

        {#if opdsStatus.error}
          <p class="error">{opdsStatus.error}</p>
        {/if}
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>Liseuse</h3>
            <p>Serveur Calibre wireless pour l’envoi direct vers KOReader.</p>
          </div>
          <span class="status" class:on={calibre?.connected ?? false}>
            {calibre?.connected ? "Connectée" : calibre?.running ? "En attente" : "Arrêté"}
          </span>
        </div>

        <label class="toggle">
          <input
            type="checkbox"
            checked={calibre?.running ?? false}
            onchange={(e) => onSetCalibreEnabled((e.target as HTMLInputElement).checked)}
          />
          <span>Activer l’envoi vers la liseuse</span>
        </label>

        {#if calibre?.running}
          <p class="hint">Dans KOReader : Calibre → connexion sans fil. Si la découverte automatique échoue, utilisez cette adresse.</p>
          <div class="path-row">
            <span class="path ellipsis" title={calibre.address}>{calibre.address || `port ${calibre.port}`}</span>
          </div>
          {#if calibre.connected && calibre.device}
            <p class="hint">Appareil connecté : {calibre.device}</p>
          {/if}
        {/if}
      </article>
    </div>
  </section>

  <section class="section">
    <div class="section-head">
      <div>
        <h2>Bibliothèque</h2>
        <p>Import, organisation locale, chemin KOReader et maintenance des couvertures.</p>
      </div>
    </div>

    <div class="grid">
      <article class="panel wide">
        <div class="panel-head">
          <div>
            <h3>Import</h3>
            <p>Définit si Reliure copie les fichiers ou les référence sur place.</p>
          </div>
        </div>

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
          <div class="path-row">
            <span class="path ellipsis" title={settings.libraryDir}>{settings.libraryDir}</span>
            <button class="link" onclick={onChooseFolder}>Modifier…</button>
          </div>
        {:else}
          <p class="hint">Les fichiers restent à leur emplacement d’origine ; Reliure ne fait que les indexer.</p>
        {/if}
      </article>

      <article class="panel wide">
        <div class="panel-head">
          <div>
            <h3>Chemin KOReader</h3>
            <p>Modèle de chemin utilisé pour les prochains envois vers la liseuse.</p>
          </div>
        </div>

        <input
          class="template"
          bind:value={remotePathTemplate}
          onblur={saveTemplate}
          onkeydown={(e) => {
            if (e.key === "Enter") saveTemplate();
          }}
        />
        <p class="hint">Variables : {`{authors}`}, {`{series}`}, {`{series_index}`}, {`{title}`}, {`{tags}`}, {`{language}`}.</p>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>Métadonnées EPUB</h3>
            <p>Option avancée pour synchroniser les fichiers avec la base Reliure.</p>
          </div>
        </div>

        <label class="toggle">
          <input
            type="checkbox"
            checked={settings.writeMetadataToFile}
            onchange={(e) => onSetWriteMetadataToFile((e.target as HTMLInputElement).checked)}
          />
          <span>Écrire dans le fichier EPUB</span>
        </label>
        <p class="hint">Modifie le fichier sur disque. En mode indexé sur place, cela édite les originaux.</p>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>Vignettes</h3>
            <p>Reconstruit les couvertures absentes dans le cache local.</p>
          </div>
        </div>

        <button class="action" onclick={regenerate} disabled={regenerating}>
          {regenerating ? "Génération…" : "Régénérer les vignettes"}
        </button>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>Lecture (KOReader)</h3>
            <p>Importe la progression, le statut et les surlignages depuis les fichiers <code>.sdr</code> de la liseuse.</p>
          </div>
        </div>

        <div class="subhead">Depuis la liseuse connectée (WiFi)</div>
        {#if calibre?.connected}
          <button class="action" onclick={onSyncKoreaderFromDevice} disabled={syncingKoreader}>
            {syncingKoreader ? "Synchronisation…" : `Synchroniser depuis ${calibre.device || "la liseuse"}`}
          </button>
        {:else}
          <button class="action" disabled>Aucune liseuse connectée</button>
          <p class="hint">Ouvrez « Calibre » dans KOReader et connectez-vous au serveur (section Réseau ci-dessus), puis revenez ici.</p>
        {/if}

        <div class="subhead">Depuis un dossier (USB / synchronisé)</div>
        {#if settings.koreaderSyncDir}
          <div class="path-row"><span class="path ellipsis" title={settings.koreaderSyncDir}>{settings.koreaderSyncDir}</span></div>
        {/if}
        <div class="btnrow">
          <button class="action" onclick={onChooseKoreader} disabled={syncingKoreader}>
            {syncingKoreader ? "Synchronisation…" : settings.koreaderSyncDir ? "Changer de dossier…" : "Choisir un dossier…"}
          </button>
          {#if settings.koreaderSyncDir}
            <button class="action ghost" onclick={onSyncKoreader} disabled={syncingKoreader}>
              {syncingKoreader ? "…" : "Resynchroniser"}
            </button>
          {/if}
        </div>
      </article>
    </div>
  </section>

  <section class="section">
    <div class="section-head">
      <div>
        <h2>Fonctionnalités</h2>
        <p>Modules optionnels affichés dans la barre latérale.</p>
      </div>
    </div>

    <div class="grid">
      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>Découvrir</h3>
            <p>Recherche et import depuis Project Gutenberg.</p>
          </div>
        </div>
        <label class="toggle">
          <input
            type="checkbox"
            checked={settings.featureDiscover}
            onchange={(e) => onSetFeatureDiscover((e.target as HTMLInputElement).checked)}
          />
          <span>Afficher Découvrir</span>
        </label>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>Étagères intelligentes</h3>
            <p>Collections dynamiques basées sur des règles.</p>
          </div>
        </div>
        <label class="toggle">
          <input
            type="checkbox"
            checked={settings.featureSmartShelves}
            onchange={(e) => onSetFeatureSmartShelves((e.target as HTMLInputElement).checked)}
          />
          <span>Afficher les étagères</span>
        </label>
      </article>
    </div>
  </section>

  <section class="section compact">
    <div class="section-head">
      <div>
        <h2>Apparence</h2>
        <p>Thème de l’interface.</p>
      </div>
    </div>
    <div class="seg appearance" role="radiogroup" aria-label="Thème">
      {#each themes as t}
        <button
          class="seg-btn"
          class:active={(settings.theme || "system") === t.value}
          aria-pressed={(settings.theme || "system") === t.value}
          onclick={() => onSetTheme(t.value)}
        >
          {t.label}
        </button>
      {/each}
    </div>
  </section>
</div>

<style>
  .settings-page {
    max-width: 1120px;
    margin: 0 auto;
    padding: 1.5rem;
  }
  .section {
    padding: 1.25rem 0 1.6rem;
    border-bottom: 1px solid var(--border);
  }
  .section:first-child {
    padding-top: 0;
  }
  .section:last-child {
    border-bottom: none;
  }
  .section.compact {
    max-width: 560px;
  }
  .section-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  h2,
  h3,
  p {
    margin: 0;
  }
  h2 {
    font-size: 1rem;
    letter-spacing: 0;
  }
  .section-head p,
  .panel-head p,
  .hint {
    color: var(--muted);
    font-size: 0.82rem;
    line-height: 1.45;
  }
  .section-head p {
    margin-top: 0.25rem;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.85rem;
  }
  .panel {
    padding: 1rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel);
  }
  .panel.wide {
    grid-column: 1 / -1;
  }
  .panel-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.8rem;
    margin-bottom: 0.9rem;
  }
  h3 {
    font-size: 0.9rem;
    letter-spacing: 0;
  }
  .status {
    flex: none;
    padding: 0.2rem 0.45rem;
    border-radius: 999px;
    background: var(--surface);
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 650;
  }
  .status.on {
    background: color-mix(in srgb, var(--ok) 18%, transparent);
    color: var(--ok);
  }
  .toggle {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    color: var(--text);
    font-size: 0.88rem;
  }
  .toggle input {
    width: 16px;
    height: 16px;
    accent-color: var(--accent);
  }
  .field-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 0.9rem;
  }
  .field-row label {
    width: 3rem;
    color: var(--muted);
    font-size: 0.82rem;
  }
  .port,
  .template {
    min-width: 0;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--inset);
    color: var(--text);
    font: inherit;
    font-size: 0.86rem;
    outline: none;
  }
  .port {
    width: 7rem;
    padding: 0.45rem 0.55rem;
  }
  .template {
    width: 100%;
    padding: 0.6rem 0.7rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }
  .port:focus,
  .template:focus {
    border-color: var(--border-hi);
    background: var(--surface-hi);
  }
  .path-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 0.85rem;
    padding: 0.55rem 0.7rem;
    background: var(--inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 0.8rem;
  }
  .path {
    flex: 1;
    min-width: 0;
    color: var(--muted);
    user-select: text;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .link {
    flex: none;
    border: none;
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: 0.8rem;
    cursor: pointer;
  }
  .link:hover {
    text-decoration: underline;
  }
  .hint {
    margin-top: 0.75rem;
  }
  .error {
    margin-top: 0.75rem;
    color: var(--danger);
    font-size: 0.82rem;
  }
  .seg {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: 1fr;
    gap: 4px;
    padding: 4px;
    background: var(--inset);
    border: 1px solid var(--border);
    border-radius: 10px;
  }
  .seg.appearance {
    max-width: 360px;
  }
  .seg-btn {
    min-width: 0;
    padding: 0.55rem 0.5rem;
    border: none;
    border-radius: 7px;
    background: transparent;
    color: var(--muted);
    font: inherit;
    font-size: 0.85rem;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }
  .seg-btn:hover:not(.active) {
    color: var(--text);
  }
  .seg-btn.active {
    background: var(--surface-hi);
    color: var(--text);
    box-shadow: inset 0 0 0 1px var(--border);
  }
  .action {
    width: 100%;
    padding: 0.6rem 0.9rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 0.86rem;
    cursor: pointer;
  }
  .action:hover:not(:disabled) {
    background: var(--surface-hi);
    border-color: var(--border-hi);
  }
  .action:disabled {
    cursor: default;
    opacity: 0.6;
  }
  .subhead {
    margin-top: 1rem;
    margin-bottom: 0.15rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--faint);
  }
  .subhead:first-of-type {
    margin-top: 0.5rem;
  }
  .btnrow {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.85rem;
  }
  .btnrow .action {
    flex: 1;
    width: auto;
  }
  .action.ghost {
    flex: none;
    background: transparent;
    color: var(--muted);
  }

  @media (max-width: 820px) {
    .settings-page {
      padding: 1rem;
    }
    .grid {
      grid-template-columns: 1fr;
    }
  }
</style>
