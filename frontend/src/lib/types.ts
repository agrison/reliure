// View is the current library selection driving the main list.
export type ReadingStatus = "reading" | "complete" | "abandoned";

export type View =
  | { kind: "all" }
  | { kind: "dashboard" }
  | { kind: "quickedit" }
  | { kind: "settings" }
  | { kind: "gutenberg" }
  | { kind: "annotations" }
  | { kind: "reading"; status: ReadingStatus }
  | { kind: "author"; id: number; name: string }
  | { kind: "series"; id: number; name: string }
  | { kind: "tag"; id: number; name: string };

export const readingStatusLabels: Record<ReadingStatus, string> = {
  reading: "En cours",
  complete: "Terminés",
  abandoned: "Abandonnés",
};

export function viewTitle(v: View): string {
  switch (v.kind) {
    case "all":
      return "Tous les livres";
    case "dashboard":
      return "Tableau de bord";
    case "quickedit":
      return "Édition rapide";
    case "settings":
      return "Réglages";
    case "gutenberg":
      return "Découvrir";
    case "annotations":
      return "Annotations";
    case "reading":
      return readingStatusLabels[v.status];
    case "author":
      return v.name;
    case "series":
      return v.name;
    case "tag":
      return v.name;
  }
}
