package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/adapters/messaging"
	"riha/internal/core/domain"
)

// smsReport takes MozeSMS's delivery reports. MozeSMS doesn't document how it signs
// them, so a report decides nothing: RIHA asks MozeSMS, with its own key, what
// became of the message, and that answer counts. A forged report costs one lookup
// of a message RIHA sent, and changes nothing.
func (srv *Server) smsReport(c *gin.Context) {
	raw, _ := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	srv.logSMSReport(c, raw)
	var r messaging.MozeSMSReport
	if err := json.Unmarshal(raw, &r); err != nil || r.Data.MessageID == "" {
		c.JSON(200, gin.H{"ok": true})
		return
	}
	// MozeSMS gives report times in Maputo time, without a zone.
	var at time.Time
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", r.Data.DeliveryTime, domain.Maputo); err == nil {
		at = t.UTC()
	}
	check, cancel := context.WithTimeout(ctx(c), 8*time.Second)
	defer cancel()
	if err := srv.A.CheckMessage(check, r.Data.MessageID, at); err != nil {
		log.Printf("mozesms report: %v", err)
	}
	c.JSON(200, gin.H{"ok": true})
}

// logSMSReport logs what a report looked like, without values that could be
// secrets: the names of its headers, its event, and whether any header holds an
// HMAC-SHA256 of the body made with the webhook secret. That is how to find out
// MozeSMS's signature, and then require it.
func (srv *Server) logSMSReport(c *gin.Context, raw []byte) {
	names := make([]string, 0, len(c.Request.Header))
	for k := range c.Request.Header {
		names = append(names, k)
	}
	sort.Strings(names)
	var ev struct {
		Event string `json:"event"`
	}
	_ = json.Unmarshal(raw, &ev)
	signed := "no secret set"
	if secret := srv.cfg.SMSWebhookSecret; secret != "" {
		signed = "no header matches"
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(raw)
		sum := mac.Sum(nil)
		for _, k := range names {
			v := strings.TrimSpace(c.Request.Header.Get(k))
			v = strings.TrimPrefix(strings.TrimPrefix(v, "sha256="), "sha256:")
			if hmac.Equal([]byte(strings.ToLower(v)), []byte(hex.EncodeToString(sum))) || v == base64.StdEncoding.EncodeToString(sum) {
				signed = "HMAC-SHA256 of the body in " + k
			}
			if v == secret {
				signed = "the secret itself in " + k
			}
		}
	}
	log.Printf("mozesms report: %s from %s, %d bytes, headers=%v, signature: %s", domain.FirstNonEmpty(ev.Event, "?"), c.ClientIP(), len(raw), names, signed)
}
