package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/andrey/cfa-bonds/internal/auth"
)

func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger(s.log))

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowOrigins = s.cfg.CORSOrigins
	corsCfg.AllowHeaders = append(corsCfg.AllowHeaders, "Authorization")
	r.Use(cors.New(corsCfg))

	v1 := r.Group("/api/v1")
	v1.GET("/health", s.health)
	v1.GET("/ready", s.ready)

	authed := v1.Group("")
	authed.Use(auth.Middleware(s.auth, false))
	authed.Use(rateLimit(s.limiter, s.log))
	{
		authed.POST("/issuers", s.createIssuer)
		authed.GET("/issuers/:id", s.getIssuer)
		authed.GET("/issuers", s.listIssuers)

		authed.POST("/issues", s.createIssue)
		authed.GET("/issues/:id", s.getIssue)
		authed.GET("/issues", s.listIssues)
		authed.PUT("/issues/:id/status", s.updateIssueStatus)
		authed.GET("/issues/:id/holders", s.issueHolders)
		authed.GET("/issues/:id/trades", s.issueTrades)
		authed.GET("/issues/:id/events", s.issueEvents)
		authed.POST("/issues/:id/place", s.placeIssue)

		authed.POST("/trades", s.createTrade)
		authed.GET("/trades/:id", s.getTrade)
		authed.GET("/trades", s.listTrades)

		authed.POST("/investors", s.createInvestor)
		authed.GET("/investors/:id", s.getInvestor)
		authed.GET("/investors/:id/portfolio", s.investorPortfolio)
		authed.POST("/investors/:id/deposit", s.investorDeposit)
		authed.GET("/investors/:id/events", s.investorEvents)

		authed.GET("/analytics/overview", s.analyticsOverview)
		authed.GET("/analytics/issues/:id", s.analyticsIssue)
	}

	return r
}
