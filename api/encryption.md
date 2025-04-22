# Encryption

Enigma uses `AES GCM` to [encrypt and decrypt](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/encrypt#aes-gcm) the message on client side. The server never knows the password.

## Key

`sha512(password)` for `10,000` iterations and take the first `128 bits/16 bytes`. The key will be never leave user's browser/device.

## Retrival token

The Retrival token is used for 

`sha512(password)` for `2,000` iterations and take the last `32 bits/4 bytes`. The retrival token will be sent to server.

## Initialization Vector

`sha512(password)` for `5,000` iterations and take the last `96 bit/12 bytes`. 

