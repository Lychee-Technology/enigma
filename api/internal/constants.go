package internal

const (
	MaxEncryptedDataLen = 2000 // Base64 encoded AES-GCM ciphertext limit
	MaxShortIdLen       = 18   // Maximum shortId length before collision exhaustion
	MaxTtlHours         = 168  // 7 days maximum message lifetime
	ShardKeyLen         = 3    // Number of chars used as Oracle NoSQL shard key
)
