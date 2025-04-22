# Database

We use Oracle NoSQL Cloud to store data.

There are 1 type of entities in the table:

1. Encrypted Messages

## Table Schema

```sql
CREATE TABLE Enigma(
    SKey STRING,
    ShortId STRING,
    Content STRING NOT NULL DEFAULT "",
    ContentHash STRING DEFAULT "",
    Cookie STRING NOT NULL DEFAULT "",
    ExpiresAt LONG NOT NULL DEFAULT 0,
    PRIMARY KEY(SHARD(SKey), ShortId)
) USING TTL 8 DAYS
```

### SKey
It is the `shard key` of the table. 
`SKey = base62(sha256(content)).substring(0, 3)`

### ContentHash

`ContentHash = base62(sha256(content))`


### Content
It a string which maybe the URL or the encrypted message.


## Query

### Get a message by ShortId and Cookie

```sql
select Content from EnigmaData where 
    SKey = <shortId>.substring(0, 3)
    and ShortId = <shortId>
    and Cookie = <cookie>
    and ExpiresAt > unix_timestamp(now()) * 1000
    limit 1
```
