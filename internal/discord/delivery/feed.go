package delivery

import (
	"context"
	"encoding/json"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

func (s Service) HandleFeed(c context.Context, store *sqlite.Store, j sqlite.Job) error {
	var p struct {
		DeliveryID string `json:"delivery_id"`
		ChannelID  string `json:"channel_id"`
		Content    string `json:"content"`
		ImageURL   string `json:"image_url"`
	}
	if e := json.Unmarshal([]byte(j.Payload), &p); e != nil {
		return e
	}
	id, e := s.SendWithImage(c, p.ChannelID, p.Content, p.ImageURL)
	if x := store.CompleteDelivery(c, p.DeliveryID, id, e); x != nil {
		return x
	}
	return e
}
