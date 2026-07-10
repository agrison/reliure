# Bibliothèque EPUB multiplateforme — Plan de sessions (Wails v3 + Go)

Application desktop (macOS d'abord, Windows/Linux ensuite) de gestion de bibliothèque EPUB avec envoi vers KOReader.
Backend 100% Go, UI web légère mais design pro et raffiné via webview système (Wails v3). Scope v1 : EPUB uniquement, architecture extensible.
Deux modes de transfert : OPDS (pull depuis KOReader) et protocole Calibre wireless (push depuis le poste).

## Choix techniques

- **Wails v3** (alpha mais API cœur stable ; multi-fenêtres, services, bindings générés statiquement). Fallback v2 possible si blocage, l'essentiel du code étant hors framework.
- **SQLite via `modernc.org/sqlite`** (pur Go, zéro cgo → cross-compilation triviale).
- **Frontend : Svelte + Vite** (template officiel Wails) — léger, réactif, peu de boilerplate. Vue/React possibles si préférence.
- **Séparation stricte** : tout le métier vit dans des packages Go réutilisables. Le binaire Wails n'est qu'un des points d'entrée — un `cmd/cli` headless (serveur OPDS pur) est quasi gratuit.
- **Qualité du code** : State of the art, on veut du code Go magnifique, reproductible, testé, extensible, du DRY et clean code, documenté là où ça fait sens (commentaires si besoin en anglais), une architecture expliquée dans ARCH.md et pour la DB dans DB.md
- **Rapidité** : On cherche une application blazing fast.

## Features

- Gestion de bibliothèque (import via fenetre standard et drag and drop, organisation, tags, auteurs, series etc etc, vue "bibliotheque", edition des metadatas, seul ou par lot)
- Envoi vers KOReader
- Nice to have v2: Connection à goodreads pour lier un livre distant à un local (on pourra imaginer rapprochement automatique via isbn / recherche web)

## Arborescence cible

```
bibliogo/
├── cmd/
│   ├── app/          # point d'entrée Wails
│   └── cli/          # (plus tard) mode headless/serveur
├── internal/
│   ├── core/         # modèles, DB, repositories, migrations
│   ├── formats/      # interface FormatHandler + registre
│   │   └── epub/     # parser EPUB
│   ├── library/      # import, organisation fichiers, déduplication, covers
│   ├── opds/         # serveur OPDS (net/http)
│   ├── calibre/      # protocole smart device (découverte UDP + session TCP)
│   └── hooks/        # scripting par scripts externes
└── frontend/         # Svelte
```

L'extensibilité formats : `type FormatHandler interface { CanHandle(path) bool; Metadata(path) (BookMetadata, error); Cover(path) ([]byte, error) }` + registre. Ajouter le PDF plus tard = un package `formats/pdf` de plus.

---

## ✅ Session 0 — Squelette & toolchain

- `wails3 init` avec template Svelte, vérifier `wails3 doctor`, build + dev loop OK sur le Mac.
- Mise en place de l'arborescence ci-dessus, packages vides avec interfaces.
- Premier binding Go→JS de test (méthode `Ping()`) pour valider la chaîne complète.
- CI GitHub Actions : build macOS (les cibles Windows/Linux attendront d'avoir accès à ces OS pour tester le webview).

**Livrable** : fenêtre qui affiche une donnée venant du Go.

## ✅ Session 1 — Core : modèles & SQLite

- Schéma : `book`, `author`, `series`, `book_author`, `tag`, `book_tag`, `file` (1 livre → N formats), FTS5 pour la recherche.
- Système de migrations maison simple (table `schema_version` + fichiers SQL embarqués via `embed`).
- Repositories avec tests (`go test`, DB en mémoire) : CRUD, recherche, listes par auteur/série.
- Emplacement DB : dossier de config OS (`os.UserConfigDir()`), dossier bibliothèque configurable séparément.

**Livrable** : package `core` testé, indépendant de Wails.

## ✅ Session 2 — Parser EPUB

- `formats/epub` : `archive/zip` + `encoding/xml`. Lire `META-INF/container.xml` → OPF → Dublin Core (titre, créateurs avec `role`/`file-as`, langue, description, identifiants, date).
- Meta Calibre : `calibre:series`, `calibre:series_index`, `calibre:title_sort` (tes EPUB existants en sont pleins).
- Couverture : `<meta name="cover">` → propriétés EPUB3 `cover-image` → heuristiques de secours. Génération de vignettes (`image` stdlib ou `disintegration/imaging`).
- **Tolérance maximale aux EPUB mal formés** : jamais d'échec bloquant, import dégradé avec le nom de fichier comme titre + log.

**Livrable** : corpus de tests avec EPUB réels (dont volontairement cassés), `go test` vert.

## ✅ Session 3 — Library : import & fichiers

- Pipeline d'import : détection format (registre) → métadonnées → déduplication (SHA-256 + heuristique titre/auteur) → copie vers `Bibliothèque/Auteur/Titre/` → insertion DB → vignette en cache.
- Import concurrent (worker pool, c'est du Go) avec canal de progression remonté à l'UI via les events Wails.
- Sources : sélecteur de fichiers natif (dialogs Wails) + scan récursif de dossier.

**Livrable** : importer ta bibliothèque Calibre complète, séries et auteurs corrects, progression visible.

## ✅ Session 4 — UI bibliothèque (Svelte)

- Layout : sidebar (Tous, Auteurs, Séries, Tags) + zone principale grille de couvertures / liste (toggle).
- Recherche instantanée (FTS5 côté Go, debounce côté front), tris.
- Vue détail : couverture, métadonnées, formats.
- Vignettes servies par l'asset handler Wails (pas de base64 dans le JSON).
- Drag & drop de fichiers sur la fenêtre → import.

**Livrable** : navigation fluide sur plusieurs centaines de livres.

## ✅ Session 5 — Édition des métadonnées

- Formulaire d'édition : titre, auteurs (multi + autocomplétion), série + numéro, tags, description, langue.
- Remplacement de couverture (fichier ou presse-papiers).
- Renommage des dossiers si titre/auteur changent (opération atomique : DB puis fichiers, rollback si échec).
- Édition par lot (assigner une série à une sélection).
- Option désactivée par défaut : réécriture de l'OPF dans l'EPUB (réécriture du zip).

Config global, dans la meme veine que Calibre on veut pouvoir définir globalement une chaine de caractère avec des variables qui seront remplacées, qui permet de definir le chemin distant quand le fichier sera envoyé sur KOReader.

Du genre `{series:||/}{series_index:0>2s|| }{title} - {authors}` mais imagineons un format moins obscure que les `:||` etc. 
Peut-être via possibilité de scripting aussi (par exemple `si ce livre appartient à une série alors voici le format, sinon ranger par auteur ou bien par genre`, enfin tu vois l'idée)
Le but étant que quand on envoie les fichiers directement dans l'arborescence souhaitée.

On poura **toujours** overrider dans les métadata attaché à un livre, si on veut pouvoir en ranger un spécifiquement autrement.

**Livrable** : gestion complète séries/auteurs, la DB reste la source de vérité.

## ✅ Session 6 — Serveur OPDS (pull)

- `net/http` pur dans `internal/opds` : racine → navigation (Récents, Auteurs, Séries, recherche OpenSearch) → entrées d'acquisition (`application/epub+zip`) + vignettes.
- Templates Atom via `text/template` ou construction `encoding/xml`.
- Annonce mDNS/Bonjour (`_opds._tcp`, lib `grandcat/zeroconf` ou équivalent maintenu) + affichage URL + QR code dans l'UI.
- Contrôles UI : on/off, port, état.
- Test réel depuis KOReader (ajout de catalogue OPDS).

**Livrable** : télécharger un livre depuis KOReader en WiFi.

## ✅ Session 7 — Protocole Calibre wireless (push)

Session exploratoire. Références : `calibre/src/calibre/devices/smart_device_app/driver.py` et le plugin KOReader `wireless.koplugin` (les deux côtés sont lisibles).

- Découverte : écoute des broadcasts UDP de KOReader sur les ports Calibre, réponse avec le port TCP.
- Session TCP : messages JSON préfixés par leur longueur, sous-ensemble d'opcodes nécessaire à l'envoi (`GET_INITIALIZATION_INFO`, `GET_DEVICE_INFORMATION`, `SEND_BOOK`, `NOOP`…). Go est idéal ici (`net`, `encoding/json`, goroutines).
- UI : badge "liseuse connectée", action "Envoyer vers la liseuse" sur la sélection, file d'envoi avec progression.
- Robustesse : timeouts, déconnexions, reprise.

**Livrable** : le workflow Calibre actuel reproduit — sélection, envoi, les livres arrivent sur KOReader.

## ✅ Session 8 — Inventaire liseuse `.reliure`

Objectif : garder un état fiable de ce que Reliure a déjà envoyé vers KOReader, sans dépendre d'un scan complet de la liseuse à chaque connexion.

- Lors d'un envoi via le protocole Calibre wireless, écrire ou mettre à jour un fichier `.reliure` à la racine logique gérée par Reliure sur la liseuse.
- Format JSON versionné, extensible, lisible humainement : version du schéma, date de génération, device id/nom si disponible, liste des fichiers transférés.
- Pour chaque entrée : `book_id` local, `file_id`, chemin distant, format, taille, SHA-256 si disponible, date d'envoi, titre/auteurs au moment de l'envoi.
- À la connexion Calibre wireless : charger l'inventaire connu pour cette liseuse (cache local synchronisé avec le `.reliure` envoyé), valider/tolérer les versions anciennes ou fichiers partiellement corrompus, puis exposer un état "sur la liseuse / absent" dans l'UI. Lecture directe du `.reliure` distant à envisager plus tard si le protocole KOReader expose un chemin fiable de lecture de fichier arbitraire.
- UI bibliothèque : badges ou filtres permettant de voir les livres déjà présents sur la liseuse, ceux absents, et les divergences.
- Robustesse : écriture atomique du `.reliure` côté device si le protocole le permet (temp + rename), sinon stratégie safe avec backup `.reliure.bak`.
- Confidentialité / portabilité : le fichier ne doit pas contenir de chemin local machine ni d'information inutilement personnelle.
- Tests : sérialisation/désérialisation, migration de version, matching local↔device, corruption partielle, doublons et chemins renommés.

**Livrable** : dès qu'une liseuse KOReader est connectée, Reliure affiche quels livres sont déjà présents et évite les renvois inutiles.

## ✅ Session 9 — Édition rapide en tableau

Objectif : modifier rapidement beaucoup de livres sans ouvrir chaque fiche une par une.

- Vue dédiée "Édition rapide" sous forme de tableau dense, pensée pour plusieurs centaines de lignes.
- Première colonne stable non éditable : ID Reliure, avec titre/couverture mini ou autre repère compact pour identifier le livre sans ambiguïté.
- Colonnes éditables : titre, title sort, auteurs, série, index de série, tags, langue, date, ISBN, éventuellement chemin KOReader override.
- Édition clavier efficace : tab/shift-tab, entrée, escape, copier/coller multi-cellules, sélection de plages, remplissage vers le bas si raisonnable.
- Validation locale par cellule : champs obligatoires, index de série numérique, tags/auteurs normalisés, erreurs visibles sans bloquer toute la grille.
- Sauvegarde par lot transactionnelle côté backend : prévisualiser le nombre de livres modifiés, appliquer les changements, rapporter les erreurs ligne par ligne.
- Détection des conflits : si un livre a changé depuis le chargement de la grille, prévenir et proposer de recharger ou forcer.
- Performance : virtualisation des lignes côté Svelte, appels Go groupés, pas de sauvegarde automatique implicite.
- Tests : mapping tableau→`BookUpdate`, validation, sauvegarde partielle/rollback selon la stratégie retenue, gros volume.

**Livrable** : une interface type tableur pour corriger rapidement une bibliothèque importée en masse.

## Session 10 — Hooks de scripting

- Événements : `post-import`, `pre-send`, `post-metadata-edit`.
- Scripts exécutables dans `<config>/hooks/<event>/`, contexte JSON sur stdin, métadonnées modifiées acceptées sur stdout (`os/exec`, timeout, sandbox minimal).
- Journal des exécutions dans l'UI.

**Livrable** : hook d'exemple normalisant `Nom, Prénom` → `Prénom Nom` à l'import.

## Session 11 — Multiplateforme & finitions

- Build et test Windows (WebView2) et Linux (WebKitGTK) : chemins (`filepath` partout, jamais de `/` codé en dur — à vérifier dès la session 1 en fait), dialogs, autostart.
- Packaging : `.app` signé/notarisé si distribution, NSIS/zip Windows, AppImage ou paquet Linux.
- Préférences : dossier bibliothèque, ports, options serveurs, hooks.
- Icône system tray (natif Wails v3) avec état des serveurs.

**Livrable** : binaires fonctionnels sur les trois OS.

---

## Extensions futures

- `formats/pdf` (métadonnées via `pdfcpu`).
- `cmd/cli` headless : le serveur OPDS sur un NAS/VPS, même codebase.
- Récupération de métadonnées en ligne (Google Books, BNF).
- Sync des statistiques/positions de lecture KOReader.

## Risques

1. **Wails v3 alpha** : API cœur stable mais tooling mouvant — épingler la version, lire le changelog avant chaque bump. Mitigation : 90% du code est dans `internal/`, indépendant du framework.
2. **EPUB mal formés** : le corpus de test de la session 2 reste l'investissement le plus rentable.
3. **Protocole Calibre** : non documenté officiellement ; l'OPDS couvre déjà le besoin si la session 7 déborde.
4. **Webview Linux/Windows** : comportements divergents (scroll, polices) — garder l'UI simple, tester tôt.
