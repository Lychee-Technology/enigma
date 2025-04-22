package internal

import (
	"crypto/sha256"
	"errors"
	"log"
	"math/big"
	"strings"
	"time"
)

var ErrContentTooLarge = errors.New("content too large")

type EnigmaMessageRepository struct {
	DataSource EngimaDataSource
}

func (repository *EnigmaMessageRepository) SaveMessage(request *SaveMessageRequest) (*SaveMessageResponse, error) {
	if len(request.EncryptedData) > 2000 {
		return nil, ErrContentTooLarge
	}

	log.Printf("%s", request.EncryptedData)
	hash := sha256.Sum256([]byte(request.EncryptedData))
	var i big.Int

	base62Hash := i.SetBytes(hash[:]).Text(62)

	engimaRecord := &EnigmaRecord{
		SKey:        base62Hash[0:3],
		Content:     request.EncryptedData,
		ContentHash: base62Hash,
		Cookie:      request.Cookie,
		ExpiresAt:   time.Now().Unix() + request.TtlHours*3600,
	}

	shortId, err := repository.save(engimaRecord, request.TtlHours)

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
	skey := record.ContentHash[0:3]
	log.Printf("save, SKey: %v, content hash: %v\n", skey, record.ContentHash)
	records, err := repository.DataSource.GetDataByShardKey(skey)

	log.Printf("save, found %v records with same SKey: %v\n", len(records), skey)

	for i := 0; i < len(records); i++ {
		r := records[i]
		log.Printf("save, Existed record, SKey: %v, content hash: %v\n", r.SKey, r.ContentHash)

		if r.ContentHash == record.ContentHash {
			return r.ShortId, nil
		}
	}

	var longestShortId string = skey
	if err != nil {
		log.Printf("get data by shard key (%v) failed, error: %v\n", skey, err)
	} else {
		for i := 0; i < len(records); i++ {
			r := records[i]
			if r.ContentHash == record.ContentHash {
				return r.ShortId, nil
			}
			if strings.HasPrefix(r.ShortId, longestShortId) && len(r.ShortId) > len(longestShortId) {
				longestShortId = r.ShortId
			}
		}
	}

	for i := len(longestShortId); i < 18; i++ {
		record.ShortId = record.ContentHash[0:i]
		shortId, err := repository.DataSource.Save(record, ttlHours)
		if errors.Is(err, ErrDuplicatedShortId) {
			continue
		}
		return shortId, nil
	}
	return "", errors.New("Save failed, all shortId are used")
}

func (repository *EnigmaMessageRepository) DeleteEnigmaRecord(shortId string, cookie string) (*EnigmaRecord, error) {
	return repository.DataSource.DeleteData(shortId[0:3], shortId, cookie)
}

func (repository *EnigmaMessageRepository) Close() error {
	log.Printf("Closing EnigmaMessageRepository.")
	err := repository.DataSource.Close()
	if err != nil {
		log.Printf("Close EnigmaService, error: %v.\n", err)
	}
	return err
}
