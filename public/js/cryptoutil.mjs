const BIN_MIME = "application/octet-stream";
const DATA_URL_PREFIX = `data:${BIN_MIME};base64,`;

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
 * Decode base64 string to an array buffer.
 * @param {string} base64 
 * @returns {Promise<ArrayBuffer>} array buffer decoded from the base64
 */
async function base64Decode(base64) {
    const res = await fetch(`${DATA_URL_PREFIX}${base64}`);

    if (!res.ok) {
        throw new Error(`Failed to decode base64`);
    }
    const data = await res.arrayBuffer();
    return data;
}

/**
 * hash data with SHA-512 for n iterations
 * @param {string | ArrayBuffer} data text to hash
 * @param {number} iterations iterations
 * @returns {Promise<ArrayBuffer>} hash data
 */
async function sha512(data, iterations = 1) {
    const encoder = new TextEncoder();
    let bytes = typeof data === 'string' ? encoder.encode(data) : data;
    for (let i = 0; i < iterations; i++) {
        bytes = await crypto.subtle.digest("SHA-512", bytes);
    }
    return bytes;
}

/**
 * Create key, IV and cookie (retrieval token) from passowrd.
 * @param {string} password 
 * @param {KeyUsage} usage , can be "encrypt" or "decrypt"
 * @returns A tuple which contains key and IV.
 */
async function passwordToCryptoParams(password, usage) {
    const passwordHash2000 = await sha512(password, 2000);

    // The cookie is a 4-byte value derived from the password hash.
    // It serves to verify the user’s likelihood of possessing the correct password.
    const cookie = createCookie(passwordHash2000);

    const passwordHash5000 = (await sha512(passwordHash2000, 3000));
    const iv = passwordHash5000.slice(-12);

    const passwordHash10k = await sha512(passwordHash5000, 5000);

    const keyBuffer = passwordHash10k.slice(0, 16);

    const key = await window.crypto.subtle.importKey(
        "raw",
        keyBuffer,
        "AES-GCM",
        false,
        [usage]);

    return { key, iv, cookie };
}

/**
 * create a cookie (retrieval token) from the password hash.
 * @param {ArrayBuffer} passwordHash 
 * @returns {string} cookie
 */
function createCookie(passwordHash) {
    const passwordBytes = new Uint8Array(passwordHash);
    const offset = passwordBytes[0] % 52;
    return uint8ArrayToHex(passwordBytes.slice(offset, offset + 4));
}

/**
 * create an IV from the password hash.
 * @param {ArrayBuffer} passwordHash 
 * @returns 
 */
function createIv(passwordHash) {
    const offset = passwordHash[passwordHash.byteLength - 1] % 52
    return passwordHash.slice(offset, offset + 12);
}

/**
 * Encrypt the message with the given password.
 * @param {string} message message to encrypt
 * @param {string} password password
 * @returns {Promise<object>} Base64 encoded encrypted data.
 */
async function encrypt(message, password) {
    const { key, iv, cookie } = await passwordToCryptoParams(password, "encrypt");

    const encrypted = await window.crypto.subtle.encrypt(
        { name: "AES-GCM", iv: iv },
        key,
        await compress(message));
    return {
        encrypted: await base64Encode(encrypted),
        cookie,
    };
}

/**
 * Decrypt an base64 encrypted string.
 * @param {string} encryptedBase64 
 * @param {string} password 
 * @returns 
 */
async function decrypt(encryptedBase64, password) {
    const { key, iv } = await passwordToCryptoParams(password, "decrypt");
    const encryptedData = await base64Decode(encryptedBase64);
    const decrypted = await window.crypto.subtle.decrypt(
        { name: "AES-GCM", iv: iv },
        key,
        encryptedData);
    return await decompress(decrypted);
}


/**
 * Transform an readable stream to Uint8Array, like compress or decompress
 * @param {string | Uint8Array} data
 * @param {GenericTransformStream} transformStream
 * @returns {Promise<Uint8Array>} output data
 */
async function transform(data, transformStream) {
    const inputStream = new Blob([data]).stream();
    const transformedStream = inputStream.pipeThrough(transformStream);
    // Read all the bytes from this stream.
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
    // Convert the string to a byte stream.
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
    // Convert the bytes to a string.
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
 * Create a hex string from an Uint8Array.
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
    createIv,
    createCookie,
    compress,
    decompress,
};