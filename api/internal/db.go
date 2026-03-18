package internal

import (
	"errors"
	"fmt"
	"log/slog"
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
type EnigmaDataSource interface {
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

type OracleNoSqlEnigmaDataSource struct {
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
		return nil, fmt.Errorf("failed to create IAM signature provider: %w", err)
	}

	return &nosqldb.Config{
		Mode:                  "cloud",
		Region:                common.RegionPHX,
		AuthorizationProvider: provider,
	}, nil
}

func NewOracleNoSqlEnigmaDataSource() (*OracleNoSqlEnigmaDataSource, error) {
	// Create an IAM authentication provider for Oracle NoSQL Cloud Service
	cfg, err := createNoSqlDbConfig()
	if err != nil {
		return nil, err
	}
	client, err := nosqldb.NewClient(*cfg)
	if err != nil {
		return nil, err
	}
	slog.Info("NoSQL client created")
	return &OracleNoSqlEnigmaDataSource{client: client}, nil
}

func (dataSource *OracleNoSqlEnigmaDataSource) DeleteData(skey string, shortId string, cookie string) (*EnigmaRecord, error) {
	records, err := dataSource.query(
		"DECLARE $skey STRING; $shortId STRING; $cookie STRING; $now LONG; "+
			"SELECT SKey, Content, ContentHash, ShortId, Cookie "+
			"FROM Enigma WHERE SKey = $skey AND ShortId = $shortId AND Cookie = $cookie AND ExpiresAt > $now",
		skey, map[string]interface{}{"$shortId": shortId, "$cookie": cookie, "$now": time.Now().Unix()})

	if err != nil {
		return nil, err
	}

	count := len(records)

	slog.Info("delete lookup", "shortId", shortId, "count", count)

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
		slog.Warn("delete failed", "error", err)
	}

	if !deleteResult.Success {
		slog.Warn("delete returned failure", "result", deleteResult)
	}

	return records[0], nil
}

func (dataSource *OracleNoSqlEnigmaDataSource) AsEnigmaDataSource() EnigmaDataSource {
	return dataSource
}

func (dataSource *OracleNoSqlEnigmaDataSource) Close() error {
	slog.Info("closing NoSQL client")
	return dataSource.client.Close()
}

func (dataSource *OracleNoSqlEnigmaDataSource) Save(record *EnigmaRecord, ttlHours int64) (string, error) {
	if len(record.ContentHash) < ShardKeyLen {
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
		if ttlHours > MaxTtlHours {
			ttlHours = MaxTtlHours
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
		slog.Error("failed to put data", "error", err)
		return "", err
	}

	if result.Success() {
		slog.Info("save success", "shortId", record.ShortId)
		return record.ShortId, nil
	}
	slog.Info("save failed (duplicate)", "shortId", record.ShortId)
	return "", ErrDuplicatedShortId
}

func (dataSource *OracleNoSqlEnigmaDataSource) query(sql string, skey string, params map[string]interface{}) ([]*EnigmaRecord, error) {
	if len(skey) != ShardKeyLen {
		return []*EnigmaRecord{}, fmt.Errorf("invalid shard key: %v", skey)
	}

	request := &nosqldb.PrepareRequest{
		Statement: sql,
	}
	prepareResult, err := dataSource.client.Prepare(request)

	if err != nil {
		slog.Error("failed to prepare query", "error", err)
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
		slog.Error("failed to execute query", "error", err)
		return []*EnigmaRecord{}, err
	}

	values, err := resp.GetResults()

	if err != nil {
		slog.Error("failed to get results", "error", err)
		return []*EnigmaRecord{}, err
	}

	result := make([]*EnigmaRecord, len(values))

	for i := 0; i < len(values); i++ {
		result[i] = toEnigmaRecord(values[i])
	}

	return result, nil
}

func (dataSource *OracleNoSqlEnigmaDataSource) GetDataByShardKey(skey string) ([]*EnigmaRecord, error) {
	return dataSource.query(
		"DECLARE $skey STRING; "+
			"SELECT SKey, Content, ContentHash, ShortId, Cookie "+
			"FROM Enigma WHERE SKey = $skey AND starts_with(ShortId, $skey) "+
			"ORDER BY ShortId",
		skey, map[string]interface{}{})
}

func (dataSource *OracleNoSqlEnigmaDataSource) GetDataByShortId(shortId string) (*EnigmaRecord, error) {
	records, err := dataSource.query(
		"DECLARE $skey STRING; $shortId STRING; "+
			"SELECT SKey, Content, ContentHash, ShortId, Cookie "+
			"FROM Enigma WHERE SKey = $skey AND ShortId = $shortId "+
			"ORDER BY ShortId",
		shortId[0:ShardKeyLen], map[string]interface{}{"$shortId": shortId})

	if err != nil {
		return nil, err
	}

	count := len(records)

	slog.Info("get by short id", "shortId", shortId, "count", count)

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
