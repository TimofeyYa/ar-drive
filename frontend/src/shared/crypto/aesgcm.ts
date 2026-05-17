// AES-GCM-256 для шифрования учётных данных в localStorage.
// Ключ деривируется PBKDF2-SHA256 (200 000 итераций) из парольной фразы +
// device-salt (хранится в localStorage как открытое значение).

const SALT_KEY = 'ar-drive.salt';
const ITERATIONS = 200_000;

function b64encode(buf: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(buf)));
}
function b64decode(s: string): Uint8Array {
  return Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
}

function getOrCreateSalt(): Uint8Array {
  const stored = localStorage.getItem(SALT_KEY);
  if (stored) return b64decode(stored);
  const salt = crypto.getRandomValues(new Uint8Array(16));
  localStorage.setItem(SALT_KEY, b64encode(salt.buffer));
  return salt;
}

async function deriveKey(passphrase: string): Promise<CryptoKey> {
  const enc = new TextEncoder();
  const baseKey = await crypto.subtle.importKey(
    'raw',
    enc.encode(passphrase || `${navigator.userAgent}::ar-drive-default`),
    'PBKDF2',
    false,
    ['deriveKey'],
  );
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt: getOrCreateSalt(), iterations: ITERATIONS, hash: 'SHA-256' },
    baseKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt'],
  );
}

export async function encryptJSON(value: unknown, passphrase: string): Promise<string> {
  const key = await deriveKey(passphrase);
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const data = new TextEncoder().encode(JSON.stringify(value));
  const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, data);
  // Формат: base64(iv) + "." + base64(ct)
  return `${b64encode(iv.buffer)}.${b64encode(ct)}`;
}

export async function decryptJSON<T>(payload: string, passphrase: string): Promise<T> {
  const [ivB64, ctB64] = payload.split('.');
  if (!ivB64 || !ctB64) throw new Error('invalid encrypted payload');
  const key = await deriveKey(passphrase);
  const pt = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: b64decode(ivB64) },
    key,
    b64decode(ctB64),
  );
  return JSON.parse(new TextDecoder().decode(pt)) as T;
}
