import { create } from 'zustand';
import { encryptJSON, decryptJSON } from '@shared/crypto/aesgcm';

export type Credentials = {
  clientId: string;
  clientSecret: string;
  projectId: string;
};

const STORAGE_KEY = 'ar-drive.creds.v1';

type State = {
  creds: Credentials | null;
  setCreds: (c: Credentials, passphrase: string) => Promise<void>;
  loadFromStorage: (passphrase: string) => Promise<boolean>;
  clear: () => void;
};

export const useCredentialsStore = create<State>((set) => ({
  creds: null,
  async setCreds(c, passphrase) {
    const enc = await encryptJSON(c, passphrase);
    localStorage.setItem(STORAGE_KEY, enc);
    set({ creds: c });
  },
  async loadFromStorage(passphrase) {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return false;
    try {
      const c = await decryptJSON<Credentials>(raw, passphrase);
      set({ creds: c });
      return true;
    } catch {
      return false;
    }
  },
  clear() {
    localStorage.removeItem(STORAGE_KEY);
    set({ creds: null });
  },
}));
