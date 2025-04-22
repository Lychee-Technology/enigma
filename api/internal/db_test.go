package internal

import (
	"log"
	"os"
	"testing"

	"github.com/oracle/nosql-go-sdk/nosqldb"
)

func clearTable(conn *OracleNoSqlEngimaDataSource) {
	// Delete all rows in the Engima table before each test

	deleteResult, err := conn.client.Query(&nosqldb.QueryRequest{
		Statement: "DELETE FROM Enigma",
		TableName: "Enigma",
	})

	if err != nil {
		log.Fatalf("Failed to clear Enigma table: %v", err)
	}

	log.Println(deleteResult)

	log.Println("Enigma table cleared.")
}

func TestDatabaseConnection(t *testing.T) {
	if os.Getenv("ENIGMA_TEST") != "Integration" {
		t.Skip("Skipping integration tests. Set ENIGMA_TEST=Integration to run.")
		return
	}

	// Implement your test logic here
	conn, err := NewOracleNoSqlEngimaDataSource()
	if err != nil {
		t.Fatalf("Failed to create database connection: %v", err)
	}

	t.Cleanup(func() {
		// Clear the table after the test
		clearTable(conn)
		conn.Close()
	})

	// Test if the connection is valid
	if conn.client == nil {
		t.Fatalf("Database connection is nil")
	}

}

func TestDatabaseQuery(t *testing.T) {
	if os.Getenv("ENIGMA_TEST") != "Integration" {
		t.Skip("Skipping integration tests. Set ENIGMA_TEST=Integration to run.")
		return
	}

	// Implement your test logic here
	conn, err := NewOracleNoSqlEngimaDataSource()
	if err != nil {
		t.Fatalf("Failed to create database connection: %v", err)
	}
	t.Cleanup(func() {
		// Clear the table after the test
		clearTable(conn)
		conn.Close()
	})
	id, err := conn.Save(&EnigmaRecord{
		SKey:        "123",
		ShortId:     "12345",
		Cookie:      "testCookie",
		Content:     "testContent",
		ContentHash: "testContentHash"}, 20)

	if err != nil {
		t.Fatalf("Failed to save record: %v", err)
	}
	record, err := conn.GetDataByShortId(id)
	if err != nil {
		t.Fatalf("Failed to get record by short id: %v", err)
	}
	if record == nil {
		t.Fatalf("Record is nil")
	}
	if record.ShortId != id {
		t.Fatalf("Record short id does not match, expected: %v, got: %v", id, record.ShortId)
	}
}
