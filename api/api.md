# APIs

## Send a message

`POST /api/v1/messages`

### Request Body

```json
{
    "encryptedData": "<base64 string of AES encrypted data>",
    "ttl": "<number of minutes, 1(a minute) - 10080 (a week)>",
    "cookie": "<last 32 bits (4 bytes) of initialization vector>",
}
```

### Response Body

```json
{
    "id": "<string of id>",
    "expiresAt": "<epoch in sec>"
}
```
## Get a message

`GET /api/v1/messages/<id>?ivSuffix=<ivSuffix>`

### Response Body

```json
{
    "encryptedData": "<base64 string of encrypted data>",
}
```

## Create a short URL

`POST /api/v1/shorturl`

### Request Body

```json
{
    "url": "<url>"
}
```