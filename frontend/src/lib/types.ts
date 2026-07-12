import { t } from "./i18n";

// View is the current library selection driving the main list.
export type ReadingStatus = "reading" | "complete" | "abandoned";

export type View =
  | { kind: "all" }
  | { kind: "dashboard" }
  | { kind: "quickedit" }
  | { kind: "settings" }
  | { kind: "gutenberg" }
  | { kind: "annotations" }
  | { kind: "contentOccurrences"; query: string; scope: { kind: string; id: number }; title: string }
  | { kind: "shelves" }
  | { kind: "shelf"; id: number; name: string }
  | { kind: "reading"; status: ReadingStatus }
  | { kind: "author"; id: number; name: string }
  | { kind: "series"; id: number; name: string }
  | { kind: "tag"; id: number; name: string };

export const readingStatusLabels: Record<ReadingStatus, string> = {
  reading: t("nav.reading"),
  complete: t("nav.complete"),
  abandoned: t("nav.abandoned"),
};

export function viewTitle(v: View): string {
  switch (v.kind) {
    case "all":
      return t("nav.allBooks");
    case "dashboard":
      return t("nav.dashboard");
    case "quickedit":
      return t("nav.quickEdit");
    case "settings":
      return t("nav.settings");
    case "gutenberg":
      return t("nav.discover");
    case "annotations":
      return t("nav.annotations");
    case "contentOccurrences":
      return t("content.occurrences.title");
    case "shelves":
      return t("nav.shelves");
    case "shelf":
      return v.name;
    case "reading":
      return t(v.status === "reading" ? "nav.reading" : v.status === "complete" ? "nav.complete" : "nav.abandoned");
    case "author":
      return v.name;
    case "series":
      return v.name;
    case "tag":
      return v.name;
  }
}
