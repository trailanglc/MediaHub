import {
  abortUpload,
  completeUpload,
  initUpload,
  uploadChunk,
} from "@/lib/api/api-client";

export type ChunkedUploadProgress = {
  loaded: number;
  total: number;
  percent: number;
};

export async function uploadFileChunked(
  file: File,
  parentId: string,
  onProgress?: (p: ChunkedUploadProgress) => void,
  signal?: AbortSignal,
): Promise<string> {
  const init = await initUpload({
    parent_id: parentId,
    file_name: file.name,
    size: file.size,
    mime_type: file.type || "application/octet-stream",
  });

  const sessionId = init.session_public_id;
  const chunkSize = init.chunk_size;
  let offset = 0;
  let index = 0;

  try {
    while (offset < file.size) {
      if (signal?.aborted) {
        await abortUpload(sessionId);
        throw new DOMException("Upload aborted", "AbortError");
      }
      const end = Math.min(offset + chunkSize, file.size);
      const chunk = file.slice(offset, end);
      await uploadChunk(sessionId, index, chunk);
      offset = end;
      index += 1;
      onProgress?.({
        loaded: offset,
        total: file.size,
        percent: Math.round((offset / file.size) * 100),
      });
    }
    const obj = await completeUpload(sessionId);
    return obj.public_id;
  } catch (err) {
    try {
      await abortUpload(sessionId);
    } catch {
      /* ignore cleanup errors */
    }
    throw err;
  }
}
