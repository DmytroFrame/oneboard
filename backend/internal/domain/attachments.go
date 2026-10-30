package domain

import "time"

const (
	MaxAttachmentBytes    = 50 << 20
	MaxAttachmentsPerCard = 50
)

type AttachmentKind string

const (
	AttachmentImage AttachmentKind = "image"
	AttachmentVideo AttachmentKind = "video"
	AttachmentFile  AttachmentKind = "file"
)

// previewTypes are content types the browser may render inline. Everything else (including SVG and
// HTML, which can run scripts) is only ever served as a download.
var previewTypes = map[string]AttachmentKind{
	"image/png":  AttachmentImage,
	"image/jpeg": AttachmentImage,
	"image/gif":  AttachmentImage,
	"image/webp": AttachmentImage,
	"video/mp4":  AttachmentVideo,
	"video/webm": AttachmentVideo,
}

// KindOf classifies a sniffed content type.
func KindOf(contentType string) AttachmentKind {
	if k, ok := previewTypes[contentType]; ok {
		return k
	}
	return AttachmentFile
}

type Attachment struct {
	ID          int64
	CardID      int64
	ProjectID   int64
	UploaderID  *int64
	FileName    string
	ContentType string
	Size        int64
	Kind        AttachmentKind
	StorageKey  string
	ThumbKey    string // "" when there is no thumbnail
	CreatedAt   time.Time
}

func (a Attachment) HasThumb() bool { return a.ThumbKey != "" }

// CardCover is the attachment shown on the card face: the first image or video.
type CardCover struct {
	AttachmentID int64
	Kind         AttachmentKind
	HasThumb     bool
}
