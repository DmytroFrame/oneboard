package httpapi

import (
	"fmt"
	"strconv"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// JSON shapes of the attachments module.

type attachmentDTO struct {
	ID          int64                 `json:"id"`
	CardID      int64                 `json:"cardId"`
	FileName    string                `json:"fileName"`
	ContentType string                `json:"contentType"`
	Size        int64                 `json:"size"`
	Kind        domain.AttachmentKind `json:"kind"`
	UploaderID  *int64                `json:"uploaderId"`
	CreatedAt   time.Time             `json:"createdAt"`
	URL         string                `json:"url"`         // inline view (images, videos)
	DownloadURL string                `json:"downloadUrl"` // always a download
	ThumbURL    *string               `json:"thumbUrl"`
}

// Attachment URLs expire.
// The expiry is rounded to a day boundary (valid for 1–2 days), so the URL stays the same across
// requests during a day and the browser cache keeps working.
func (p *Presenter) attachmentExpiry() int64 {
	const day = 24 * 60 * 60
	return (p.now().Unix()/day + 2) * day
}

func attachmentMessage(id int64, variant string, exp int64) string {
	return fmt.Sprintf("attachment:%d:%s:%d", id, variant, exp)
}

func (p *Presenter) attachmentURL(id int64, variant string) string {
	exp := p.attachmentExpiry()
	return fmt.Sprintf("/api/attachments/%d/%s?exp=%d&sig=%s", id, variant, exp, p.sign(attachmentMessage(id, variant, exp)))
}

func (p *Presenter) validAttachmentSig(id int64, variant, expParam, sig string) bool {
	exp, err := strconv.ParseInt(expParam, 10, 64)
	if err != nil || p.now().Unix() > exp {
		return false
	}
	return p.verify(attachmentMessage(id, variant, exp), sig)
}

func (p *Presenter) Attachment(a domain.Attachment) attachmentDTO {
	url := p.attachmentURL(a.ID, "file")
	d := attachmentDTO{
		ID: a.ID, CardID: a.CardID, FileName: a.FileName, ContentType: a.ContentType, Size: a.Size,
		Kind: a.Kind, UploaderID: a.UploaderID, CreatedAt: a.CreatedAt,
		URL: url, DownloadURL: url + "&download=1",
	}
	if a.HasThumb() {
		thumb := p.attachmentURL(a.ID, "thumb")
		d.ThumbURL = &thumb
	}
	return d
}
