package content

import (
	"cloud.google.com/go/datastore"
	"decodica.com/spellbook"
	"encoding/json"
	"github.com/decodica/model/v2"
	"regexp"
	"time"
)

type Place struct {
	model.Model  `json:"-"`
	Name         string             `json:"name"`
	Address      string             `json:"address"`
	Street       string             `json:"street"`
	StreetNumber string             `json:"streetNumber"`
	Area         string             `json:"area"`
	City         string             `json:"city"`
	PostalCode   string             `json:"postalCode"`
	Country      string             `json:"country"`
	Phone        string             `json:"phone"`
	Description  string             `json:"description";model:"noindex"`
	Position     datastore.GeoPoint `model:"search"`
	Website      string             `json:"website"`
	Created      time.Time          `json:"created"`
	Updated      time.Time          `json:"updated"`
}

var extract = regexp.MustCompile("[^0-9+]+")

func (place Place) FormatPhone() string {
	return extract.ReplaceAllString(place.Phone, "")
}

func (place *Place) UnmarshalJSON(data []byte) error {

	alias := struct {
		Name         string    `json:"name"`
		Address      string    `json:"address"`
		Street       string    `json:"street"`
		StreetNumber string    `json:"streetNumber"`
		Area         string    `json:"area"`
		City         string    `json:"city"`
		PostalCode   string    `json:"postalCode"`
		Country      string    `json:"country"`
		Phone        string    `json:"phone"`
		Description  string    `json:"description"`
		Lat          float64   `json:"lat"`
		Lng          float64   `json:"lng"`
		Website      string    `json:"website"`
		Created      time.Time `json:"created"`
		Updated      time.Time `json:"updated"`
	}{}

	err := json.Unmarshal(data, &alias)
	if err != nil {
		return err
	}

	place.Name = alias.Name
	place.Address = alias.Address
	place.Street = alias.Street
	place.StreetNumber = alias.StreetNumber
	place.Area = alias.Area
	place.City = alias.City
	place.PostalCode = alias.PostalCode
	place.Country = alias.Country
	place.Phone = alias.Phone
	place.Description = alias.Description
	place.Website = alias.Website
	place.Created = alias.Created
	place.Updated = alias.Updated
	place.Position = datastore.GeoPoint{Lat: alias.Lat, Lng: alias.Lng}

	return nil
}

func (place *Place) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Name         string    `json:"name"`
		Address      string    `json:"address"`
		Street       string    `json:"street"`
		StreetNumber string    `json:"streetNumber"`
		Area         string    `json:"area"`
		City         string    `json:"city"`
		PostalCode   string    `json:"postalCode"`
		Country      string    `json:"country"`
		Phone        string    `json:"phone"`
		Description  string    `json:"description"`
		Lat          float64   `json:"lat"`
		Lng          float64   `json:"lng"`
		Website      string    `json:"website"`
		Created      time.Time `json:"created"`
		Updated      time.Time `json:"updated"`
		Id           int64     `json:"id"`
	}

	return json.Marshal(&struct {
		Alias
	}{
		Alias{
			Name:         place.Name,
			Address:      place.Address,
			Street:       place.Street,
			StreetNumber: place.StreetNumber,
			Area:         place.Area,
			City:         place.City,
			PostalCode:   place.PostalCode,
			Country:      place.Country,
			Phone:        place.Phone,
			Description:  place.Description,
			Lat:          place.Position.Lat,
			Lng:          place.Position.Lng,
			Website:      place.Website,
			Created:      place.Created,
			Updated:      place.Updated,
			Id:           place.IntID(),
		},
	})
}

func (place *Place) Id() string {
	return place.StringID()
}

func (place *Place) FromRepresentation(rtype spellbook.RepresentationType, data []byte) error {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Unmarshal(data, place)
	}
	return spellbook.NewUnsupportedError()
}

func (place *Place) ToRepresentation(rtype spellbook.RepresentationType) ([]byte, error) {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Marshal(place)
	}
	return nil, spellbook.NewUnsupportedError()
}
