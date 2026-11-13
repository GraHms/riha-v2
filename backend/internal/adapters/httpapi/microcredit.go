package httpapi

import (
	"github.com/gin-gonic/gin"

	"riha/internal/core/domain"
)

// requireMicrocredit closes every Microcrédito route to accounts a RIHA super admin
// has not turned the service on for.
func (srv *Server) requireMicrocredit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if e := domain.RequireMicrocredit(merchant(c)); e != nil {
			fail(c, e)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (srv *Server) microcreditRoutes(g *gin.RouterGroup) {
	g.GET("", func(c *gin.Context) {
		m := merchant(c)
		c.JSON(200, gin.H{"enabled": true, "since": iso(m.MicrocreditAt)})
	})
}
