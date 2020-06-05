package content

import (
	"database/sql"
	"decodica.com/spellbook"
	"encoding/json"
	"fmt"
	"github.com/decodica/model/v2"
	"strings"
	"time"
)

type ByOrder []*Content
type ByTitle []*Content
type ByStartDate []*Content
type ByPublished []*Content

/**
ByOrder start
*/
func (content ByOrder) Len() int {
	return len(content)
}

func (content ByOrder) Swap(i, j int) {
	content[i], content[j] = content[j], content[i]
}

func (content ByOrder) Less(i, j int) bool {
	return content[i].Order < content[j].Order
}

/**
ByOrder end
*/

/**
ByTitle start
*/
func (content ByTitle) Len() int {
	return len(content)
}

func (content ByTitle) Swap(i, j int) {
	content[i], content[j] = content[j], content[i]
}

func (content ByTitle) Less(i, j int) bool {
	return content[i].Title < content[j].Title
}

/**
ByTitle end
*/

/**
ByStartDate start
*/
func (content ByStartDate) Len() int {
	return len(content)
}

func (content ByStartDate) Swap(i, j int) {
	content[i], content[j] = content[j], content[i]
}

func (content ByStartDate) Less(i, j int) bool {
	return content[i].StartDate.Before(content[j].StartDate)
}

/**
ByStartDate end
*/

/**
ByPublished start
*/
func (content ByPublished) Len() int {
	return len(content)
}

func (content ByPublished) Swap(i, j int) {
	content[i], content[j] = content[j], content[i]
}

func (content ByPublished) Less(i, j int) bool {
	return content[i].Published.Before(content[j].Published)
}

/**
ByPublished end
*/

type PublicationState string

const PublicationStatePublished PublicationState = "PUBLISHED"
const PublicationStateUnpublished PublicationState = "UNPUBLISHED"

const AttachmentParentTypeContent = "content"

type Content struct {
	model.Model `json:"-"`
	ID          uint           `model:"-" json:"-"`
	Type        string         `model:"search"`
	IdTranslate string         `gorm:"UNIQUE_INDEX:content_idtranslate_locale"`
	Slug        string         `gorm:"-"`
	SqlSlug     sql.NullString `model:"-" gorm:"column:slug;UNIQUE_INDEX:content_slug"`
	Title       string         `model:"search"`
	Subtitle    string         `model:"search"`
	Body        string         `model:"search,noindex,HTML"`
	Tags        string         `model:"search"`
	Category    string         `model:"search,atom" page:"gettable,category"`
	Topic       string         `model:"search"`
	Locale      string         `model:"search,atom" gorm:"NOT NULL;UNIQUE_INDEX:content_code_locale,content_idtranslate_locale"`
	Description string         `model:"search"`
	Cover       string
	Revision    int
	Order       int           `model:"search"`
	Attachments []*Attachment `model:"-" gorm:"foreignkey:ParentID"`
	// username of the author
	Author           string `model:"search"`
	Editor           string `model:"search"`
	Created          time.Time
	Updated          time.Time        `model:"search"`
	Published        time.Time        `model:"search"`
	PublicationState PublicationState `model:"search,atom"`
	// todo: add slq parent id to the content model
	ParentKey string         `model:"search,atom" gorm:"column:parent"`
	Code      string         `gorm:"-"`
	SqlCode   sql.NullString `model:"-" gorm:"column:code;UNIQUE_INDEX:content_code_locale"`

	// KeyTypeEvent
	StartDate time.Time
	EndDate   time.Time
}

// code setters and getters
func (content *Content) setCode(code string) {
	content.Code = code
	if code == "" {
		content.SqlCode.Valid = false
		return
	}
	content.SqlCode.Valid = true
	content.SqlCode.String = code
}

func (content *Content) getCode() string {
	if content.SqlCode.Valid {
		return content.SqlCode.String
	}
	return content.Code
}

// slug setter and getter
func (content *Content) setSlug(slug string) {
	content.Slug = slug
	if slug == "" {
		content.SqlSlug.Valid = false
		return
	}
	content.SqlSlug.Valid = true
	content.SqlSlug.String = slug
}

func (content *Content) getSlug() string {
	if content.SqlSlug.Valid {
		return content.SqlSlug.String
	}
	return content.Slug
}

func (content Content) IsPublished() bool {
	return !content.Published.IsZero()
}

func (content Content) hasStartDate() bool {
	return !content.StartDate.IsZero()
}

func (content Content) hasEndDate() bool {
	return !content.EndDate.IsZero()
}

