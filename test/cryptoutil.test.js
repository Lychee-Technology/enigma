// cryptoutil.test.js
import { beforeAll, expect, jest } from '@jest/globals';
import { webcrypto } from 'node:crypto';
import { TextEncoder, TextDecoder } from 'util';
import { CompressionStream, DecompressionStream } from 'stream/web';
import { Blob, Buffer } from 'node:buffer';
import { mockFetchBase64DataUrl } from './mockFetch.mjs'; 

import {
    bytesToBase64DataUrl,
    base64Encode,
    base64Decode,
    passwordToCryptoParams,
    encrypt,
    decrypt,
    createIv,
    createCookie,
    compress,
    decompress,
} from '../public/js/cryptoutil.mjs';

const BIN_MIME = "application/octet-stream";
const DATA_URL_PREFIX = `data:${BIN_MIME};base64,`;

// Import the functions to test
// In a real environment, you would import from the actual file
// For this test, we'll mock the functions based on the provided code

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
            FileReader,
            fetch: mockFetchBase64DataUrl,
        });

        // Mock FileReader implementation for testing purposes.
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
                }
                ).catch((error) => {
                    if (this.onerror) {
                        this.onerror(error);
                    }
                });
            }
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
        it('should decode base64 string to arraybuffer', async () => {
            const base64 = "YWJj";
            const result = await base64Decode(base64);
            expect(new TextDecoder().decode(result)).toBe('abc');
        });
    });

    describe('passwordToEncryptParams', () => {
        it('should create key, IV and cookie from password', async () => {
            const password = "testPassword";
            const usage = "encrypt";

            const mockKey = { type: 'secret', algorithm: 'AES-GCM' };

            const importKeyOriginal = crypto.subtle.importKey;
            const mockImportKey = jest.fn().mockResolvedValue(mockKey);
            crypto.subtle.importKey = mockImportKey;
            const result = await passwordToCryptoParams(password, usage);
            crypto.subtle.importKey = importKeyOriginal;

            expect(result).toEqual({
                key: mockKey,
                iv: expect.any(Object),
                cookie: expect.any(String)
            });

            expect(mockImportKey).toHaveBeenCalledWith(
                "raw",
                expect.any(Object),
                "AES-GCM",
                false,
                [usage]
            );
        });
    });

    describe('createCookie', () => {
        it('should create a cookie (retrieval token) from password hash', () => {
            const passwordHash = new Uint8Array(64);
            passwordHash[0] = 10; // offset will be 10 % 52 = 10

            for (let i = 1; i < 64; i++) {
                passwordHash[i] = i;
            }

            const result = createCookie(passwordHash);

            expect(result).toEqual('0a0b0c0d');
        });
    });

    describe('createIv', () => {
        it('should create an IV from password hash', () => {
            const passwordHash = new Uint8Array(64);
            passwordHash[63] = 10; // offset will be 10 % 52 = 10
            for (let i = 0; i < 63; i++) {
                passwordHash[i] = i;
            }
            const result = createIv(passwordHash);
            expect(result).toEqual(passwordHash.slice(10, 22));
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



// The functions under test are assumed to be in the global scope (loaded via a script tag or similar)
// If you are using modules, adjust the imports accordingly.

describe("cryptoutil functions", () => {
    test("createCookie returns correct slice from ArrayBuffer", () => {
        // Create a sample Uint8Array of 60 bytes: values 0 .. 59.
        const sample = new Uint8Array(60);
        for (let i = 0; i < 60; i++) {
            sample[i] = i;
        }
        const offset = sample[0] % 52;
        const expected = Array.from(new Uint8Array(sample.slice(offset, offset + 4)))
            .map((byte) => byte.toString(16).padStart(2, '0'))
            .join('');

        const token = createCookie(sample);
        // Compare the returned Uint8Array with expected slice.
        expect(token).toEqual(expected);
    });

    test("createIv returns correct slice from ArrayBuffer", () => {
        // Create a sample Uint8Array of 70 bytes.
        const sample = new Uint8Array(70);
        for (let i = 0; i < 70; i++) {
            sample[i] = i;
        }
        const offset = sample[sample.byteLength - 1] % 52;
        const expected = sample.slice(offset, offset + 12);

        const iv = createIv(sample);
        expect(new Uint8Array(iv)).toEqual(expected);
    });

    test("compress and decompress roundtrip", async () => {
        // Only run this test if CompressionStream and DecompressionStream are available.
        if (
            typeof CompressionStream === "function" &&
            typeof DecompressionStream === "function"
        ) {
            const original = "The quick brown fox jumps over the lazy dog";
            const compressed = await compress(original);
            const decompressed = await decompress(compressed);
            expect(decompressed).toBe(original);
        } else {
            // Skip test if compression APIs are not available.
            expect(true).toBe(true);
        }
    });

    test("encrypt and decrypt roundtrip", async () => {
        // Using a simple password and message. This test relies on all crypto APIs and compression APIs.
        // It also depends on the passwordToKeyAndIV function (referenced from encrypt/decrypt).
        // Make sure that the functions encrypt() and decrypt() are loaded into the global scope.
        const message = "Hello, world!";
        const password = "correcthorsebatterystaple";

        // Encrypt the message.
        const encryptedResult = await encrypt(message, password);
        expect(encryptedResult).toHaveProperty("encrypted");
        expect(encryptedResult.encrypted.length).toBeGreaterThan(0);

        // Decrypt the message.
        const decryptedMessage = await decrypt(encryptedResult.encrypted, password);
        expect(decryptedMessage).toBe(message);
    });
});
