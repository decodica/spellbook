package content

import (
	"database/sql"
	"decodica.com/spellbook"
	"encoding/json"
	"fmt"
	"github.com/decodica/model/v2"
	"strconv"
	"time"
)

const (
	// global name for attachments without parents
	AttachmentGlobalParent = "GLOBAL"

	// supported attachments
	AttachmentTypeGallery    = "gallery"
	AttachmentTypeAttachment = "attachments"
	AttachmentTypeVideo      = "video"
)

type Attachment struct {
	model.Model      `json:"-"`
	ID               uint `model:"-";json:"-"`
	Name             string
	AltText          string
	Description      string `model:"noindex"`
	ResourceUrl      string `model:"noindex"`
	ResourceThumbUrl string `model:"noindex"`
	Group            string
	Type             string
	ParentKey        string
	ParentType       string `gorm:"NOT NULL"`
	// inner foreign key when using sql backend
	ParentID     sql.NullInt64 `model:"-" json:"-" gorm:"type:integer"`
	DisplayOrder int
	Created      time.Time
	Updated      time.Time
	Uploader     string
}

func (attachment *Attachment) setParentKey(key string) {
	if key == AttachmentGlobalParent {
		attachment.ParentID.Valid = false
		attachment.ParentKey = key
		return
	}

	if v, err := strconv.Atoi(key); err == nil {
		attachment.ParentID.Int64 = int64(v)
		attachment.ParentID.Valid = true
	}
	attachment.ParentKey = key
}

// returns the global key if there is no foreign key set
// or returns the parent key if a foreign key has been set
func (attachment *Attachment) getParentKey() string {
	if attachment.ParentID.Valid {
		return fmt.Sprintf("%d", attachment.ParentID.Int64)
	}
	if attachment.ParentKey != "" {
		return attachment.ParentKey
	}
	return AttachmentGlobalParent
}

func (attachment *Attachment) UnmarshalJSON(data []byte) error {

	alias := struct {
		Name             string    `json:"name"`
		Description      string    `json:"description"`
		ResourceUrl      string    `json:"resourceUrl"`
		ResourceThumbUrl string    `json:"resourceThumbUrl"`
		Group            string    `json:"group"`
		Type             string    `json:"type"`
		ParentKey        string    `json:"parentKey"`
		ParentType       string    `json:"parentType"`
		Created          time.Time `json:"created"`
		Updated          time.Time `json:"updated"`
		Uploader         string    `json:"uploader"`
		AltText          string    `json:"altText"`
		DisplayOrder     int       `json:"displayOrder"`
	}{}

	err := json.Unmarshal(data, &alias)
	if err != nil {
		return err
	}

	attachment.Name = alias.Name
	attachment.Description = alias.Description
	attachment.ResourceUrl = alias.ResourceUrl
	attachment.ResourceThumbUrl = alias.ResourceThumbUrl
	attachment.Group = alias.Group
	attachment.Type = alias.Type
	attachment.setParentKey(alias.ParentKey)
	attachment.ParentType = alias.ParentType
	attachment.Created = alias.Created
	attachment.Updated = alias.Updated
	attachment.Uploader = alias.Uploader
	attachment.AltText = alias.AltText
	attachment.DisplayOrder = alias.DisplayOrder

	return nil
}

func (attachment *Attachment) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Id               string    `json:"id"`
		Name             string    `json:"name"`
		Description      string    `json:"description"`
		ResourceUrl      string    `json:"resourceUrl"`
		ResourceThumbUrl string    `json:"resourceThumbUrl"`
		Group            string    `json:"group"`
		Type             string    `json:"type"`
		ParentKey        string    `json:"parentKey"`
		ParentType       string    `json:"parentType"`
		Created          time.Time `json:"created"`
		Updated          time.Time `json:"updated"`
		Uploader         string    `json:"uploader"`
		AltText          string    `json:"altText"`
		DisplayOrder     int       `json:"displayOrder"`
	}

	return json.Marshal(&struct {
		Alias
	}{
		Alias{
			Id:               attachment.Id(),
			Name:             attachment.Name,
			Description:      attachment.Description,
			ResourceUrl:      attachment.ResourceUrl,
			ResourceThumbUrl: attachment.ResourceThumbUrl,
			Group:            attachment.Group,
			Type:             attachment.Type,
			ParentKey:        attachment.getParentKey(),
			ParentType:       attachment.ParentType,
			Created:          attachment.Created,
			Updated:          attachment.Updated,
			Uploader:         attachment.Uploader,
			AltText:          attachment.AltText,
			DisplayOrder:     attachment.DisplayOrder,
		},
	})
}

func (attachment *Attachment) Id() string {
	if id := attachment.EncodedKey(); id != "" {
		return id
	}
	return fmt.Sprintf("%d", attachment.ID)
}

func (attachment *Attachment) FromRepresentation(rtype spellbook.RepresentationType, data []byte) error {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Unmarshal(data, attachment)
	}
	return spellbook.NewUnsupportedError()
}

func (attachment *Attachment) ToRepresentation(rtype spellbook.RepresentationType) ([]byte, error) {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Marshal(attachment)
	}
	return nil, spellbook.NewUnsupportedError()
}
