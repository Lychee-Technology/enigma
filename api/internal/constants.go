package internal

const (
	// MaxEncryptedDataLen is the base64-encoded ciphertext size limit (bytes).
	// 2000 UI characters × up to 3 UTF-8 bytes each → ~6 KB plaintext.
	// After gzip + AES-GCM-256 + 28-byte salt/IV header + base64: up to ~8 KB.
	MaxEncryptedDataLen = 8192
	MaxShortIdLen       = 18   // Maximum shortId length before collision exhaustion
	MaxTtlHours         = 168  // 7 days maximum message lifetime
	ShardKeyLen         = 3    // Number of chars used as Oracle NoSQL shard key
)
