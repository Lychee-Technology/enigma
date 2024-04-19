# Encryption

Enigma uses `AES GCM` to [encrypt and decrypt](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/encrypt#aes-gcm) the message on client side. The server never knows the password.

## Key

`sha512(password)` for `1,000` iterations and take the first `128 bits/16 bytes`. The key will be never leave user's browser/device.

## Initialization Vector

`sha512(password)` for `2,000` iterations and take the last `96 bit/12 bytes`. 

