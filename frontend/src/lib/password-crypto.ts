import { env } from "@/lib/env";

type PublicKeyResponse = {
  algorithm: string;
  public_key: string;
};

let cachedKey: CryptoKey | null = null;
let cachedPEM: string | null = null;

function pemToArrayBuffer(pem: string): ArrayBuffer {
  const b64 = pem
    .replace(/-----BEGIN PUBLIC KEY-----/, "")
    .replace(/-----END PUBLIC KEY-----/, "")
    .replace(/\s/g, "");
  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

async function importPublicKey(pem: string): Promise<CryptoKey> {
  return crypto.subtle.importKey(
    "spki",
    pemToArrayBuffer(pem),
    { name: "RSA-OAEP", hash: "SHA-256" },
    false,
    ["encrypt"],
  );
}

async function loadPublicKey(): Promise<CryptoKey> {
  const res = await fetch(`${env.NEXT_PUBLIC_API_URL}/api/auth/crypto/public-key`, {
    credentials: "include",
  });
  if (!res.ok) {
    throw new Error("Không tải được khóa mã hóa");
  }
  const data = (await res.json()) as PublicKeyResponse;
  if (data.algorithm !== "RSA-OAEP-256") {
    throw new Error("Thuật toán mã hóa không được hỗ trợ");
  }
  if (cachedPEM === data.public_key && cachedKey) {
    return cachedKey;
  }
  cachedPEM = data.public_key;
  cachedKey = await importPublicKey(data.public_key);
  return cachedKey;
}

/** Mã hóa mật khẩu bằng RSA-OAEP (SHA-256) trước khi gửi API. */
export async function encryptPassword(plain: string): Promise<string> {
  const key = await loadPublicKey();
  const encoded = new TextEncoder().encode(plain);
  const ciphertext = await crypto.subtle.encrypt(
    { name: "RSA-OAEP" },
    key,
    encoded,
  );
  const bytes = new Uint8Array(ciphertext);
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

export function clearPasswordCryptoCache() {
  cachedKey = null;
  cachedPEM = null;
}
