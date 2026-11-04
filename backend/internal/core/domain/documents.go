package domain

type Attachment struct {
	Name        string
	ContentType string
	Data        []byte
}

type Email struct {
	To          string
	Subject     string
	Text        string
	Attachments []Attachment
}

type PayoutSheet struct {
	Payout   Payout
	Merchant Merchant
	Receipt  *Attachment
}
