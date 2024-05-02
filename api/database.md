# Database

We use Oracle NoSQL Cloud to store data.

There are 2 entities in the table:

1. Short URLs
2. Encrypted Messages

## Table Schema

### SKey
It is the `shard key` of the table. 
`SKey = base62(sha256(content)).substring(0, 3)`

### ShortId
It is the  `primary key` of the tbale.
It is a string which is the shortest prefix of `X (X > 5)` letters of the `base62(sha256(content))` which is globally unique.

### ContentHash

`ContentHash = base62(sha256(content))`


### Content
It a string which maybe the URL or the encrypted message.

### Cookie
Cookie: a string, nullable, it is only used for encrypted messages. Cook it the result of 10000 iterations of `sha256(iv)`

## Query

### Get a short URL by ShortId

```sql
select Content from EnigmaData where 
    SKey = <shortId>.substring(0, 5)
    and starts_with(PKey, <shortId>)
    and ShortId = <shortId>
    limit 1
```

### Get a message by ShortId and Cookie

```sql
select Content from EnigmaData where 
    SKey = <shortId>.substring(0, 5)
    and starts_with(PKey, <shortId>)
    and ShortId = <shortId>
    and Cookie = <cookie>
    limit 1
```