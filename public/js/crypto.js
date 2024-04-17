(async function () {
    const BIN_MIME = "application/octet-stream";
    const DATA_URL_PREFIX = `data:${BIN_MIME};base64,`;

    async function bytesToBase64DataUrl(bytes) {
        return await new Promise((resolve, reject) => {
            const reader = Object.assign(new FileReader(), {
                onload: () => resolve(reader.result),
                onerror: () => reject(reader.error)
            });
            reader.readAsDataURL(new Blob([bytes], { type: BIN_MIME }));
        });
    }

    // encode an ArrayBuffer to base64 string
    async function base64Encode(bytes) {
        return (await bytesToBase64DataUrl(bytes)).split(',', 2)[1];
    }

    // decode a base64 string to ArrayBuffer
    // base64 is a string
    async function base64Decode(base64) {
        const res = await fetch(`${DATA_URL_PREFIX}${base64}`);
        return await res.arrayBuffer();
    }

    // hash data with SHA-512 for n iterations
    // data is a string
    // n is an integer
    async function sha512(data, n = 1) {
        const encoder = new TextEncoder();
        let bytes = encoder.encode(data);
        for (let i = 0; i < n; i++) {
            bytes = await crypto.subtle.digest("SHA-512", bytes);
        }
        return bytes;
    }

    async function passwordToKeyAndIV(password, usage) {
        const passwordHash = await sha512(password, 1000);
        const key = await window.crypto.subtle.importKey(
            "raw",
            passwordHash.slice(0, 16),
            "AES-GCM",
            false,
            usage);
        const iv = passwordHash.slice(-12);
        return { key, iv };
    }

    // message is a string
    // password is a string
    async function encrypt(message, password) {
        const { key, iv } = await passwordToKeyAndIV(password, ["encrypt"]);
        const encrypted = await window.crypto.subtle.encrypt(
            { name: "AES-GCM", iv: iv },
            key,
            new TextEncoder().encode(message));
        return await base64Encode(encrypted);
    }

    // encrypted is a base64 string
    // password is a string
    async function decrypt(encryptedBase64, password) {
        const { key, iv } = await passwordToKeyAndIV(password, ["decrypt"]);
        const encryptedData = await base64Decode(encryptedBase64);
        const decrypted = await window.crypto.subtle.decrypt(
            { name: "AES-GCM", iv: iv },
            key,
            encryptedData);
        return new TextDecoder().decode(decrypted);
    }

    async function getEncryptedData(id, ivSuffix) {

        const res = await fetch(`/api/v1/messages/${id}?ivSuffix=${ivSuffix}`);
        return await res.text();
    }

    async function saveEncryptedMessage(message, password) {
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