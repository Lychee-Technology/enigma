package internal

type EnigmaRecord struct {
	SKey        string
	ShortId     string
	Content     string
	ContentHash string
	Cookie      string
}

type SaveMessageRequest struct {
	EncryptedData string `json:"encryptedData"`
	TtlHours           int64    `json:"ttlHours"`
	Cookie        string `json:"cookie"`
}

type SaveMessageResponse struct {
	ShortId string `json:"shortId"`
}

type GetMessageRequest struct {
	Cookie string `json:"cookie"`
}

type GetMessageResponse struct {
	EncryptedData string `json:"encryptedData"`
}

type SaveUrlRequest struct {
	Url string `json:"url"`
}

type SaveUrlResponse struct {
	ShortId string `json:"shortId"`
}
