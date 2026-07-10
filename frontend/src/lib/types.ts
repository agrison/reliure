// View is the current library selection driving the main list.
export type View =
  | { kind: "all" }
  | { kind: "author"; id: number; name: string }
  | { kind: "series"; id: number; name: string }
  | { kind: "tag"; id: number; name: string };

export function viewTitle(v: View): string {
  switch (v.kind) {
    case "all":
      return "Tous les livres";
    case "author":
      return v.name;
    case "series":
      return v.name;
    case "tag":
      return v.name;
  }
}
