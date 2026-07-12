<script lang="ts">
  import type { AppSettings, OPDSStatus, CalibreStatus } from "./api";
  import { languageOptions, t, type Locale } from "./i18n";

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
    onChooseWatchFolder,
    onClearWatchFolder,
    onSetWatchFolderEnabled,
    onSetWatchFolderDelay,
    onSetWatchFolderDelete,
    onSetContentSearchEnabled,
    onSetContentSearchContext,
    onReindexContent,
    onRegenerateCovers,
    onSetTheme,
    onSetLanguage,
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
    onChooseWatchFolder: () => Promise<void> | void;
    onClearWatchFolder: () => Promise<void> | void;
    onSetWatchFolderEnabled: (enabled: boolean) => Promise<void> | void;
    onSetWatchFolderDelay: (seconds: number) => Promise<void> | void;
    onSetWatchFolderDelete: (enabled: boolean) => Promise<void> | void;
    onSetContentSearchEnabled: (enabled: boolean) => Promise<void> | void;
    onSetContentSearchContext: (mode: "minimal" | "phrase" | "paragraph") => Promise<void> | void;
    onReindexContent: () => Promise<void> | void;
    onRegenerateCovers: () => Promise<void> | void;
    onSetTheme: (theme: "system" | "light" | "dark") => void;
    onSetLanguage: (language: Locale) => void;
    onChooseKoreader: () => Promise<void> | void;
    onSyncKoreader: () => Promise<void> | void;
    onSyncKoreaderFromDevice: () => Promise<void> | void;
    syncingKoreader: boolean;
  } = $props();

  const themes: { value: "system" | "light" | "dark"; labelKey: Parameters<typeof t>[0] }[] = [
    { value: "system", labelKey: "settings.theme.system" },
    { value: "light", labelKey: "settings.theme.light" },
    { value: "dark", labelKey: "settings.theme.dark" },
  ];
  const contentContexts: { value: "minimal" | "phrase" | "paragraph"; labelKey: Parameters<typeof t>[0] }[] = [
    { value: "minimal", labelKey: "settings.content.context.minimal" },
    { value: "phrase", labelKey: "settings.content.context.phrase" },
    { value: "paragraph", labelKey: "settings.content.context.paragraph" },
  ];

  let regenerating = $state(false);
  let reindexingContent = $state(false);
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

  async function reindexContent() {
    if (reindexingContent) return;
    reindexingContent = true;
    try {
      await onReindexContent();
    } finally {
      reindexingContent = false;
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
        <h2>{t("settings.network.title", settings.language)}</h2>
        <p>{t("settings.network.description", settings.language)}</p>
      </div>
    </div>

    <div class="grid">
      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>OPDS</h3>
            <p>{t("settings.opds.description", settings.language)}</p>
          </div>
          <span class="status" class:on={opdsStatus.running}>{opdsStatus.running ? t("common.online", settings.language) : t("common.offline", settings.language)}</span>
        </div>

        <label class="toggle">
          <input
            type="checkbox"
            checked={opdsStatus.enabled}
            onchange={(e) => onSetOPDSEnabled((e.target as HTMLInputElement).checked)}
          />
          <span>{t("settings.opds.enable", settings.language)}</span>
        </label>

        <div class="field-row">
          <label for="opds-port">{t("settings.port", settings.language)}</label>
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
            <h3>{t("settings.reader.title", settings.language)}</h3>
            <p>{t("settings.reader.description", settings.language)}</p>
          </div>
          <span class="status" class:on={calibre?.connected ?? false}>
            {calibre?.connected ? t("status.connected.lower", settings.language) : calibre?.running ? t("status.waiting.lower", settings.language) : t("status.stopped.lower", settings.language)}
          </span>
        </div>

        <label class="toggle">
          <input
            type="checkbox"
            checked={calibre?.running ?? false}
            onchange={(e) => onSetCalibreEnabled((e.target as HTMLInputElement).checked)}
          />
          <span>{t("settings.reader.enable", settings.language)}</span>
        </label>

        {#if calibre?.running}
          <p class="hint">{t("settings.reader.koreaderHint", settings.language)}</p>
          <div class="path-row">
            <span class="path ellipsis" title={calibre.address}>{calibre.address || `port ${calibre.port}`}</span>
          </div>
          {#if calibre.connected && calibre.device}
            <p class="hint">{t("settings.reader.connectedDevice", settings.language, { device: calibre.device })}</p>
          {/if}
        {/if}
      </article>
    </div>
  </section>

  <section class="section">
    <div class="section-head">
      <div>
        <h2>{t("settings.library.title", settings.language)}</h2>
        <p>{t("settings.library.description", settings.language)}</p>
      </div>
    </div>

    <div class="grid">
      <article class="panel wide">
        <div class="panel-head">
          <div>
            <h3>{t("settings.import.title", settings.language)}</h3>
            <p>{t("settings.import.description", settings.language)}</p>
          </div>
        </div>

        <div class="seg" role="radiogroup" aria-label={t("settings.import.mode", settings.language)}>
          <button
            class="seg-btn"
            class:active={settings.importMode === "copy"}
            aria-pressed={settings.importMode === "copy"}
            onclick={() => onSetMode("copy")}
          >
            {t("settings.import.copy", settings.language)}
          </button>
          <button
            class="seg-btn"
            class:active={settings.importMode === "reference"}
            aria-pressed={settings.importMode === "reference"}
            onclick={() => onSetMode("reference")}
          >
            {t("settings.import.reference", settings.language)}
          </button>
        </div>

        {#if settings.importMode === "copy"}
          <div class="path-row">
            <span class="path ellipsis" title={settings.libraryDir}>{settings.libraryDir}</span>
            <button class="link" onclick={onChooseFolder}>{t("settings.change", settings.language)}</button>
          </div>
        {:else}
          <p class="hint">{t("settings.import.referenceHint", settings.language)}</p>
        {/if}
      </article>

      <article class="panel wide">
        <div class="panel-head">
          <div>
            <h3>{t("settings.watch.title", settings.language)}</h3>
            <p>{t("settings.watch.description", settings.language)}</p>
          </div>
          <span class="status" class:on={settings.watchFolderEnabled && !!settings.watchFolderDir}>
            {settings.watchFolderEnabled && settings.watchFolderDir ? t("common.active", settings.language) : t("common.stopped", settings.language)}
          </span>
        </div>

        {#if settings.watchFolderDir}
          <div class="path-row">
            <span class="path ellipsis" title={settings.watchFolderDir}>{settings.watchFolderDir}</span>
            <button class="link" onclick={onChooseWatchFolder}>{t("settings.change", settings.language)}</button>
            <button class="link danger-link" onclick={onClearWatchFolder}>{t("settings.remove", settings.language)}</button>
          </div>
        {:else}
          <button class="action" onclick={onChooseWatchFolder}>{t("settings.chooseFolder", settings.language)}</button>
        {/if}

        <div class="watch-options">
          <label class="toggle">
            <input
              type="checkbox"
              checked={settings.watchFolderEnabled}
              disabled={!settings.watchFolderDir}
              onchange={(e) => onSetWatchFolderEnabled((e.target as HTMLInputElement).checked)}
            />
            <span>{t("settings.watch.enable", settings.language)}</span>
          </label>

          <label class="delay-field">
            <span>{t("settings.watch.delay", settings.language)}</span>
            <input
              type="number"
              min="1"
              max="3600"
              value={settings.watchFolderDelaySeconds}
              onblur={(e) => onSetWatchFolderDelay(Number.parseInt((e.target as HTMLInputElement).value, 10))}
              onkeydown={(e) => {
                if (e.key === "Enter") onSetWatchFolderDelay(Number.parseInt((e.target as HTMLInputElement).value, 10));
              }}
            />
            <span>{t("common.seconds", settings.language)}</span>
          </label>

          <label class="toggle">
            <input
              type="checkbox"
              checked={settings.watchFolderDeleteSource}
              disabled={settings.importMode !== "copy"}
              onchange={(e) => onSetWatchFolderDelete((e.target as HTMLInputElement).checked)}
            />
            <span>{t("settings.watch.deleteSource", settings.language)}</span>
          </label>
        </div>
        {#if settings.importMode !== "copy"}
          <p class="hint">{t("settings.watch.deleteDisabled", settings.language)}</p>
        {/if}
      </article>

      <article class="panel wide">
        <div class="panel-head">
          <div>
            <h3>{t("settings.remotePath.title", settings.language)}</h3>
            <p>{t("settings.remotePath.description", settings.language)}</p>
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
        <p class="hint">{t("settings.remotePath.variables", settings.language)}</p>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>{t("settings.epubMetadata.title", settings.language)}</h3>
            <p>{t("settings.epubMetadata.description", settings.language)}</p>
          </div>
        </div>

        <label class="toggle">
          <input
            type="checkbox"
            checked={settings.writeMetadataToFile}
            onchange={(e) => onSetWriteMetadataToFile((e.target as HTMLInputElement).checked)}
          />
          <span>{t("settings.epubMetadata.write", settings.language)}</span>
        </label>
        <p class="hint">{t("settings.epubMetadata.warning", settings.language)}</p>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>{t("settings.covers.title", settings.language)}</h3>
            <p>{t("settings.covers.description", settings.language)}</p>
          </div>
        </div>

        <button class="action" onclick={regenerate} disabled={regenerating}>
          {regenerating ? t("settings.covers.generating", settings.language) : t("settings.covers.regenerate", settings.language)}
        </button>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>{t("settings.content.title", settings.language)}</h3>
            <p>{t("settings.content.description", settings.language)}</p>
          </div>
        </div>

        <label class="toggle">
          <input
            type="checkbox"
            checked={settings.contentSearchEnabled}
            onchange={(e) => onSetContentSearchEnabled((e.target as HTMLInputElement).checked)}
          />
          <span>{t("settings.content.enable", settings.language)}</span>
        </label>
        <div class="seg context" role="radiogroup" aria-label={t("settings.content.context", settings.language)}>
          {#each contentContexts as c}
            <button
              class="seg-btn"
              class:active={(settings.contentSearchContext || "minimal") === c.value}
              aria-pressed={(settings.contentSearchContext || "minimal") === c.value}
              onclick={() => onSetContentSearchContext(c.value)}
              disabled={!settings.contentSearchEnabled}
            >
              {t(c.labelKey, settings.language)}
            </button>
          {/each}
        </div>
        <button class="action secondary-action" onclick={reindexContent} disabled={!settings.contentSearchEnabled || reindexingContent}>
          {reindexingContent ? t("settings.content.indexing", settings.language) : t("settings.content.reindex", settings.language)}
        </button>
        <p class="hint">{t("settings.content.hint", settings.language)}</p>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>{t("settings.koreader.title", settings.language)}</h3>
            <p>{t("settings.koreader.description", settings.language)}</p>
          </div>
        </div>

        <div class="subhead">{t("settings.koreader.fromWifi", settings.language)}</div>
        {#if calibre?.connected}
          <button class="action" onclick={onSyncKoreaderFromDevice} disabled={syncingKoreader}>
            {syncingKoreader ? t("settings.koreader.syncing", settings.language) : t("settings.koreader.syncFrom", settings.language, { device: calibre.device || t("settings.reader.title", settings.language).toLowerCase() })}
          </button>
        {:else}
          <button class="action" disabled>{t("settings.koreader.noReader", settings.language)}</button>
          <p class="hint">{t("settings.koreader.noReaderHint", settings.language)}</p>
        {/if}

        <div class="subhead">{t("settings.koreader.fromFolder", settings.language)}</div>
        {#if settings.koreaderSyncDir}
          <div class="path-row"><span class="path ellipsis" title={settings.koreaderSyncDir}>{settings.koreaderSyncDir}</span></div>
        {/if}
        <div class="btnrow">
          <button class="action" onclick={onChooseKoreader} disabled={syncingKoreader}>
            {syncingKoreader ? t("settings.koreader.syncing", settings.language) : settings.koreaderSyncDir ? t("settings.koreader.changeFolder", settings.language) : t("settings.chooseFolder", settings.language)}
          </button>
          {#if settings.koreaderSyncDir}
            <button class="action ghost" onclick={onSyncKoreader} disabled={syncingKoreader}>
              {syncingKoreader ? "..." : t("settings.koreader.resync", settings.language)}
            </button>
          {/if}
        </div>
      </article>
    </div>
  </section>

  <section class="section">
    <div class="section-head">
      <div>
        <h2>{t("settings.features.title", settings.language)}</h2>
        <p>{t("settings.features.description", settings.language)}</p>
      </div>
    </div>

    <div class="grid">
      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>{t("settings.features.discover.title", settings.language)}</h3>
            <p>{t("settings.features.discover.description", settings.language)}</p>
          </div>
        </div>
        <label class="toggle">
          <input
            type="checkbox"
            checked={settings.featureDiscover}
            onchange={(e) => onSetFeatureDiscover((e.target as HTMLInputElement).checked)}
          />
          <span>{t("settings.features.discover.show", settings.language)}</span>
        </label>
      </article>

      <article class="panel">
        <div class="panel-head">
          <div>
            <h3>{t("settings.features.shelves.title", settings.language)}</h3>
            <p>{t("settings.features.shelves.description", settings.language)}</p>
          </div>
        </div>
        <label class="toggle">
          <input
            type="checkbox"
            checked={settings.featureSmartShelves}
            onchange={(e) => onSetFeatureSmartShelves((e.target as HTMLInputElement).checked)}
          />
          <span>{t("settings.features.shelves.show", settings.language)}</span>
        </label>
      </article>
    </div>
  </section>

  <section class="section compact">
    <div class="section-head">
      <div>
        <h2>{t("settings.appearance.title", settings.language)}</h2>
        <p>{t("settings.appearance.description", settings.language)}</p>
      </div>
    </div>
    <div class="field-label">{t("settings.theme.label", settings.language)}</div>
    <div class="seg appearance" role="radiogroup" aria-label={t("settings.theme.label", settings.language)}>
      {#each themes as theme}
        <button
          class="seg-btn"
          class:active={(settings.theme || "system") === theme.value}
          aria-pressed={(settings.theme || "system") === theme.value}
          onclick={() => onSetTheme(theme.value)}
        >
          {t(theme.labelKey, settings.language)}
        </button>
      {/each}
    </div>
    <label class="select-field">
      <span>{t("settings.language.label", settings.language)}</span>
      <select value={settings.language || "fr"} onchange={(e) => onSetLanguage((e.target as HTMLSelectElement).value as Locale)}>
        {#each languageOptions as lang}
          <option value={lang.value}>{lang.label}</option>
        {/each}
      </select>
    </label>
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
  .danger-link {
    color: var(--danger);
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
  .field-label,
  .select-field span {
    display: block;
    margin-bottom: 0.45rem;
    color: var(--muted);
    font-size: 0.78rem;
    font-weight: 600;
  }
  .select-field {
    display: block;
    margin-top: 1rem;
  }
  .select-field select {
    width: min(100%, 360px);
  }
  .seg.context {
    margin-top: 0.85rem;
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
  .secondary-action {
    margin-top: 0.85rem;
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
  .watch-options {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    gap: 0.85rem;
    align-items: center;
    margin-top: 0.9rem;
  }
  .delay-field {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    color: var(--muted);
    font-size: 0.82rem;
  }
  .delay-field input {
    width: 5rem;
    padding: 0.4rem 0.5rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--inset);
    color: var(--text);
    font: inherit;
    font-size: 0.84rem;
    outline: none;
  }
  .delay-field input:focus {
    border-color: var(--border-hi);
    background: var(--surface-hi);
  }

  @media (max-width: 820px) {
    .settings-page {
      padding: 1rem;
    }
    .grid {
      grid-template-columns: 1fr;
    }
    .watch-options {
      grid-template-columns: 1fr;
      align-items: flex-start;
    }
  }
</style>
