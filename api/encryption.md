# Encryption

Enigma uses **AES-GCM with a 256-bit key** to encrypt and decrypt messages entirely in the browser. The server never receives the password or the plaintext.

## Key Derivation

The encryption key is derived using **PBKDF2-SHA256** (600,000 iterations — OWASP 2023 recommendation):

- A **random 16-byte salt** is generated at encryption time via `crypto.getRandomValues`.
- PBKDF2 combines the password and salt to produce a 256-bit AES-GCM key.
- The key never leaves the user's browser/device.

## Initialization Vector (Nonce)

A **random 12-byte IV** is generated at encryption time via `crypto.getRandomValues`.
Using a random IV ensures that the same password + plaintext combination produces a different ciphertext every time, preventing nonce reuse.

## Ciphertext Format

The value stored on the server is a **base64-encoded** concatenation:

| Offset (bytes) | Length | Field |
|----------------|--------|-------|
| 0 | 16 | PBKDF2 salt (random per message) |
| 16 | 12 | AES-GCM nonce / IV (random per message) |
| 28 | variable | AES-GCM ciphertext + 16-byte GCM authentication tag |

The salt and IV are prepended to the ciphertext so that decryption requires only the password — no separate metadata storage is needed.

Before encryption, the plaintext is **gzip-compressed** to reduce ciphertext size.

## Retrieval Token (Cookie)

The retrieval token is a **deterministic 4-byte value** (8 hex characters) derived from the password:

- Derived via **PBKDF2-SHA256** (100,000 iterations) with the fixed domain-separation salt `"enigma-retrieval-token-v2"`.
- The token is sent to the server as part of the GET/DELETE request URL.
- It routes the request to the correct record but does not protect the ciphertext — the AES key does.
- An attacker who obtains the token cannot derive the password or decrypt the message.

## ⚠️ Breaking Change — Schema Version v2

**Messages encrypted before the PBKDF2 migration (prior to 2026-03-18) used a different scheme:**

- Key derivation: iterated SHA-512 (10,000 rounds), no salt
- IV: deterministic, derived from the password hash
- Key size: 128-bit AES

These legacy messages **cannot be decrypted with the current code**.

A future improvement would prepend a 1-byte version field (e.g., `0x02` for the current scheme) to the ciphertext, allowing decoders to detect and handle both formats. This has not yet been implemented.
