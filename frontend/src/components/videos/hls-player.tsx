"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Hls, { type Level } from "hls.js";
import { NativeSelect } from "@/components/ui/native-select";
import { cn } from "@/lib/utils";

type QualityOption = {
  value: string;
  label: string;
};

function levelLabel(level: Level): string {
  const height = level.height;
  const bitrate =
    level.bitrate && level.bitrate > 0
      ? `${Math.round(level.bitrate / 1000)} kbps`
      : null;
  if (height) {
    return bitrate ? `${height}p · ${bitrate}` : `${height}p`;
  }
  return bitrate ?? "Không rõ";
}

function levelsToOptions(levels: Level[]): QualityOption[] {
  const indexed = levels.map((level, index) => ({ level, index }));
  indexed.sort((a, b) => (b.level.height ?? 0) - (a.level.height ?? 0));
  return [
    { value: "auto", label: "Tự động (ABR)" },
    ...indexed.map(({ level, index }) => ({
      value: String(index),
      label: levelLabel(level),
    })),
  ];
}

/** Path-only key — token/exp in query must not remount the player on refetch. */
function playbackPathKey(src: string): string {
  try {
    return new URL(src).pathname;
  } catch {
    return src.split("?")[0] ?? src;
  }
}

export function HlsPlayer({ src, className }: { src: string; className?: string }) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const hlsRef = useRef<Hls | null>(null);
  const srcRef = useRef(src);
  srcRef.current = src;
  const [qualityOptions, setQualityOptions] = useState<QualityOption[]>([]);
  const [selectedQuality, setSelectedQuality] = useState("auto");
  const [nativeOnly, setNativeOnly] = useState(false);
  const srcKey = playbackPathKey(src);

  const applyQuality = useCallback((value: string) => {
    const hls = hlsRef.current;
    if (!hls) return;
    if (value === "auto") {
      hls.currentLevel = -1;
    } else {
      const idx = parseInt(value, 10);
      if (!Number.isNaN(idx)) {
        hls.currentLevel = idx;
      }
    }
    setSelectedQuality(value);
  }, []);

  useEffect(() => {
    const video = videoRef.current;
    const loadSrc = srcRef.current;
    if (!video || !loadSrc) return;

    setQualityOptions([]);
    setSelectedQuality("auto");
    setNativeOnly(false);

    const releaseVideo = () => {
      hlsRef.current = null;
      video.pause();
      video.removeAttribute("src");
      video.load();
    };

    if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        xhrSetup(xhr) {
          xhr.withCredentials = true;
        },
        maxBufferLength: 30,
        maxMaxBufferLength: 60,
      });
      hlsRef.current = hls;

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        setQualityOptions(levelsToOptions(hls.levels));
        setSelectedQuality("auto");
      });

      hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
        if (hls.autoLevelEnabled) {
          setSelectedQuality("auto");
        } else {
          setSelectedQuality(String(data.level));
        }
      });

      hls.loadSource(loadSrc);
      hls.attachMedia(video);
      return () => {
        hls.destroy();
        releaseVideo();
      };
    }

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = loadSrc;
      setNativeOnly(true);
      return releaseVideo;
    }

    return undefined;
  }, [srcKey]);

  const showQualityPicker = qualityOptions.length > 2;

  return (
    <div className="relative w-full">
      {showQualityPicker && (
        <div className="absolute right-2 top-2 z-10">
          <NativeSelect
            className="h-8 w-auto min-w-[148px] border-white/25 bg-black/75 text-xs text-white shadow-sm backdrop-blur-sm"
            value={selectedQuality}
            onChange={(e) => applyQuality(e.target.value)}
            aria-label="Chọn độ phân giải"
          >
            {qualityOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </NativeSelect>
        </div>
      )}
      {nativeOnly && (
        <p className="absolute right-2 top-2 z-10 rounded-md bg-black/70 px-2 py-1 text-xs text-white/80">
          Trình duyệt này phát HLS native — không chọn chất lượng thủ công.
        </p>
      )}
      <video
        ref={videoRef}
        controls
        playsInline
        crossOrigin="use-credentials"
        className={cn("aspect-video w-full rounded-lg bg-black", className)}
      />
    </div>
  );
}