func (content *Content) UnmarshalJSON(data []byte) error {

	alias := struct {
		Type        string        `json:"type"`
		IdTranslate string        `json:"idTranslate"`
		Parent      string        `json:"parent"`
		Slug        string        `json:"slug"`
		Title       string        `json:"title"`
		Subtitle    string        `json:"subtitle"`
		Body        string        `json:"body"`
		Tags        []string      `json:"tags"`
		Category    string        `json:"category"`
		Topic       string        `json:"topic"`
		Locale      string        `json:"locale"`
		Description string        `json:"description"`
		Revision    int           `json:"revision"`
		Order       int           `json:"order"`
		Attachments []*Attachment `json:"attachments"`
		Author      string        `json:"author"`
		Editor      string        `json:"editor"`
		Cover       string        `json:"cover"`
		Code        string        `json:"code"`
		Created     time.Time     `json:"created"`
		Updated     time.Time     `json:"updated"`
		Published   time.Time     `json:"published"`
		IsPublished bool          `json:"isPublished"`
		StartDate   time.Time     `json:"startDate"`
		EndDate     time.Time     `json:"endDate"`
	}{}

	err := json.Unmarshal(data, &alias)
	if err != nil {
		return err
	}

	content.Type = alias.Type
	content.setSlug(alias.Slug)
	content.Title = alias.Title
	content.Subtitle = alias.Subtitle
	content.Body = alias.Body
	content.Category = alias.Category
	content.Topic = alias.Topic
	content.Locale = alias.Locale
	content.Description = alias.Description
	content.Revision = alias.Revision
	content.Order = alias.Order
	content.Author = alias.Author
	content.Cover = alias.Cover
	content.Editor = alias.Editor
	content.Attachments = alias.Attachments
	content.Created = alias.Created
	content.Updated = alias.Updated
	content.StartDate = alias.StartDate
	content.EndDate = alias.EndDate
	content.setCode(alias.Code)
	content.IdTranslate = alias.IdTranslate
	content.ParentKey = alias.Parent
	if alias.IsPublished {
		content.Published = time.Now().UTC()
	}
	content.Tags = strings.Join(alias.Tags[:], ";")

	return nil
}

func (content *Content) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Id          string        `json:"id"`
		Type        string        `json:"type"`
		IdTranslate string        `json:"idTranslate"`
		Slug        string        `json:"slug"`
		Title       string        `json:"title"`
		Subtitle    string        `json:"subtitle"`
		Body        string        `json:"body"`
		Tags        []string      `json:"tags"`
		Category    string        `json:"category"`
		Topic       string        `json:"topic"`
		Locale      string        `json:"locale"`
		Description string        `json:"description"`
		Revision    int           `json:"revision"`
		Order       int           `json:"order"`
		Attachments []*Attachment `json:"attachments"`
		Author      string        `json:"author"`
		Editor      string        `json:"editor"`
		Cover       string        `json:"cover"`
		Code        string        `json:"code"`
		Created     time.Time     `json:"created"`
		Updated     time.Time     `json:"updated"`
		Published   time.Time     `json:"published"`
		Parent      string        `json:"parent"`
		StartDate   time.Time     `json:"startDate"`
		EndDate     time.Time     `json:"endDate"`
	}

	tags := make([]string, 0, 0)
	if len(content.Tags) > 0 {
		tags = strings.Split(content.Tags, ";")
	}

	isPublished := content.IsPublished()
	hasEndDate := content.hasEndDate()
	hasStartDate := content.hasStartDate()

	return json.Marshal(&struct {
		Tags         []string `json:"tags"`
		IsPublished  bool     `json:"isPublished"`
		HasStartDate bool     `json:"hasStartDate"`
		HasEndDate   bool     `json:"hasEndDate"`
		Alias
	}{
		tags,
		isPublished,
		hasStartDate,
		hasEndDate,
		Alias{
			Id:          content.Id(),
			Type:        content.Type,
			IdTranslate: content.IdTranslate,
			Slug:        content.getSlug(),
			Title:       content.Title,
			Subtitle:    content.Subtitle,
			Body:        content.Body,
			Category:    content.Category,
			Topic:       content.Topic,
			Locale:      content.Locale,
			Description: content.Description,
			Cover:       content.Cover,
			Editor:      content.Editor,
			Revision:    content.Revision,
			Order:       content.Order,
			Attachments: content.Attachments,
			Author:      content.Author,
			Created:     content.Created,
			Code:        content.getCode(),
			Updated:     content.Updated,
			Published:   content.Published,
			StartDate:   content.StartDate,
			EndDate:     content.EndDate,
			Parent:      content.ParentKey,
		},
	})
}

/**
* Resource representation
 */

func (content *Content) Id() string {
	if id := content.EncodedKey(); id != "" {
		return id
	}
	return fmt.Sprintf("%d", content.ID)
}

func (content *Content) FromRepresentation(rtype spellbook.RepresentationType, data []byte) error {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Unmarshal(data, content)
	}
	return spellbook.NewUnsupportedError()
}

func (content *Content) ToRepresentation(rtype spellbook.RepresentationType) ([]byte, error) {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Marshal(content)
	}
	return nil, spellbook.NewUnsupportedError()
}
