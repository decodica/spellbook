package mailmessage

import (
	"decodica.com/spellbook"
	"encoding/json"
	"github.com/decodica/model/v2"
	"time"
)

type MailMessage struct {
	model.Model `json:"-"`
	Recipient   string    `model:"search,atom"`
	Sender      string    `model:"search"`
	Object      string    `model:"search"`
	Body        string    `model:"search"`
	Created     time.Time `model:"search"`
}

func (mailMessage *MailMessage) UnmarshalJSON(data []byte) error {

	alias := struct {
		Recipient string    `json:"recipient"`
		Sender    string    `json:"sender"`
		Object    string    `json:"object"`
		Body      string    `json:"body"`
		Created   time.Time `json:"created"`
	}{}

	err := json.Unmarshal(data, &alias)
	if err != nil {
		return err
	}

	mailMessage.Recipient = alias.Recipient
	mailMessage.Sender = alias.Sender
	mailMessage.Object = alias.Object
	mailMessage.Body = alias.Body
	mailMessage.Created = alias.Created

	return nil
}

func (mailMessage *MailMessage) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Recipient string    `json:"recipient"`
		Sender    string    `json:"sender"`
		Object    string    `json:"object"`
		Body      string    `json:"body"`
		Created   time.Time `json:"created"`
		Id        int64     `json:"id"`
	}

	return json.Marshal(&struct {
		Alias
	}{
		Alias{
			Recipient: mailMessage.Recipient,
			Sender:    mailMessage.Sender,
			Object:    mailMessage.Object,
			Body:      mailMessage.Body,
			Created:   mailMessage.Created,
			Id:        mailMessage.IntID(),
		},
	})
}

func (mailMessage *MailMessage) Id() string {
	return mailMessage.StringID()
}

func (mailMessage *MailMessage) FromRepresentation(rtype spellbook.RepresentationType, data []byte) error {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Unmarshal(data, mailMessage)
	}
	return spellbook.NewUnsupportedError()
}

func (mailMessage *MailMessage) ToRepresentation(rtype spellbook.RepresentationType) ([]byte, error) {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Marshal(mailMessage)
	}
	return nil, spellbook.NewUnsupportedError()
}
