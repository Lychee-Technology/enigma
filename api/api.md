# APIs

## POST a message

### Method
`POST`
### URL
`/api/v1/messages`


### Request Body

```json
{
    "encryptedData": "<base64 string of AES encrypted data>",
    "ttl": "<number of minutes, 1(a minute) - 10080 (a week)>",
    "ivSuffix": "<last 32 bits (4 bytes) of initialization vector>",
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

### Method

`GET`

### URL

`/api/v1/messages/<id>?ivSuffix=<ivSuffix>`

### Response Body

```json
{
    "encryptedData": "<base64 string of encrypted data>",
}
```