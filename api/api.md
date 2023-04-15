# APIs

## POST a message

### Method
`POST`
### URL
`/api/v1/messages`


### Request Body

```json
{
    "encryptedData": <base64 string of DES encrypted data>,
    "ttl": <number of days, max: 7>,
    "iv": <96 bit initialization vector>,
    "hashSuffix": <hex string of the last 4 digits of sha256 hash of the password>
}
```

### Response Body

```json
{
    "id": "string of id",
    "expiresAt": 
}
```
## Get a message

### Method

`GET`

### URL

`/api/v1/messages/<id>?hashSuffix=<hashSuffix>`

### Response Body

```json
{
    "encryptedData": <base64>,
    "iv": <96 bit initialization vector>
}
```