export type ToastKind = "info" | "error" | "success";

export interface ToastMessage {
  id: number;
  message: string;
  kind: ToastKind;
}

/**
 * In-memory toast queue. Each entry owns its timeout; dismissing an entry
 * clears the timer so a stale callback cannot remove a newer toast.
 */
export class ToastStore {
  toasts = $state<ToastMessage[]>([]);

  #timers: Record<number, ReturnType<typeof setTimeout>> = {};
  #nextId = 0;

  push(message: string, kind: ToastKind = "info", timeoutMs = 5000): number {
    const id = ++this.#nextId;
    this.toasts = [...this.toasts, { id, message, kind }];
    if (timeoutMs > 0) {
      this.#timers[id] = setTimeout(() => {
        delete this.#timers[id];
        this.dismiss(id);
      }, timeoutMs);
    }
    return id;
  }

  dismiss(id: number): void {
    const timer = this.#timers[id];
    if (timer !== undefined) {
      clearTimeout(timer);
      delete this.#timers[id];
    }
    this.toasts = this.toasts.filter((toast) => toast.id !== id);
  }

  clear(): void {
    for (const id of Object.keys(this.#timers)) {
      clearTimeout(this.#timers[Number(id)]);
    }
    this.#timers = {};
    this.toasts = [];
  }
}

export const toasts = new ToastStore();
