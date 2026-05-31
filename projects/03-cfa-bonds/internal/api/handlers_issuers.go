package api

import (
	"github.com/gin-gonic/gin"

	"github.com/andrey/cfa-bonds/internal/models"
)

type createIssuerReq struct {
	Name         string `json:"name" binding:"required"`
	INN          string `json:"inn" binding:"required"`
	OGRN         string `json:"ogrn"`
	ContactEmail string `json:"contact_email"`
}

func (s *Server) createIssuer(c *gin.Context) {
	var req createIssuerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid issuer payload: "+err.Error())
		return
	}
	issuer := &models.Issuer{
		Name:         req.Name,
		INN:          req.INN,
		OGRN:         req.OGRN,
		ContactEmail: req.ContactEmail,
		Active:       true,
	}
	if err := s.issuers.Create(c.Request.Context(), issuer); err != nil {
		failWith(c, err)
		return
	}
	created(c, issuer)
}

func (s *Server) getIssuer(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	issuer, err := s.issuers.Get(c.Request.Context(), id)
	if err != nil {
		failWith(c, err)
		return
	}
	sendOK(c, issuer)
}

func (s *Server) listIssuers(c *gin.Context) {
	limit, offset := parsePaging(c)
	items, total, err := s.issuers.List(c.Request.Context(), limit, offset)
	if err != nil {
		failWith(c, err)
		return
	}
	listResponse(c, items, pageMeta{Limit: limit, Offset: offset, Total: total})
}
