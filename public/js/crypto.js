(async function () {
    const BIN_MIME = "application/octet-stream";
    const DATA_URL_PREFIX = `data:${BIN_MIME};base64,`;
    const HOURS_OF_ONE_DAY = 24;

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
     * @returns {Promise<object>} Base64 encoded encrypted data.
     */
    async function encrypt(message, password) {
        const { key, iv } = await passwordToKeyAndIV(password, "encrypt");
        const encrypted = await window.crypto.subtle.encrypt(
            { name: "AES-GCM", iv: iv },
            key,
            await compress(message));
        return { encrypted: await base64Encode(encrypted), cookie: (await base64Encode(iv)).substring(0, 3) };
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
     * Save message.
     * @param {string} encryptedData 
     * @param {string} cookie 
     * @param {number} ttl
     */
    async function saveMessage(encryptedData, cookie, ttlHours = MINUTES_OF_ONE_DAY) {

        if (!encryptedData || encryptedData.length == 0) {
            showError("Message is empty.")
            return "";
        }

        const resp = await fetch("/api/v1/messages", {
            method: "POST",
            body: JSON.stringify({
                encryptedData,
                cookie,
                ttlHours
            })
        });

        if (resp.ok) {
            showShortUrl((await resp.json()).shortId, document.getElementById("messageUrl"));
        } else {
            showError(`Error: ${resp.statusText}`)
        }
    }

    function showShortUrl(shortId, control) {
        const l = document.location
        const url = `${l.protocol}://${l.host}/${shortId}`
        control.value = url;
    }



    /**
     * Set tool tip content of an element.
     * @param {Element} element
     * @param {string} content 
     */
    function setToolTip(element, content) {
        bootstrap.Tooltip.getInstance(element).setContent({ '.tooltip-inner': content })
    }

    /**
     * Reset tool tip content of an element to its `data-bs-title` attribute 
     * @param {Element} element 
     */
    function resetToolTip(element) {
        setToolTip(element, element.getAttribute('data-bs-title'))
    }

    const clipboard = new ClipboardJS('.bi-clipboard');
    clipboard.on('success', e => {
        setToolTip(e.trigger, 'Copied!')
        e.trigger.classList.remove('bi-clipboard')
        e.trigger.classList.add('bi-check-circle-fill')
        e.clearSelection();
    });

    const elementsWithToolTip = document.querySelectorAll('[data-bs-toggle="tooltip"]');
    const toolTips = [...elementsWithToolTip].map(
        tooltipTriggerEl => new bootstrap.Tooltip(tooltipTriggerEl));
    elementsWithToolTip.forEach(tp => {
        tp.addEventListener('hidden.bs.tooltip',
            e => {
                resetToolTip(e.target)
                e.target.classList.remove('bi-check-circle-fill')
                e.target.classList.add('bi-clipboard')
            })
    });

    /**
     * Show error message.
     * @param {string} message 
     */
    function showError(message) {
        const errorSpan = document.getElementById("error-message");
        errorSpan.innerText = message;

        const errorContainer = document.getElementById("error-message-container");
        errorContainer.classList.remove('d-none');
        errorContainer.classList.add('d-block');
    }

    function hideError() {
        const errorContainer = document.querySelector("#error-message-container");
        errorContainer.classList.add('d-none');
        errorContainer.classList.remove('d-block');
    }

    const encryptMessageForm = document.getElementById("encrypt-message-form");
    const encryptMessageButton = document.getElementById("encrypt-message");
    const messageInput = document.getElementById("message");
    const messageCharCounter = document.getElementById("message-char-counter");
    const encryptMessageProgress = document.getElementById("encrypt-message-progress")
    const passowrdInput = document.getElementById("password");
    const ttlInput = document.getElementById("ttl");

    async function encryptMessage() {
        encryptMessageForm.classList.add('was-validated')

        if (!encryptMessageForm.checkValidity()) {
            e.preventDefault()
            e.stopPropagation()
            return
        }
        encryptMessageButton.classList.add('invisible')
        encryptMessageButton.classList.remove('mb-3')
        encryptMessageProgress.classList.remove('invisible')
        encryptMessageProgress.classList.add('mt-3')

        const message = messageInput.value;
        const passowrd = passowrdInput.value;
        const ttlHours = Number.parseInt(ttlInput.value);
        const { encrypted, cookie } = await encrypt(message, passowrd);
        await saveMessage(encrypted, cookie, ttlHours);

        encryptMessageProgress.classList.add('invisible');
        encryptMessageProgress.classList.remove('mt-3')
        encryptMessageButton.classList.remove('invisible');
        encryptMessageButton.classList.add('mb-3')
    }

    const shortenUrlForm = document.getElementById("shorten-url-form")
    const urlInput = document.getElementById("shorten-url-input");
    const shortenUrlButton = document.getElementById("shorten-url-button")

    shortenUrlButton.addEventListener('click', createShortUrl);


    /**
     * create short url
     * @param {Event} e 
     * @returns 
     */
    async function createShortUrl(e) {
        shortenUrlForm.classList.add('was-validated')

        if (!shortenUrlForm.checkValidity()) {
            e.preventDefault()
            e.stopPropagation()
            return
        }

        const resp = await fetch(
            "/api/v1/url", {
            method: "POST",
            body: JSON.stringify({ url: urlInput.value })
        });

        if (resp.ok) {
            showShortUrl((await resp.json()).shortId, document.getElementById("shortUrl"));
        } else {
            showError(`Error: ${resp.statusText}`)
        }
    }


    messageInput.addEventListener("keyup", e => {
        messageCharCounter.innerText = `${e.target.value.length}/2000`
    })

    encryptMessageButton.addEventListener('click', async e => {
        await encryptMessage(e)
    });
})();