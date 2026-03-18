package internal

import (
	"crypto/sha256"
	"errors"
	"log/slog"
	"math/big"
	"strings"
	"time"
)

var ErrContentTooLarge = errors.New("content too large")

type EnigmaMessageRepository struct {
	DataSource EnigmaDataSource
}

func (repository *EnigmaMessageRepository) SaveMessage(request *SaveMessageRequest) (*SaveMessageResponse, error) {
	if len(request.EncryptedData) > MaxEncryptedDataLen {
		return nil, ErrContentTooLarge
	}

	hash := sha256.Sum256([]byte(request.EncryptedData))
	var i big.Int

	base62Hash := i.SetBytes(hash[:]).Text(62)

	enigmaRecord := &EnigmaRecord{
		SKey:        base62Hash[0:ShardKeyLen],
		Content:     request.EncryptedData,
		ContentHash: base62Hash,
		Cookie:      request.Cookie,
		ExpiresAt:   time.Now().Unix() + request.TtlHours*3600,
	}

	shortId, err := repository.save(enigmaRecord, request.TtlHours)

	if err != nil {
		return nil, err
	}

	return &SaveMessageResponse{
		ShortId: shortId,
	}, nil
}

func (repository *EnigmaMessageRepository) GetEnigmaRecord(shortId string) (*EnigmaRecord, error) {
	return repository.DataSource.GetDataByShortId(shortId)
}

func (repository *EnigmaMessageRepository) save(record *EnigmaRecord, ttlHours int64) (string, error) {
	skey := record.ContentHash[0:ShardKeyLen]
	slog.Info("save", "skey", skey, "contentHash", record.ContentHash)
	records, err := repository.DataSource.GetDataByShardKey(skey)

	var longestShortId string = skey
	if err != nil {
		slog.Error("get data by shard key failed", "skey", skey, "error", err)
	} else {
		slog.Info("shard key lookup", "skey", skey, "count", len(records))
		for _, r := range records {
			if r.ContentHash == record.ContentHash {
				return r.ShortId, nil
			}
			if strings.HasPrefix(r.ShortId, longestShortId) && len(r.ShortId) > len(longestShortId) {
				longestShortId = r.ShortId
			}
		}
	}

	for i := len(longestShortId); i < MaxShortIdLen; i++ {
		record.ShortId = record.ContentHash[0:i]
		shortId, err := repository.DataSource.Save(record, ttlHours)
		if errors.Is(err, ErrDuplicatedShortId) {
			continue
		}
		if err != nil {
			return "", err
		}
		return shortId, nil
	}
	return "", errors.New("Save failed, all shortId are used")
}

func (repository *EnigmaMessageRepository) DeleteEnigmaRecord(shortId string, cookie string) (*EnigmaRecord, error) {
	return repository.DataSource.DeleteData(shortId[0:ShardKeyLen], shortId, cookie)
}

func (repository *EnigmaMessageRepository) Close() error {
	slog.Info("closing EnigmaMessageRepository")
	err := repository.DataSource.Close()
	if err != nil {
		slog.Error("close EnigmaMessageRepository failed", "error", err)
	}
	return err
}
