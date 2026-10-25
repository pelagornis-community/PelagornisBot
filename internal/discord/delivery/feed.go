package delivery

import (
	"context"
	"encoding/json"
	"github.com/pelagornis/communityos/internal/storage/postgres"
)

func (s Service) HandleFeed(c context.Context, store *postgres.Store, j postgres.Job) error {
	var p struct {
		DeliveryID string `json:"delivery_id"`
		ChannelID  string `json:"channel_id"`
		Content    string `json:"content"`
	}
	if e := json.Unmarshal([]byte(j.Payload), &p); e != nil {
		return e
	}
	id, e := s.Send(c, p.ChannelID, p.Content)
	if x := store.CompleteDelivery(c, p.DeliveryID, id, e); x != nil {
		return x
	}
	return e
}
