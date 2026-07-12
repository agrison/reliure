import { mount } from "svelte";
import App from "./App.svelte";

// Apply the persisted theme before the app paints, to avoid a flash when the
// user forced a theme different from the OS. The Go settings remain the source
// of truth and re-sync on mount.
const saved = localStorage.getItem("theme");
if (saved === "light" || saved === "dark") {
  document.documentElement.dataset.theme = saved;
}
const savedLanguage = localStorage.getItem("language");
if (savedLanguage === "fr" || savedLanguage === "en" || savedLanguage === "de" || savedLanguage === "es" || savedLanguage === "it") {
  document.documentElement.lang = savedLanguage;
}

mount(App, { target: document.getElementById("app")! });
