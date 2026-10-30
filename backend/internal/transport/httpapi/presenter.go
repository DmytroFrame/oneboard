package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/dashboard"
)

// Presenter turns domain objects into DTOs and signs the URLs inside them. <img> and <video> can't send
// the Authorization header, so file endpoints are public and the HMAC proves the URL came from an
// authenticated response.
type Presenter struct {
	key []byte
	now func() time.Time
}

func NewPresenter(cfg config.Config) *Presenter {
	// A key derived for this one purpose, so a URL signature can never double as anything else.
	mac := hmac.New(sha256.New, cfg.JWTSecret)
	mac.Write([]byte("flowboard/signed-urls"))
	return &Presenter{key: mac.Sum(nil), now: time.Now}
}

func (p *Presenter) sign(message string) string {
	mac := hmac.New(sha256.New, p.key)
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

func (p *Presenter) verify(message, sig string) bool {
	return hmac.Equal([]byte(sig), []byte(p.sign(message)))
}

// ---------- avatars: permanent URLs, versioned by upload time ----------

func avatarMessage(userID, version int64) string { return fmt.Sprintf("avatar:%d:%d", userID, version) }

func (p *Presenter) validAvatarSig(userID, version int64, sig string) bool {
	return p.verify(avatarMessage(userID, version), sig)
}

func (p *Presenter) User(u domain.User) userDTO {
	d := userDTO{ID: u.ID, Name: u.Name, Email: u.Email, Color: u.Color}
	if u.HasAvatar() {
		var v int64
		if u.AvatarUpdatedAt != nil {
			v = u.AvatarUpdatedAt.UnixMilli()
		}
		url := fmt.Sprintf("/api/users/%d/avatar?v=%d&sig=%s", u.ID, v, p.sign(avatarMessage(u.ID, v)))
		d.AvatarURL = &url
	}
	return d
}

func (p *Presenter) Me(u domain.User) meDTO {
	return meDTO{p.User(u), settingsDTO{u.Settings.Language, u.Settings.Theme}}
}

func (p *Presenter) Members(ms []domain.Member) []memberDTO {
	return mapSlice(ms, func(m domain.Member) memberDTO { return memberDTO{p.User(m.User), m.Role, m.AssignedCount} })
}

func (p *Presenter) OrgMembers(ms []domain.OrgMember) []orgMemberDTO {
	return mapSlice(ms, func(m domain.OrgMember) orgMemberDTO { return orgMemberDTO{p.User(m.User), m.Role, m.AddedAt} })
}

// ---------- attachments: URLs expire ----------

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

// ---------- cards ----------

func (p *Presenter) Card(c domain.Card) cardDTO {
	d := cardDTO{c.ID, c.ProjectID, c.ColumnID, c.Key, c.Title, c.Description, c.Priority, c.Position,
		c.AuthorID, c.AssigneeID, c.CreatedAt, c.UpdatedAt, c.AttachmentCount, nil}
	if cv := c.Cover; cv != nil {
		variant := "file"
		if cv.HasThumb {
			variant = "thumb"
		}
		d.Cover = &coverDTO{Kind: cv.Kind, URL: p.attachmentURL(cv.AttachmentID, variant)}
	}
	return d
}

func (p *Presenter) CardPage(page domain.CardPage) cardPageDTO {
	return cardPageDTO{Cards: mapSlice(page.Cards, p.Card), NextCursor: cursorString(page.NextCursor), Total: page.Total}
}

func (p *Presenter) Dashboard(d dashboard.Dashboard) dashboardDTO {
	var out dashboardDTO
	out.Stats.Organizations = d.Stats.Organizations
	out.Stats.Projects = d.Stats.Projects
	out.Stats.Assigned = d.Stats.Assigned
	out.Stats.Urgent = d.Stats.Urgent
	out.Stats.Created = d.Stats.Created
	out.Assigned = mapSlice(d.Assigned, func(a domain.AssignedCard) assignedCardDTO {
		return assignedCardDTO{p.Card(a.Card), a.ProjectName, a.ColumnName}
	})
	return out
}
