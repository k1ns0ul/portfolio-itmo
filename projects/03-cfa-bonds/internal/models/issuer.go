package models

import (
	"time"

	"github.com/google/uuid"
)

type Issuer struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	INN          string    `json:"inn"`
	OGRN         string    `json:"ogrn"`
	ContactEmail string    `json:"contact_email"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}
