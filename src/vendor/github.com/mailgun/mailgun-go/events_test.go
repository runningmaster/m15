package mailgun

import (
	"testing"
	"time"

	"github.com/facebookgo/ensure"
)

func TestEventIterator(t *testing.T) {
	mg, err := NewMailgunFromEnv()
	ensure.Nil(t, err)

	// Grab the list of events (as many as we can get)
	ei := mg.NewEventIterator()
	err = ei.GetFirstPage(GetEventsOptions{ForceDescending: true})
	ensure.Nil(t, err)

	// Print out the kind of event and timestamp.
	// Specifics about each event will depend on the "event" type.
	events := ei.Events()
	t.Log("Event\tTimestamp\t")
	for _, event := range events {
		t.Logf("%s\t%s\n", event["event"], time.Unix(int64(event["timestamp"].(float64)), 0))
	}
	t.Logf("%d events dumped\n\n", len(events))
	ensure.True(t, len(events) != 0)

	// TODO: (thrawn01) The more I look at this and test it, the more I doubt it will ever work consistently
	// We're on the first page.  We must at the beginning.
	/*ei.GetPrevious()
	if len(ei.Events()) != 0 {
		t.Fatal("Expected to be at the beginning")
	}*/
}
