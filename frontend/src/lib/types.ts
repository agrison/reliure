import { t, type Locale } from "./i18n";

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

export function viewTitle(v: View, locale?: Locale): string {
  switch (v.kind) {
    case "all":
      return t("nav.allBooks", locale);
    case "dashboard":
      return t("nav.dashboard", locale);
    case "quickedit":
      return t("nav.quickEdit", locale);
    case "settings":
      return t("nav.settings", locale);
    case "gutenberg":
      return t("nav.discover", locale);
    case "annotations":
      return t("nav.annotations", locale);
    case "contentOccurrences":
      return t("content.occurrences.title", locale);
    case "shelves":
      return t("nav.shelves", locale);
    case "shelf":
      return v.name;
    case "reading":
      return t(v.status === "reading" ? "nav.reading" : v.status === "complete" ? "nav.complete" : "nav.abandoned", locale);
    case "author":
      return v.name;
    case "series":
      return v.name;
    case "tag":
      return v.name;
  }
}
