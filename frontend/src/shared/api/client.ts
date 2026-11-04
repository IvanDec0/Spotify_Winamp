export const UNKNOWN_ERROR_CODE = "unknown";

/**
 * Normalized failure at the API boundary. Every call made through `call`
 * rejects with an ApiError so views never branch on transport-specific shapes.
 */
export class ApiError extends Error {
  readonly code: string;
  override readonly cause?: unknown;

  constructor(message: string, code: string = UNKNOWN_ERROR_CODE, cause?: unknown) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.cause = cause;
  }
}

interface ErrorPayload {
  message?: unknown;
  code?: unknown;
}

function isErrorPayload(value: unknown): value is ErrorPayload {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

function readCode(value: unknown): string | null {
  return typeof value === "string" && value !== "" ? value : null;
}

/**
 * Converts any rejection value into an ApiError. Strings and `{message, code}`
 * payloads are preserved; unknown values fall back to a generic message.
 */
export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;

  if (typeof error === "string") {
    return new ApiError(error, UNKNOWN_ERROR_CODE, error);
  }

  if (error instanceof Error) {
    const code = readCode((error as Error & { code?: unknown }).code);
    return new ApiError(
      error.message || "The operation failed.",
      code ?? UNKNOWN_ERROR_CODE,
      error,
    );
  }

  if (isErrorPayload(error)) {
    const message = readString(error.message);
    const code = readCode(error.code);
    if (message || code) {
      return new ApiError(message ?? "The operation failed.", code ?? UNKNOWN_ERROR_CODE, error);
    }
  }

  return new ApiError("The operation failed.", UNKNOWN_ERROR_CODE, error);
}

/**
 * Runs a generated binding call and rethrows every failure as an ApiError.
 */
export async function call<T>(run: () => Promise<T>): Promise<T> {
  try {
    return await run();
  } catch (error) {
    throw toApiError(error);
  }
}
