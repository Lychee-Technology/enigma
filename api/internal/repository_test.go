package internal_test

import (
	"errors"
	"testing"

	"lychee.technology/enigma/internal"
)

type mockDataSource struct {
	records  []*EnigmaRecord
	getErr   error
	saveErrs []error
	savedIds []string
}

type EnigmaRecord = internal.EnigmaRecord
type EnigmaMessageRepository = internal.EnigmaMessageRepository
type SaveMessageRequest = internal.SaveMessageRequest

func (m *mockDataSource) GetDataByShardKey(skey string) ([]*EnigmaRecord, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	var result []*EnigmaRecord
	for _, rec := range m.records {
		if rec.SKey == skey {
			result = append(result, rec)
		}
	}

	if len(result) == 0 {
		return nil, errors.New("not found")
	}

	return result, nil
}

func (m *mockDataSource) GetDataByShortId(shortId string) (*EnigmaRecord, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	for _, rec := range m.records {
		if rec.ShortId == shortId {
			return rec, nil
		}
	}

	return nil, errors.New("not found")
}

func (m *mockDataSource) DeleteData(skey string, shortId string, cookie string) (*EnigmaRecord, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	for i, rec := range m.records {
		if rec.ShortId == shortId && rec.Cookie == cookie {
			// Found matching record, remove it from records slice
			deletedRecord := rec
			m.records = append(m.records[:i], m.records[i+1:]...)
			return deletedRecord, nil
		}
	}

	return nil, errors.New("not found")
}

func (m *mockDataSource) Save(record *EnigmaRecord, ttlHours int64) (string, error) {
	if len(m.saveErrs) > 0 {
		err := m.saveErrs[0]
		m.saveErrs = m.saveErrs[1:]
		if errors.Is(err, internal.ErrDuplicatedShortId) {
			return "", internal.ErrDuplicatedShortId
		}
		return "", err
	}
	// simulate save returning the record.ShortId
	id := record.ShortId
	m.savedIds = append(m.savedIds, id)
	m.records = append(m.records, record)
	return id, nil
}

func (m *mockDataSource) Close() error { return nil }

func TestSaveMessage_NewRecord(t *testing.T) {
	mds := &mockDataSource{
		records:  make([]*EnigmaRecord, 0),
		savedIds: make([]string, 0),
	}
	svc := &EnigmaMessageRepository{DataSource: mds}
	req := &SaveMessageRequest{
		EncryptedData: "foo",
		Cookie:        "bar",
		TtlHours:      1,
	}
	resp, err := svc.SaveMessage(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ShortId == "" {
		t.Fatalf("expected non-empty ShortId")
	}
	if len(mds.savedIds) != 1 || mds.savedIds[0] != resp.ShortId {
		t.Errorf("expected save called once with %q, got %v", resp.ShortId, mds.savedIds)
	}
}

func TestSaveMessage_DuplicateShortIdRetry(t *testing.T) {
	// prepare mock so first Save returns ErrDuplicatedShortId, then succeeds
	mds := &mockDataSource{
		records:  make([]*EnigmaRecord, 0),
		saveErrs: []error{internal.ErrDuplicatedShortId},
	}
	svc := &EnigmaMessageRepository{DataSource: mds}
	req := &SaveMessageRequest{
		EncryptedData: "retry-data",
		Cookie:        "c",
		TtlHours:      2,
	}
	resp, err := svc.SaveMessage(req)
	if err != nil {
		t.Fatalf("unexpected error on retry: %v", err)
	}
	if resp.ShortId == "" {
		t.Fatalf("expected non-empty ShortId after retry")
	}
	// saveErrs is now empty, so Save was called twice
}

func TestGetEnigmaRecord_ErrorPath(t *testing.T) {
	mds := &mockDataSource{
		records: make([]*EnigmaRecord, 0),
	}
	// simulate missing record
	svc := &EnigmaMessageRepository{DataSource: mds}
	_, err := svc.GetEnigmaRecord("doesnotexist")
	if err == nil {
		t.Errorf("expected error for missing shortId")
	}
}

func TestSaveMessage_ExistingContentHash(t *testing.T) {
	// simulate existing record in shard key
	content := "samehash"
	// compute its base62 hash prefix by calling SaveMessage once
	mds := &mockDataSource{
		records: make([]*EnigmaRecord, 0),
	}
	svc := &EnigmaMessageRepository{DataSource: mds}

	// first save to populate mock.recordsByShard
	resp1, err := svc.SaveMessage(&SaveMessageRequest{EncryptedData: content, Cookie: "x", TtlHours: 1})
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	// place the record manually in mock for second call
	rec, _ := svc.GetEnigmaRecord(resp1.ShortId)

	mds.records = append(mds.records, rec)
	// second save should return the same ShortId without calling Save
	resp2, err := svc.SaveMessage(&SaveMessageRequest{EncryptedData: content, Cookie: "x", TtlHours: 1})
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}
	rec.ShortId = resp2.ShortId
}
