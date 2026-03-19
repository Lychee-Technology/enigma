const BIN_MIME = "application/octet-stream";

/**
 * Encode an ArrayBuffer to Data Url
 * @param {ArrayBuffer} bytes
 * @returns {Promise<string>} data url
 */
async function bytesToBase64DataUrl(bytes) {
    return await new Promise((resolve, reject) => {
        const reader = Object.assign(new FileReader(), {
            onload: () => resolve(reader.result),
            onerror: () => reject(reader.error)
        });
        reader.readAsDataURL(new Blob([bytes], { type: BIN_MIME }));
    });
}

/**
 * Encode an ArrayBuffer to base64 string
 * @param {ArrayBuffer} bytes
 * @returns {Promise<string>} base64 string
 */
async function base64Encode(bytes) {
    const base64String = (await bytesToBase64DataUrl(bytes)).split(',', 2)[1];
    return base64String;
}

/**
 * Decode a base64 string to an ArrayBuffer.
 * @param {string} base64
 * @returns {ArrayBuffer}
 */
function base64Decode(base64) {
    const binaryString = atob(base64);
    const len = binaryString.length;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) {
        bytes[i] = binaryString.charCodeAt(i);
    }
    return bytes.buffer;
}

/**
 * Derive an AES-GCM-256 key from password and salt using PBKDF2-SHA256.
 * @param {string} password
 * @param {Uint8Array} salt  16-byte random salt
 * @param {KeyUsage} usage   "encrypt" or "decrypt"
 * @returns {Promise<CryptoKey>}
 */
async function deriveKey(password, salt, usage) {
    const encoder = new TextEncoder();
    const keyMaterial = await crypto.subtle.importKey(
        "raw",
        encoder.encode(password),
        "PBKDF2",
        false,
        ["deriveKey"]
    );
    return crypto.subtle.deriveKey(
        {
            name: "PBKDF2",
            salt,
            iterations: 600000,
            hash: "SHA-256",
        },
        keyMaterial,
        { name: "AES-GCM", length: 256 },
        false,
        [usage]
    );
}

/**
 * Derive a deterministic retrieval token (cookie) from the password.
 * Uses PBKDF2 with a fixed domain-separation salt so the same password
 * always produces the same token, enabling server-side lookup without
 * storing the password.
 * @param {string} password
 * @returns {Promise<string>} 8-char hex cookie
 */
async function deriveRetrievalToken(password) {
    const encoder = new TextEncoder();
    const keyMaterial = await crypto.subtle.importKey(
        "raw",
        encoder.encode(password),
        "PBKDF2",
        false,
        ["deriveBits"]
    );
    const bits = await crypto.subtle.deriveBits(
        {
            name: "PBKDF2",
            salt: encoder.encode("enigma-retrieval-token-v2"),
            iterations: 100000,
            hash: "SHA-256",
        },
        keyMaterial,
        32  // 4 bytes = 32 bits
    );
    return uint8ArrayToHex(new Uint8Array(bits));
}

/**
 * Create key and cookie (retrieval token) from password.
 * Kept for backward compatibility with existing call sites in main.js.
 * Returns only `cookie`; key derivation now requires a salt and is handled
 * inside encrypt() / decrypt().
 * @param {string} password
 * @returns {Promise<{cookie: string}>}
 */
async function passwordToCryptoParams(password) {
    const cookie = await deriveRetrievalToken(password);
    return { cookie };
}

/**
 * Encrypt the message with the given password.
 *
 * Output format (base64-encoded): [16-byte salt][12-byte IV][ciphertext]
 *
 * @param {string} message   plaintext message to encrypt
 * @param {string} password  encryption password
 * @returns {Promise<{encrypted: string, cookie: string}>}
 */
async function encrypt(message, password) {
    const salt = crypto.getRandomValues(new Uint8Array(16));
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const key = await deriveKey(password, salt, "encrypt");
    const cookie = await deriveRetrievalToken(password);

    const ciphertext = await crypto.subtle.encrypt(
        { name: "AES-GCM", iv },
        key,
        await compress(message)
    );

    // Prepend salt and IV so decrypt() can recover them without extra storage.
    const combined = await concatUint8Arrays([salt, iv, new Uint8Array(ciphertext)]);
    return {
        encrypted: await base64Encode(combined),
        cookie,
    };
}

/**
 * Decrypt a base64-encoded ciphertext produced by encrypt().
 *
 * Expected format: [16-byte salt][12-byte IV][ciphertext]
 *
 * @param {string} encryptedBase64
 * @param {string} password
 * @returns {Promise<string>} decrypted plaintext
 */
async function decrypt(encryptedBase64, password) {
    const combined = new Uint8Array(base64Decode(encryptedBase64));
    const salt = combined.slice(0, 16);
    const iv = combined.slice(16, 28);
    const ciphertext = combined.slice(28);
    const key = await deriveKey(password, salt, "decrypt");
    const decrypted = await crypto.subtle.decrypt(
        { name: "AES-GCM", iv },
        key,
        ciphertext
    );
    return await decompress(decrypted);
}


/**
 * Transform a readable stream with a TransformStream (e.g. compress or decompress).
 * @param {string | Uint8Array} data
 * @param {GenericTransformStream} transformStream
 * @returns {Promise<Uint8Array>} output data
 */
async function transform(data, transformStream) {
    const inputStream = new Blob([data]).stream();
    const transformedStream = inputStream.pipeThrough(transformStream);
    const chunks = [];
    const reader = transformedStream.getReader();
    while (true) {
        const { value, done } = await reader.read();
        if (value) {
            chunks.push(value);
        }
        if (done) {
            break;
        }
    }
    reader.releaseLock();
    await transformedStream.cancel();
    return await concatUint8Arrays(chunks);
}

/**
 * Convert a string to its UTF-8 bytes and compress it.
 *
 * @param {string} data
 * @returns {Promise<Uint8Array>}
 */
async function compress(data) {
    return await transform(data, new CompressionStream("gzip"));
}

/**
 * Decompress bytes into a UTF-8 string.
 *
 * @param {Uint8Array} compressed
 * @returns {Promise<string>}
 */
async function decompress(compressed) {
    const decompressed = await transform(compressed, new DecompressionStream("gzip"));
    return new TextDecoder().decode(decompressed);
}

/**
 * Combine multiple Uint8Arrays into one.
 *
 * @param {ReadonlyArray<Uint8Array>} uint8arrays
 * @returns {Promise<Uint8Array>}
 */
async function concatUint8Arrays(uint8arrays) {
    const blob = new Blob(uint8arrays);
    const buffer = await blob.arrayBuffer();
    return new Uint8Array(buffer);
}

/**
 * Create a hex string from a Uint8Array.
 *
 * @param {Uint8Array} uint8array
 * @returns {string}
 */
function uint8ArrayToHex(uint8array) {
    return Array.from(uint8array)
        .map((byte) => byte.toString(16).padStart(2, "0"))
        .join('');
}

export {
    bytesToBase64DataUrl,
    base64Encode,
    base64Decode,
    passwordToCryptoParams,
    encrypt,
    decrypt,
    compress,
    decompress,
};
