package helper

import (
	"context"
	"time"

	"github.com/gravypower/dd"
	ddapi "github.com/gravypower/dd/api"
	"github.com/sirupsen/logrus"
)

// LoopMessages loops over messages, fetching every few seconds and emitting to the channel.
// It terminates if and when the context is stopped, or returns an error if messages
// can no longer be fetched. Messages that are not a door status are skipped.
func LoopMessages(ctx context.Context, conn *dd.Conn, ch chan<- ddapi.DoorStatus) error {
	for {
		messages, err := conn.Messages()
		if err != nil {
			return err
		}
		for _, m := range messages {
			var out ddapi.DoorStatus
			if err := m.Decode(&out); err != nil {
				// One undecodable message must not kill the loop.
				logrus.WithError(err).WithField("type", m.Type).Warn("Skipping message that is not a door status")
				continue
			}
			select {
			case ch <- out:
			case <-ctx.Done():
				return nil
			}
		}

		timer := time.NewTimer(time.Second * 2)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil
		}
	}
}
