package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/adapters/messaging"
	"riha/internal/core/domain"
)

// smsReport takes MozeSMS's delivery reports. MozeSMS doesn't sign them, so the
// address carries a secret only RIHA and MozeSMS know.
func (srv *Server) smsReport(c *gin.Context) {
	want := srv.cfg.SMSWebhookToken
	if want == "" || subtle.ConstantTimeCompare([]byte(c.Param("token")), []byte(want)) != 1 {
		c.JSON(404, gin.H{"error": gin.H{"code": "route_not_found", "message": "Rota desconhecida."}})
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	var r messaging.MozeSMSReport
	if err := json.Unmarshal(raw, &r); err != nil || r.Data.MessageID == "" {
		log.Printf("mozesms report: not understood (%d bytes)", len(raw))
		c.JSON(200, gin.H{"ok": true})
		return
	}
	status, why := r.Outcome()
	// MozeSMS gives report times in Maputo time, without a zone.
	var at time.Time
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", r.Data.DeliveryTime, domain.Maputo); err == nil {
		at = t.UTC()
	}
	if err := srv.A.MessageReport(ctx(c), r.Data.MessageID, status, why, at); err != nil {
		log.Printf("mozesms report: %v", err)
		c.JSON(500, gin.H{"ok": false})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
