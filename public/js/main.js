import { encrypt, decrypt, passwordToCryptoParams } from "./cryptoutil.mjs";

(async function () {
    function showShortUrl(shortId, control) {
        const l = document.location
        const url = `${l.protocol}//${l.host}/${shortId}`
        control.value = url;
    }

    function apiBaseUrl() {
        return "/api/v1"
    }

    /**
     * Fetch encrypted data from server.
     * @param {string} id 
     * @param {string} ivSuffix 
     * @returns {string} encrypted data.
     */
    async function getEncryptedData(id, cookie, turnstileToken) {
        const url = `${apiBaseUrl()}/messages/${id}/${cookie}`;
        console.log("getEncryptedData from: ", url);
        const res = await fetch(url, {
            headers: {
                "Authorization": `Turnstile ${turnstileToken}`
            }
        });
        if (!res.ok) {
            throw new Error({
                message: `Failed to fetch encrypted data. Status: ${res.statusText}`,
                status: res.status
            });
        }
        return (await res.json()).encryptedData;
    }

    /**
     * Save message.
     * @param {string} encryptedData 
     * @param {string} cookie 
     * @param {number} ttl
     */
    async function saveMessage(encryptedData, cookie, ttlHours, turnstileToken) {
        if (!message || message.length == 0) {
            showError("Message is empty.")
            return;
        }

        const resp = await fetch(`${apiBaseUrl()}/messages`, {
            method: "POST",
            body: JSON.stringify({
                encryptedData,
                cookie,
                ttlHours
            }),
            headers: {
                "Content-Type": "application/json",
                "Authorization": `Turnstile ${turnstileToken}`
            },
        });

        if (resp.ok) {
            showShortUrl((await resp.json()).shortId, document.getElementById("messageUrl"));
        } else {
            showError(`Error: ${resp.statusText}`)
        }
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
        navigator.share({
            title: "Encrypted message",
            text: "Here is your encrypted message.",
            url: e.trigger.getAttribute('data-bs-url')
        }).catch(err => {
            console.error("Error sharing the message:", err);
        });
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
    const decryptMessageButton = document.getElementById("decrypt-message");
    const messageInput = document.getElementById("message");
    const messageCharCounter = document.getElementById("message-char-counter");
    const encryptMessageProgress = document.getElementById("encrypt-message-progress")
    const passowrdInput = document.getElementById("password");
    const ttlInput = document.getElementById("ttl");

    async function encryptMessage(ev) {
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
        const ttlHours = Number.parseInt(ttlInput.value) * 24;
        const { encrypted, cookie } = await encrypt(message, passowrd);

        const turnstileDiv = document.getElementById('cf-turnstile');
        const turnstileToken = turnstileDiv.dataset.token;

        try {
            const resp = await saveMessage(encrypted, cookie, ttlHours, turnstileToken);
        } catch (err) {
            console.error("Error saving message:", err);
            showError("Failed to save the message. Please try again.");
            return;
        } finally {
            encryptMessageProgress.classList.add('invisible');
            encryptMessageProgress.classList.remove('mt-3')
            encryptMessageButton.classList.remove('invisible');
            encryptMessageButton.classList.add('mb-3')
        }
        // ... after successful encryption ...
        // Calculate expiry date based on TTL
        const expiryDate = new Date();
        expiryDate.setHours(expiryDate.getHours() + ttlHours);
        const expiryDateString = expiryDate.toLocaleDateString(undefined, {
            weekday: 'long',
            year: 'numeric',
            month: 'long',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
        document.getElementById('expiry-info').textContent = `ℹ️ This message will expire on ${expiryDateString}.`;
        document.getElementById('message-result-box').classList.remove('d-none');
    }

    messageInput.addEventListener("keyup", e => {
        messageCharCounter.innerText = `${e.target.value.length}/2000`
    })

    encryptMessageButton.addEventListener('click', async e => {
        await encryptMessage(e)
    });

    // Decrypt simulation logic (demo purpose)
    decryptMessageButton.addEventListener('click', async () => {
        const password = document.getElementById('decryptPassword').value;
        if (!password) {
            showError("Please enter a password.");
            // Optionally trigger Bootstrap validation here
            return;
        }

        const path = location.pathname;
        if (path.length <= 3) {
            showError("Invalid URL. Please check the link.");
            return;
        }

        const { cookie } = await passwordToCryptoParams(password, "decrypt");

        document.getElementById('decrypt-progress').classList.remove('invisible');
        document.getElementById('decrypt-progress').classList.add('mt-3');
        const turnstileDiv = document.getElementById('cf-turnstile');
        const turnstileToken = turnstileDiv.dataset.token;

        try {
            const shortId = path.substring(1);
            const encryptedData = await getEncryptedData(shortId, cookie, turnstileToken);
            const decryptedMessage = await decrypt(encryptedData, password);
            document.getElementById('decrypt-result').value = decryptedMessage;
            document.getElementById('decrypted-message-box').classList.remove('d-none');
        } catch (err) {
            console.error("Error fetching encrypted data:", err);
            if (err.status >= 400 && err.status < 500) {
                // Handle client-side errors or resource not found
                showError("Oops! It seems like there’s a mix-up with the link or password. Could you please double-check and try again? Thanks a bunch!");
            } else {
                // Server-side errors
                showError("Oops! It seems like there was a hiccup and we couldn’t get the encrypted data. But don’t worry, just give it a bit of time and try again.");
            }
            return;
        } finally {
            document.getElementById('decrypt-progress').classList.add('invisible');
            document.getElementById('decrypt-progress').classList.remove('mt-3');
        }
    });

    // Initialize tooltips
    const tooltipTriggerList = document.querySelectorAll('[data-bs-toggle="tooltip"]');
    const tooltipList = [...tooltipTriggerList].map(tooltipTriggerEl => new bootstrap.Tooltip(tooltipTriggerEl));

    // Update TTL label and hidden select based on range slider
    const ttlRange = document.getElementById('ttl');
    const ttlLabel = document.getElementById('ttl-label');
    const ttlSelect = document.getElementById('ttl-select'); // Assuming you might need this for form submission

    if (ttlRange && ttlLabel) {
        ttlRange.addEventListener('input', function () {
            const days = parseInt(this.value);
            ttlLabel.textContent = `${days} ${days === 1 ? 'day' : 'days'}`;
            // Optional: Update hidden select value (hours)
            if (ttlSelect) {
                ttlSelect.value = days * 24;
            }
        });
        // Initial update
        const initialDays = parseInt(ttlRange.value);
        ttlLabel.textContent = `${initialDays} ${initialDays === 1 ? 'day' : 'days'}`;
        if (ttlSelect) {
            ttlSelect.value = initialDays * 24;
        }
    }

    const path = location.pathname;
    if (path.length > 3) {
        document.getElementById('decrypt-section').classList.remove('d-none');
        document.getElementById('underline-encryptor').classList.add('d-none');
    } else {
        document.getElementById('underline-encryptor').classList.remove('d-none');
        document.getElementById('decrypt-section').classList.add('d-none');
    }
})();