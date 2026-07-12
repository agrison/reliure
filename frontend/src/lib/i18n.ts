import { fr, type MessageKey } from "./i18n/fr";
import { en } from "./i18n/en";
import { de } from "./i18n/de";
import { es } from "./i18n/es";
import { it } from "./i18n/it";

export type Locale = "fr" | "en" | "de" | "es" | "it";

export const defaultLocale: Locale = "fr";

export const languageOptions: { value: Locale; label: string }[] = [
  { value: "fr", label: "Français" },
  { value: "en", label: "English" },
  { value: "de", label: "Deutsch" },
  { value: "es", label: "Español" },
  { value: "it", label: "Italiano" },
];

const dictionaries: Record<Locale, Partial<Record<MessageKey, string>>> = {
  fr,
  en,
  de,
  es,
  it,
};

let currentLocale: Locale = defaultLocale;

export function normalizeLocale(value: string | undefined | null): Locale {
  return languageOptions.some((l) => l.value === value) ? (value as Locale) : defaultLocale;
}

export function setDocumentLocale(locale: string | undefined | null) {
  const normalized = normalizeLocale(locale);
  currentLocale = normalized;
  document.documentElement.lang = normalized;
  try {
    localStorage.setItem("language", normalized);
  } catch {}
}

export type MessageParams = Record<string, string | number | boolean | null | undefined>;

export function plural(count: number): string {
  return count === 1 ? "" : "s";
}

export function t(
  key: MessageKey,
  locale: string | undefined | null = currentLocale,
  params?: MessageParams,
): string {
  const normalized = normalizeLocale(locale);
  let message = dictionaries[normalized][key] ?? fr[key];
  if (params) {
    for (const [name, value] of Object.entries(params)) {
      message = message.replaceAll(`{${name}}`, String(value ?? ""));
    }
  }
  return message;
}

export type { MessageKey };
