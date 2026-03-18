# Encryption

Enigma uses `AES-GCM` with a **256-bit key** to [encrypt and decrypt](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/encrypt#aes-gcm) the message on the client side. The server never sees the password or the plaintext.

## Key Derivation

The encryption key is derived using **PBKDF2-SHA256** (600,000 iterations, OWASP 2023 recommendation):

- A **random 16-byte salt** is generated at encryption time using `crypto.getRandomValues`.
- The salt and password are fed into PBKDF2 to produce a 256-bit AES-GCM key.
- The key never leaves the user's browser/device.

## Initialization Vector (IV / Nonce)

A **random 12-byte IV** is generated at encryption time using `crypto.getRandomValues`.
AES-GCM requires a unique nonce per (key, plaintext) pair; using a random IV achieves this without coordination.

## Ciphertext Format

The output stored on the server is a base64-encoded concatenation:

```
base64( salt[16] || iv[12] || AES-GCM-ciphertext )
```

The salt and IV are prepended to the ciphertext so decryption only requires the password.

## Retrieval Token (Cookie)

The retrieval token is used to route the GET request to the correct record on the server. It must be deterministic (same password → same token) but does not protect the ciphertext — the AES key does.

Derived via **PBKDF2-SHA256** (100,000 iterations) with a fixed domain-separation salt `"enigma-retrieval-token-v2"`, producing 4 bytes (8 hex chars). The token is sent to the server and stored alongside the ciphertext; it cannot be used to recover the password or decrypt the message.
