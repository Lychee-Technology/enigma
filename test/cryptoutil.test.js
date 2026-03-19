// cryptoutil.test.js
import { beforeAll, expect, jest } from '@jest/globals';
import { webcrypto } from 'node:crypto';
import { TextEncoder, TextDecoder } from 'util';
import { CompressionStream, DecompressionStream } from 'stream/web';
import { Blob, Buffer } from 'node:buffer';

import {
    bytesToBase64DataUrl,
    base64Encode,
    base64Decode,
    passwordToCryptoParams,
    encrypt,
    decrypt,
    compress,
    decompress,
} from '../public/js/cryptoutil.mjs';

const BIN_MIME = "application/octet-stream";
const DATA_URL_PREFIX = `data:${BIN_MIME};base64,`;

describe('Cryptoutil', () => {
    beforeAll(() => {
        Object.defineProperty(globalThis, 'crypto', {
            value: webcrypto,
        });

        Object.assign(globalThis, {
            TextDecoder,
            TextEncoder,
            CompressionStream,
            DecompressionStream,
            Blob,
        });

        // Mock FileReader for base64Encode (bytesToBase64DataUrl still uses FileReader).
        class MockFileReader {
            constructor() {
                this.onload = null;
                this.onerror = null;
                this.result = null;
            }
            readAsDataURL(blob) {
                blob.arrayBuffer().then((buffer) => {
                    const base64String = Buffer.from(buffer).toString('base64');
                    this.result = `${DATA_URL_PREFIX}${base64String}`;
                    if (this.onload) {
                        this.onload({ target: this });
                    }
                }).catch((error) => {
                    if (this.onerror) {
                        this.onerror(error);
                    }
                });
            }
        }

        // Node.js 18+ has atob/btoa built-in; ensure they're available.
        if (typeof globalThis.atob === 'undefined') {
            globalThis.atob = (b64) => Buffer.from(b64, 'base64').toString('binary');
            globalThis.btoa = (bin) => Buffer.from(bin, 'binary').toString('base64');
        }

        global.FileReader = MockFileReader;
    });

    beforeEach(() => {
        jest.clearAllMocks();
    });

    describe('bytesToBase64DataUrl', () => {
        it('should convert bytes to a base64 data URL', async () => {
            const bytes = new TextEncoder().encode("abc").buffer;
            const result = await bytesToBase64DataUrl(bytes);
            expect(result).toBe("data:application/octet-stream;base64,YWJj");
        });
    });

    describe('base64Encode', () => {
        it('should encode bytes to base64 string', async () => {
            const bytes = new TextEncoder().encode("abc");
            const result = await base64Encode(bytes);
            expect(result).toBe("YWJj");
        });
    });

    describe('base64Decode', () => {
        it('should decode base64 string to ArrayBuffer', () => {
            const result = base64Decode("YWJj");
            expect(new TextDecoder().decode(result)).toBe('abc');
        });

        it('should round-trip with base64Encode', async () => {
            const original = new TextEncoder().encode("hello world");
            const encoded = await base64Encode(original);
            const decoded = base64Decode(encoded);
            expect(Array.from(new Uint8Array(decoded))).toEqual(Array.from(original));
        });
    });

    describe('passwordToCryptoParams', () => {
        it('should return a cookie derived from the password', async () => {
            const result = await passwordToCryptoParams("testPassword");
            expect(result).toHaveProperty('cookie');
            expect(typeof result.cookie).toBe('string');
            expect(result.cookie.length).toBe(8); // 4 bytes = 8 hex chars
        });

        it('should return the same cookie for the same password', async () => {
            const r1 = await passwordToCryptoParams("same-password");
            const r2 = await passwordToCryptoParams("same-password");
            expect(r1.cookie).toBe(r2.cookie);
        });

        it('should return different cookies for different passwords', async () => {
            const r1 = await passwordToCryptoParams("password-A");
            const r2 = await passwordToCryptoParams("password-B");
            expect(r1.cookie).not.toBe(r2.cookie);
        });
    });

    describe('compress and decompress', () => {
        it('roundtrip', async () => {
            let data = "test data";
            for (let i = 0; i < 12; i++) {
                data += data;
            }
            const compressed = await compress(data);
            const decompressed = await decompress(compressed);
            expect(compressed).toBeInstanceOf(Uint8Array);
            expect(compressed.length).toBeLessThan(decompressed.length);
            expect(decompressed).toBe(data);
        });
    });
});

describe("cryptoutil functions", () => {
    test("compress and decompress roundtrip", async () => {
        if (
            typeof CompressionStream === "function" &&
            typeof DecompressionStream === "function"
        ) {
            const original = "The quick brown fox jumps over the lazy dog";
            const compressed = await compress(original);
            const decompressed = await decompress(compressed);
            expect(decompressed).toBe(original);
        } else {
            expect(true).toBe(true);
        }
    });

    test("encrypt and decrypt roundtrip", async () => {
        const message = "Hello, world!";
        const password = "correcthorsebatterystaple";

        const encryptedResult = await encrypt(message, password);
        expect(encryptedResult).toHaveProperty("encrypted");
        expect(encryptedResult.encrypted.length).toBeGreaterThan(0);

        const decryptedMessage = await decrypt(encryptedResult.encrypted, password);
        expect(decryptedMessage).toBe(message);
    });

    test("encrypt produces different ciphertext each time (random IV+salt)", async () => {
        const message = "same message";
        const password = "same password";

        const r1 = await encrypt(message, password);
        const r2 = await encrypt(message, password);
        expect(r1.encrypted).not.toBe(r2.encrypted);
    });

    // Issue #19: error path tests
    describe("error cases", () => {
        test("decrypt with wrong password should throw", async () => {
            const { encrypted } = await encrypt("secret message", "correct-password");
            await expect(decrypt(encrypted, "wrong-password")).rejects.toThrow();
        });

        test("decrypt tampered ciphertext should throw", async () => {
            const { encrypted } = await encrypt("secret message", "password");
            const combined = new Uint8Array(base64Decode(encrypted));
            combined[combined.length - 1] ^= 0xff;
            combined[combined.length - 2] ^= 0xff;
            const tampered = await base64Encode(combined);
            await expect(decrypt(tampered, "password")).rejects.toThrow();
        });

        test("decrypt empty string should throw", async () => {
            await expect(decrypt("", "password")).rejects.toThrow();
        });

        test("encrypt empty message should succeed", async () => {
            const result = await encrypt("", "password");
            expect(result).toHaveProperty("encrypted");
            const decrypted = await decrypt(result.encrypted, "password");
            expect(decrypted).toBe("");
        });

        test("decompress non-gzip data should throw", async () => {
            const notGzip = new Uint8Array([0x00, 0x01, 0x02, 0x03]);
            await expect(decompress(notGzip)).rejects.toThrow();
        });
    });
});
