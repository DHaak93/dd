package helper

import (
	"context"
	"log"
	"time"

	"github.com/gravypower/dd"
	ddapi "github.com/gravypower/dd/api"
)

// LoopMessages loops over messages, fetching every few seconds and emitting to the channel.
// It terminates if and when the context is stopped.
func LoopMessages(ctx context.Context, conn *dd.Conn, ch chan<- ddapi.DoorStatus) error {
	for {
		messages, err := conn.Messages()
		if err != nil {
			return err
		}
		for _, m := range messages {
			var out ddapi.DoorStatus
			if err := m.Decode(&out); err != nil {
				// Skip a message we can't decode rather than ending the loop
				// (which stops the bridge).
				log.Printf("skipping undecodable message (type %d): %v", m.Type, err)
				continue
			}
			// Try to send all messages in case we got multiple.
			ch <- out
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
