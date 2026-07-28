/** Renditions supported by the convert API (must match backend transcode.KnownVariants). */
export const HLS_VARIANT_OPTIONS = [
  { id: "1440p", label: "1440p (2K)", height: 1440 },
  { id: "1080p", label: "1080p (Full HD)", height: 1080 },
  { id: "720p", label: "720p (HD)", height: 720 },
] as const;

export type HLSVariantId = (typeof HLS_VARIANT_OPTIONS)[number]["id"];

export type SourceProfile = {
  height?: number | null;
  bitrateBps?: number | null;
};

const SOURCE_HEIGHT_SLACK = 16;

export type VariantFitReason = "resolution" | null;

export function whyVariantBlocked(
  variantId: HLSVariantId,
  src: SourceProfile,
): VariantFitReason {
  const opt = HLS_VARIANT_OPTIONS.find((v) => v.id === variantId);
  if (!opt) return null;
  if (src.height && src.height > 0 && opt.height > src.height + SOURCE_HEIGHT_SLACK) {
    return "resolution";
  }
  return null;
}

export function variantFitsSource(variantId: HLSVariantId, src: SourceProfile): boolean {
  return whyVariantBlocked(variantId, src) === null;
}

export function variantsAvailableForSource(src: SourceProfile): HLSVariantId[] {
  const hasHeight = src.height != null && src.height > 0;
  if (!hasHeight) {
    return HLS_VARIANT_OPTIONS.map((v) => v.id);
  }
  return HLS_VARIANT_OPTIONS.filter((v) => variantFitsSource(v.id, src)).map((v) => v.id);
}

export function defaultVariantSelection(src: SourceProfile): HLSVariantId[] {
  const available = variantsAvailableForSource(src);
  return available.length > 0 ? available : ["720p"];
}

export function formatSourceBitrate(bps?: number | null): string | null {
  if (!bps || bps <= 0) return null;
  if (bps >= 1_000_000) {
    return `${(bps / 1_000_000).toFixed(1)} Mbps`;
  }
  return `${Math.round(bps / 1000)} kb/s`;
}
