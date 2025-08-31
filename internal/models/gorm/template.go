package gorm

import (
	"time"
)

type Template struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	DisplayName   string    `gorm:"not null" json:"displayName"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	PreviewImage  string    `json:"previewImage"`
	SVGBackground string    `json:"svgBackground"`
	DataInterface string    `json:"dataInterface"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`

	Fields        []Field        `gorm:"foreignKey:TemplateID" json:"fields"`
	SVGFiles      []SVGFile      `gorm:"foreignKey:TemplateID" json:"svgFiles,omitempty"`
	Submissions   []FormSubmission `gorm:"foreignKey:TemplateID" json:"submissions,omitempty"`
}

type Field struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateID         string    `gorm:"not null;index" json:"templateId"`
	Name               string    `gorm:"not null" json:"name"`
	Type               string    `gorm:"not null" json:"type"`
	Required           bool      `json:"required"`
	DataKey            string    `gorm:"not null" json:"dataKey"`
	IsAddressComponent bool      `json:"isAddressComponent"`
	FontSize           int       `gorm:"default:12" json:"fontSize"`
	PositionTop        int       `json:"positionTop"`
	PositionLeft       int       `json:"positionLeft"`
	PositionWidth      int       `json:"positionWidth"`
	PositionHeight     int       `json:"positionHeight"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`

	Template Template `gorm:"foreignKey:TemplateID" json:"-"`
}

type Position struct {
	Top    int `json:"top"`
	Left   int `json:"left"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (f *Field) GetPosition() Position {
	return Position{
		Top:    f.PositionTop,
		Left:   f.PositionLeft,
		Width:  f.PositionWidth,
		Height: f.PositionHeight,
	}
}

func (f *Field) SetPosition(pos Position) {
	f.PositionTop = pos.Top
	f.PositionLeft = pos.Left
	f.PositionWidth = pos.Width
	f.PositionHeight = pos.Height
}

type SVGFile struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateID   string    `gorm:"not null;index" json:"templateId"`
	Filename     string    `gorm:"not null" json:"filename"`
	OriginalName string    `json:"originalName"`
	FilePath     string    `gorm:"not null" json:"filePath"`
	FileSize     int64     `json:"fileSize"`
	MimeType     string    `json:"mimeType"`
	GCSPath      string    `json:"gcsPath,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`

	Template Template `gorm:"foreignKey:TemplateID" json:"-"`
}

type FormSubmission struct {
	ID         string                 `gorm:"primaryKey" json:"id"`
	TemplateID string                 `gorm:"not null;index" json:"templateId"`
	FormData   map[string]interface{} `gorm:"serializer:json" json:"formData"`
	Status     string                 `gorm:"default:draft" json:"status"`
	CreatedAt  time.Time             `json:"createdAt"`
	UpdatedAt  time.Time             `json:"updatedAt"`

	Template Template `gorm:"foreignKey:TemplateID" json:"-"`
}

func (Template) TableName() string {
	return "templates"
}

func (Field) TableName() string {
	return "template_fields"
}

func (SVGFile) TableName() string {
	return "svg_files"
}

func (FormSubmission) TableName() string {
	return "form_submissions"
}