/**
 * Album-art URL helpers. The backend exposes opaque `/art/<key>` paths and
 * accepts one of four resized variants through a `size` query parameter.
 */
const variantSizes = [64, 128, 240, 640] as const;

const variantForPixels = (pixels: number): number => {
  if (pixels <= 80) return variantSizes[0];
  if (pixels <= 160) return variantSizes[1];
  if (pixels <= 320) return variantSizes[2];
  return variantSizes[3];
};

/**
 * Requests the variant that covers the logical size at 2x device pixels. Covers
 * that are not backend artwork paths (remote search images or demo glyph names)
 * are returned unchanged.
 */
export function coverURL(cover: string, logicalSize: number): string {
  if (!cover.startsWith("/art/")) return cover;
  return `${cover}?size=${variantForPixels(logicalSize * 2)}`;
}

/** Small covers load lazily; larger, immediately visible art loads eagerly. */
export function coverLoading(logicalSize: number): "lazy" | "eager" {
  return logicalSize <= 40 ? "lazy" : "eager";
}
