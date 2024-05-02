package internal

import (
	"crypto/sha256"
	"errors"
	"log"
	"math/big"
	"strings"
)

var ErrContentTooLarge = errors.New("content too large")

type EnigmaService struct {
	DataSource *EngimaDataSource
}

func (service *EnigmaService) SaveMessage(request *SaveMessageRequest) (*SaveMessageResponse, error) {
	if len(request.EncryptedData) > 2000 {
		return nil, ErrContentTooLarge
	}

	log.Printf(request.EncryptedData)
	hash := sha256.Sum256([]byte(request.EncryptedData))
	var i big.Int

	base62Hash := i.SetBytes(hash[:]).Text(62)

	engimaRecord := &EnigmaRecord{
		SKey:        base62Hash[0:3],
		Content:     request.EncryptedData,
		ContentHash: base62Hash,
		Cookie:      request.Cookie,
	}

	shortId, err := service.save(engimaRecord, request.TtlHours)

	if err != nil {
		return nil, err
	}

	return &SaveMessageResponse{
		ShortId: shortId,
	}, nil
}

func (service *EnigmaService) GetEnigmaRecord(shortId string) (*EnigmaRecord, error) {
	return service.DataSource.GetDataByShortId(shortId)
}

func (service *EnigmaService) SaveUrl(request *SaveUrlRequest) (*SaveUrlResponse, error) {
	if len(request.Url) > 2000 {
		return nil, ErrContentTooLarge
	}

	hash := sha256.Sum256([]byte(request.Url))
	var i big.Int

	base62Hash := i.SetBytes(hash[:]).Text(62)

	engimaRecord := &EnigmaRecord{
		SKey:        base62Hash[0:3],
		Content:     request.Url,
		ContentHash: base62Hash,
	}

	shortId, err := service.save(engimaRecord, 0)

	if err != nil {
		return nil, err
	}

	return &SaveUrlResponse{
		ShortId: shortId,
	}, nil
}

func (service *EnigmaService) save(record *EnigmaRecord, ttlHours int64) (string, error) {
	skey := record.ContentHash[0:3]
	log.Printf("save, SKey: %v, content hash: %v\n", skey, record.ContentHash)
	records, err := service.DataSource.GetDataByShardKey(skey)

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
		shortId, err := service.DataSource.Save(record, ttlHours)
		if errors.Is(err, ErrDuplicatedShortId) {
			continue
		}
		return shortId, nil
	}
	return "", errors.New("")
}

func (service *EnigmaService) Close() error {
	if service.DataSource != nil {
		err := service.DataSource.Close()
		if err != nil {
			log.Printf("Close EnigmaService, error: %v.\n", err)
		}
		return err
	}
	return nil
}
