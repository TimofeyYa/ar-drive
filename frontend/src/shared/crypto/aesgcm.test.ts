import { describe, it, expect } from 'vitest';
import { encryptJSON, decryptJSON } from './aesgcm';

describe('aesgcm round-trip', () => {
  it('encrypts and decrypts JSON', async () => {
    const value = { client_id: 'k', secret: 's', project: 'p' };
    const enc = await encryptJSON(value, 'pass');
    expect(enc).toContain('.');
    const dec = await decryptJSON<typeof value>(enc, 'pass');
    expect(dec).toEqual(value);
  });

  it('fails on wrong passphrase', async () => {
    const enc = await encryptJSON({ a: 1 }, 'pass');
    await expect(decryptJSON(enc, 'wrong')).rejects.toThrow();
  });
});
