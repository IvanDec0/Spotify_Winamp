/**
 * Base catalog. Keys are the source of truth: `MessageKey` is derived from
 * this object, so a typo is a compile-time error.
 */
export const en = {
  "ui.close": "Close",
  "ui.cancel": "Cancel",
  "ui.dismiss": "Dismiss",
  "ui.retry": "Retry",
  "ui.search": "Search",
  "ui.loading": "Loading",
  "ui.notifications": "Notifications",
  "ui.empty": "Nothing here yet",
  "ui.items": "{count, plural, one {# item} other {# items}}",
} as const;

export type MessageKey = keyof typeof en;
