import { env } from "@/lib/api/env";

type PublicKeyResponse = {
  algorithm: string;
  public_key: string;
};

let cachedKey: CryptoKey | null = null;
let cachedPEM: string | null = null;

/** Web Crypto chỉ có trên HTTPS hoặc localhost — IP LAN (http://192.168.x.x) sẽ false. */
export function canEncryptPassword(): boolean {
  if (typeof globalThis.crypto?.subtle !== "object") {
    return false;
  }
  if (typeof window !== "undefined" && window.isSecureContext === false) {
    return false;
  }
  return true;
}

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
  if (!canEncryptPassword()) {
    throw new Error(
      "Trình duyệt không hỗ trợ mã hóa mật khẩu trên HTTP qua IP LAN. Dùng https:// hoặc http://localhost, hoặc backend cho phép password plaintext (dev).",
    );
  }
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

/**
 * Dev/home lab: nếu không có secure context, gửi password plaintext
 * (backend chỉ chấp nhận khi REQUIRE_ENCRYPTED_PASSWORD=false).
 */
export async function passwordAuthFields(
  plain: string,
): Promise<{ encrypted_password: string } | { password: string }> {
  if (!canEncryptPassword()) {
    return { password: plain };
  }
  return { encrypted_password: await encryptPassword(plain) };
}

export function clearPasswordCryptoCache() {
  cachedKey = null;
  cachedPEM = null;
}
