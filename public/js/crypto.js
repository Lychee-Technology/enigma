(async function () {
    const BIN_MIME = "application/octet-stream";
    const DATA_URL_PREFIX = `data:${BIN_MIME};base64,`;

    class StepMonad {
        conconstructor(v) {
            this.value = v;
        }

        map (func) {
           return new Monad(func(this.value)); 
        }

        async mapAsync (func) {
            const mapped = await func(this.value);
            return new Monad(mapped); 
        }
    }

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
     * @returns {string} base64 string
     */
    async function base64Encode(bytes) {
        return (await bytesToBase64DataUrl(bytes)).split(',', 2)[1];
    }

    /**
     * Decode base64 string to an array buffer.
     * @param {string} base64 
     * @returns {Promise<ArrayBuffer>} array buffer decoded from the base64
     */
    async function base64Decode(base64) {
        const res = await fetch(`${DATA_URL_PREFIX}${base64}`);
        return await res.arrayBuffer();
    }

    /**
     * hash data with SHA-512 for n iterations
     * @param {string} data text to hash
     * @param {number} iterations iterations
     * @returns {Promise<ArrayBuffer>} hash data
     */
    async function sha512(data, iterations = 1) {
        const encoder = new TextEncoder();
        let bytes = encoder.encode(data);
        for (let i = 0; i < iterations; i++) {
            bytes = await crypto.subtle.digest("SHA-512", bytes);
        }
        return bytes;
    }

    /**
     * Create key and IV from passowrd.
     * @param {string} password 
     * @param {KeyUsage} usage , can be "encrypt" or "decrypt"
     * @returns A tuple which contains key and IV.
     */
    async function passwordToKeyAndIV(password, usage) {
        const passwordHash = await sha512(password, 1000);
        const key = await window.crypto.subtle.importKey(
            "raw",
            passwordHash.slice(0, 16),
            "AES-GCM",
            false,
            [usage]);
        const iv = (await sha512(passwordHash, 1000)).slice(-12);
        return { key, iv };
    }

    /**
     * Encrypt the message with the given password.
     * @param {string} message message to encrypt
     * @param {string} password password
     * @returns {Promise<string>} Base64 encoded encrypted data.
     */
    async function encrypt(message, password) {
        const { key, iv } = await passwordToKeyAndIV(password, "encrypt");
        const encrypted = await window.crypto.subtle.encrypt(
            { name: "AES-GCM", iv: iv },
            key,
            await compress(message));
        return base64Encode(encrypted);
    }

    /**
     * Decrypt an base64 encrypted string.
     * @param {string} encryptedBase64 
     * @param {string} password 
     * @returns 
     */
    async function decrypt(encryptedBase64, password) {
        const { key, iv } = await passwordToKeyAndIV(password, "decrypt");
        const encryptedData = await base64Decode(encryptedBase64);
        const decrypted = await window.crypto.subtle.decrypt(
            { name: "AES-GCM", iv: iv },
            key,
            encryptedData);
        return await decompress(decrypted);
    }

    /**
     * Fetch encrypted data from server.
     * @param {string} id 
     * @param {string} ivSuffix 
     * @returns 
     */
    async function getEncryptedData(id, ivSuffix) {

        const res = await fetch(`/api/v1/messages/${id}?ivSuffix=${ivSuffix}`);
        return await res.text();
    }

    /**
     * Post encrypted message to server.
     * @param {string} message 
     * @param {string} password 
     * @returns 
     */
    async function postEncryptedMessage(message, password) {
        const encrypted = await encrypt(message, password);
        const res = await fetch("/api/v1/messages", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify({ encrypted })
        });
        return await res.json();
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
            const {value, done} = await reader.read();
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

    let func = null;
    if (window.location.pathname.endsWith("encrypt.html")) {
        func = encrypt;
    } else if (window.location.pathname.endsWith("decrypt.html")) {
        func = decrypt;
    }

    if (func) {
        const inputTextBox = document.querySelector("#inputText");
        const passwordInput = document.querySelector("#password");
        const outputText = document.querySelector("#outputText");
        const button = document.querySelector("#exec");
        button.addEventListener("click", async () => {
            const message = await func(inputTextBox.value, passwordInput.value);
            outputText.value = message;
        });
    }
})();