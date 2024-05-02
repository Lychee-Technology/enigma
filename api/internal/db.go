package internal

import (
	"errors"
	"fmt"
	"log"

	"github.com/oracle/nosql-go-sdk/nosqldb"
	"github.com/oracle/nosql-go-sdk/nosqldb/types"
)

const TableName = "Enigma"

var ErrDuplicatedShortId = errors.New("duplicated short id")

type EngimaDataSource struct {
	client *nosqldb.Client
}

func NewEngimaDataSource() (*EngimaDataSource, error) {
	cfg := nosqldb.Config{
		Endpoint: "http://localhost:29999",
		Mode:     "onprem",
	}

	client, err := nosqldb.NewClient(cfg)
	if err != nil {
		log.Fatalf("failed to create a NoSQL client: %v\n", err)
		return nil, err
	}
	log.Println("NoSQL client 	created.")
	return &EngimaDataSource{client: client}, nil
}

func (dataSource *EngimaDataSource) Close() error {
	return dataSource.client.Close()
}

func (dataSource *EngimaDataSource) Save(record *EnigmaRecord, ttlHours int64) (string, error) {
	if len(record.ContentHash) < 3 {
		return "", fmt.Errorf("invalid content hash: %v", record.ContentHash)
	}

	var success bool = false
	value := types.NewMapValue(map[string]interface{}{
		"SKey":        record.SKey,
		"ShortId":     record.ShortId,
		"Cookie":      record.Cookie,
		"Content":     record.Content,
		"ContentHash": record.ContentHash,
	})

	var ttl *types.TimeToLive = nil

	if ttlHours > 0 {
		if ttlHours > 168 {
			ttlHours = 168
		}
		ttl = &types.TimeToLive{
			Value: ttlHours,
			Unit:  types.Hours,
		}
	}

	request := &nosqldb.PutRequest{
		TableName: TableName,
		Value:     value,
		PutOption: types.PutIfAbsent,
		TTL:       ttl,
	}

	result, err := dataSource.client.Put(request)
	if err != nil {
		log.Fatalf("Failed to put data, %v", err)
		return "", err
	}

	if result.Success() {
		success = true
	}

	log.Printf("%v, %v\n", success, record.ShortId)

	if success {
		return record.ShortId, nil
	}

	return "", ErrDuplicatedShortId
}

func (dataSource *EngimaDataSource) query(sql string, skey string, params map[string]interface{}) ([]*EnigmaRecord, error) {
	if len(skey) != 3 {
		return []*EnigmaRecord{}, fmt.Errorf("invalid shard key: %v", skey)
	}

	request := &nosqldb.PrepareRequest{
		Statement: sql,
	}
	prepareResult, err := dataSource.client.Prepare(request)

	if err != nil {
		log.Printf("[ERROR] failed to Prepare: %v\n", err)
		return []*EnigmaRecord{}, err
	}

	prepareResult.PreparedStatement.SetVariable("$skey", skey)

	for name, value := range params {
		prepareResult.PreparedStatement.SetVariable(name, value)
	}

	resp, err := dataSource.client.Query(&nosqldb.QueryRequest{
		PreparedStatement: &prepareResult.PreparedStatement,
	})

	if err != nil {
		log.Printf("[ERROR] failed to Query: %v\n", err)
		return []*EnigmaRecord{}, err
	}

	values, err := resp.GetResults()

	if err != nil {
		log.Printf("[ERROR] failed to GetResults: %v\n", err)
		return []*EnigmaRecord{}, err
	}

	result := make([]*EnigmaRecord, len(values))

	for i := 0; i < len(values); i++ {
		result[i] = toEnigmaRecord(values[i])
	}

	return result, nil
}

func (dataSource *EngimaDataSource) GetDataByShardKey(skey string) ([]*EnigmaRecord, error) {
	return dataSource.query(
		"DECLARE $skey STRING; "+
			"SELECT SKey, Content, ContentHash, ShortId, Cookie "+
			"FROM Enigma WHERE SKey = $skey AND starts_with(ShortId, $skey) "+
			"ORDER BY ShortId",
		skey, map[string]interface{}{})
}

func (dataSource *EngimaDataSource) GetDataByShortId(shortId string) (*EnigmaRecord, error) {
	records, err := dataSource.query(
		"DECLARE $skey STRING; $shortId STRING; "+
			"SELECT SKey, Content, ContentHash, ShortId, Cookie "+
			"FROM Enigma WHERE SKey = $skey AND ShortId = $shortId "+
			"ORDER BY ShortId",
		shortId[0:3], map[string]interface{}{"$shortId": shortId})

	if err != nil {
		return nil, err
	}

	count := len(records)

	log.Printf("[INFO] ShortId: %v, records count: %v", shortId, count)

	if count == 0 {
		return nil, nil
	}

	return records[0], nil
}

func toEnigmaRecord(mapValue *types.MapValue) *EnigmaRecord {
	skey, _ := mapValue.GetString("SKey")
	shortId, _ := mapValue.GetString("ShortId")
	content, _ := mapValue.GetString("Content")
	contentHash, _ := mapValue.GetString("ContentHash")
	cookie, _ := mapValue.GetString("Cookie")

	return &EnigmaRecord{
		SKey:        skey,
		ShortId:     shortId,
		Content:     content,
		ContentHash: contentHash,
		Cookie:      cookie,
	}
}
