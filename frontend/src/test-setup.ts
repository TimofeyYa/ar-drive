import '@testing-library/jest-dom';

// jsdom не имеет WebCrypto — подключаем нодовый.
import { webcrypto } from 'node:crypto';
if (!globalThis.crypto) {
  // @ts-expect-error — глобальный полифилл
  globalThis.crypto = webcrypto;
}
