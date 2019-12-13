package subscription

import (
	"decodica.com/flamel/model"
	"decodica.com/spellbook"
	"decodica.com/spellbook/format/csv"
	"encoding/json"
	"time"
)

type Subscription struct {
	model.Model  `json:"-"`
	Email        string
	Country      string
	FirstName    string
	LastName     string
	Organization string
	Position     string
	Notes        string
	Created      time.Time
	Updated      time.Time
}

func (subscription *Subscription) UnmarshalJSON(data []byte) error {

	alias := struct {
		Email        string    `json:"email"`
		Country      string    `json:"country"`
		FirstName    string    `json:"firstName"`
		LastName     string    `json:"lastName"`
		Organization string    `json:"organization"`
		Position     string    `json:"position"`
		Notes        string    `json:"notes"`
		Created      time.Time `json:"created"`
		Updated      time.Time `json:"updated"`
	}{}

	err := json.Unmarshal(data, &alias)
	if err != nil {
		return err
	}

	subscription.Email = alias.Email
	subscription.Country = alias.Country
	subscription.FirstName = alias.FirstName
	subscription.LastName = alias.LastName
	subscription.Organization = alias.Organization
	subscription.Created = alias.Created
	subscription.Updated = alias.Updated
	subscription.Position = alias.Position
	subscription.Notes = alias.Notes

	return nil
}

func (subscription *Subscription) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Email        string    `json:"email"`
		Country      string    `json:"country"`
		FirstName    string    `json:"firstName"`
		LastName     string    `json:"lastName"`
		Organization string    `json:"organization"`
		Position     string    `json:"position"`
		Notes        string    `json:"notes"`
		Created      time.Time `json:"created"`
		Updated      time.Time `json:"updated"`
		Key          string    `json:"key"`
	}

	return json.Marshal(&struct {
		Alias
	}{
		Alias{
			Email:        subscription.Email,
			Country:      subscription.Country,
			FirstName:    subscription.FirstName,
			LastName:     subscription.LastName,
			Organization: subscription.Organization,
			Created:      subscription.Created,
			Updated:      subscription.Updated,
			Position:     subscription.Position,
			Notes:        subscription.Notes,
			Key:          subscription.EncodedKey(),
		},
	})
}

// csv methods
func (subscription *Subscription) ToCSV() ([]string, error) {
	return []string{
		subscription.Id(),
		subscription.Email,
		subscription.Country,
		subscription.FirstName,
		subscription.LastName,
		subscription.Organization,
		subscription.Position,
		subscription.Notes,
		subscription.Created.Format(time.RFC3339),
	}, nil
}

func (subscription *Subscription) FromCSV(csv []string) error {
	return spellbook.NewUnsupportedError()
}

func (subscription *Subscription) Id() string {
	return subscription.StringID()
}

func (subscription *Subscription) FromRepresentation(rtype spellbook.RepresentationType, data []byte) error {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Unmarshal(data, subscription)
	}
	return spellbook.NewUnsupportedError()
}

func (subscription *Subscription) ToRepresentation(rtype spellbook.RepresentationType) ([]byte, error) {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Marshal(subscription)
	case spellbook.RepresentationTypeCSV:
		s, err := csv.Marshal(subscription)
		return []byte(s), err
	}
	return nil, spellbook.NewUnsupportedError()
}
