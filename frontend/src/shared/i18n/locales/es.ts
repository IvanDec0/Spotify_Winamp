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
};
