package internal

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/oracle/nosql-go-sdk/nosqldb"
	"github.com/oracle/nosql-go-sdk/nosqldb/auth/iam"
	"github.com/oracle/nosql-go-sdk/nosqldb/common"
	"github.com/oracle/nosql-go-sdk/nosqldb/types"
)

const TableName = "Enigma"

var ErrDuplicatedShortId = errors.New("duplicated short id")

// DataSource defines the operations for storing and retrieving Enigma records.
type EngimaDataSource interface {
	// Save inserts a new record with an optional TTL (in hours).
	// Returns the generated ShortId or ErrDuplicatedShortId if it already exists.
	Save(record *EnigmaRecord, ttlHours int64) (string, error)

	// GetDataByShardKey returns all records whose ShortId starts with the given shard key.
	GetDataByShardKey(skey string) ([]*EnigmaRecord, error)

	// GetDataByShortId looks up a single record by its full ShortId.
	GetDataByShortId(shortId string) (*EnigmaRecord, error)

	DeleteData(skey string, shortId string, cookie string) (*EnigmaRecord, error)

	// Close releases any resources held by the data source.
	Close() error
}

type OracleNoSqlEngimaDataSource struct {
	client *nosqldb.Client
}

func createNoSqlDbConfig() (*nosqldb.Config, error) {
	configFile := os.Getenv("ENIGMA_OCI_CONFIG")

	if configFile == "" {
		return &nosqldb.Config{
			Endpoint: "http://kvlite:29999",
			Mode:     "onprem",
		}, nil
	}

	provider, err := iam.NewSignatureProviderFromFile(configFile, "", "", "")

	if err != nil {
		log.Fatalf("failed to create IAM signature provider: %v\n", err)
		return nil, err
	}

	return &nosqldb.Config{
		Mode:                  "cloud",
		Region:                common.RegionPHX,
		AuthorizationProvider: provider,
	}, nil
}

func NewOracleNoSqlEngimaDataSource() (*OracleNoSqlEngimaDataSource, error) {
	// Create an IAM authentication provider for Oracle NoSQL Cloud Service
	cfg, err := createNoSqlDbConfig()
	if err != nil {
		log.Fatalf("failed to create NoSQL config: %v\n", err)
		return nil, err
	}
	client, err := nosqldb.NewClient(*cfg)
	if err != nil {
		log.Fatalf("failed to create a NoSQL client: %v\n", err)
		return nil, err
	}
	log.Println("NoSQL client created.")
	return &OracleNoSqlEngimaDataSource{client: client}, nil
}

func (dataSource *OracleNoSqlEngimaDataSource) DeleteData(skey string, shortId string, cookie string) (*EnigmaRecord, error) {
	records, err := dataSource.query(
		"DECLARE $skey STRING; $shortId STRING; $cookie STRING; $now LONG; "+
			"SELECT SKey, Content, ContentHash, ShortId, Cookie "+
			"FROM Enigma WHERE SKey = $skey AND ShortId = $shortId AND Cookie = $cookie AND ExpiresAt > $now",
		skey, map[string]interface{}{"$shortId": shortId, "$cookie": cookie, "$now": time.Now().Unix()})

	if err != nil {
		return nil, err
	}

	count := len(records)

	log.Printf("[INFO] ShortId: %v, records count: %v", shortId, count)

	if count == 0 {
		return nil, fmt.Errorf("ShortId (%s) not found", shortId)
	}

	deleteResult, err := dataSource.client.Delete(&nosqldb.DeleteRequest{
		TableName: TableName,
		Key: types.NewMapValue(map[string]interface{}{
			"SKey":    records[0].SKey,
			"ShortId": records[0].ShortId,
		})})

	if err != nil {
		log.Printf("[WARN] failed to Delete: %v\n", err)
	}

	if !deleteResult.Success {
		log.Printf("[WARN] failed to Delete: %v\n", deleteResult)
	}

	return records[0], nil
}

func (dataSource *OracleNoSqlEngimaDataSource) AsEngimaDataSource() EngimaDataSource {
	return dataSource
}

func (dataSource *OracleNoSqlEngimaDataSource) Close() error {
	log.Println("Closing NoSQL client.")
	return dataSource.client.Close()
}

func (dataSource *OracleNoSqlEngimaDataSource) Save(record *EnigmaRecord, ttlHours int64) (string, error) {
	if len(record.ContentHash) < 3 {
		return "", fmt.Errorf("invalid content hash: %v", record.ContentHash)
	}

	value := types.NewMapValue(map[string]interface{}{
		"SKey":        record.SKey,
		"ShortId":     record.ShortId,
		"Cookie":      record.Cookie,
		"Content":     record.Content,
		"ContentHash": record.ContentHash,
		"ExpiresAt":   record.ExpiresAt,
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
		log.Printf("Save short id (%v) success\n", record.ShortId)
		return record.ShortId, nil
	}
	log.Printf("Save short id (%v) failed\n", record.ShortId)
	return "", ErrDuplicatedShortId
}

func (dataSource *OracleNoSqlEngimaDataSource) query(sql string, skey string, params map[string]interface{}) ([]*EnigmaRecord, error) {
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

func (dataSource *OracleNoSqlEngimaDataSource) GetDataByShardKey(skey string) ([]*EnigmaRecord, error) {
	return dataSource.query(
		"DECLARE $skey STRING; "+
			"SELECT SKey, Content, ContentHash, ShortId, Cookie "+
			"FROM Enigma WHERE SKey = $skey AND starts_with(ShortId, $skey) "+
			"ORDER BY ShortId",
		skey, map[string]interface{}{})
}

func (dataSource *OracleNoSqlEngimaDataSource) GetDataByShortId(shortId string) (*EnigmaRecord, error) {
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
		return nil, fmt.Errorf("ShortId (%s) not found", shortId)
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
