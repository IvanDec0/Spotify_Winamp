import type { MessageKey } from "./en";

/**
 * Spanish catalog. `Record<MessageKey, string>` makes missing or unknown keys
 * a TypeScript error, so both catalogs stay in parity without a generator.
 */
export const es: Record<MessageKey, string> = {
  "ui.close": "Cerrar",
  "ui.cancel": "Cancelar",
  "ui.dismiss": "Descartar",
  "ui.retry": "Reintentar",
  "ui.search": "Buscar",
  "ui.loading": "Cargando",
  "ui.notifications": "Notificaciones",
  "ui.empty": "Todavía no hay nada aquí",
  "ui.items": "{count, plural, one {# elemento} other {# elementos}}",
  "browse.open": "Explorar",
  "browse.navLabel": "Secciones de exploración",
  "browse.search": "Buscar",
  "browse.library": "Biblioteca",
  "browse.podcasts": "Pódcasts",
  "browse.settings": "Configuración",
  "browse.searchTitle": "Buscar en el catálogo",
  "browse.searchBody":
    "Encuentra canciones, álbumes, artistas, listas, programas y episodios. La búsqueda llega en la próxima actualización.",
  "browse.libraryTitle": "Tu biblioteca",
  "browse.libraryBody":
    "Aquí estarán tus listas, álbumes guardados, artistas seguidos, programas y Tus episodios.",
  "browse.podcastsTitle": "Pódcasts",
  "browse.podcastsBody": "Sigue programas y escucha Tus episodios desde este panel.",
  "browse.settingsTitle": "Configuración",
  "browse.settingsBody":
    "Los controles de reproducción, caché y apariencia llegarán a la ventana Explorar.",
  "browse.welcomeBody":
    "Esta ventana mantiene el catálogo a mano sin tocar el reproductor compacto.",
  "browse.catalogStatus": "Catálogo de Spotify",
  "browse.skinTheme": "{skin} · {theme}",
  "browse.nowPlaying": "Reproduciendo",
  "browse.nothingPlaying": "Nada en reproducción",
  "browse.play": "Reproducir",
  "browse.pause": "Pausar",
  "browse.previous": "Anterior",
  "browse.next": "Siguiente",
  "browse.volume": "Volumen",
  "browse.progress": "Posición de reproducción",
};
