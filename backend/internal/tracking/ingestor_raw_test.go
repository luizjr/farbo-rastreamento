package tracking

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/config"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/devices"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

func TestRawPayloadOnlyWhenEnabled(t *testing.T) {
	msg := protocols.TrackerMessage{Timestamp: time.Now(), Latitude: -23.5, Longitude: -46.6, RawPayload: "78781F12"}
	dev := &devices.Device{ID: uuid.New()}

	off := &Ingestor{cfg: config.Tracking{}}
	if got := off.buildPosition(dev, msg).RawPayload; got != "" {
		t.Errorf("padrão não devia guardar o pacote bruto, veio %q", got)
	}
	on := &Ingestor{cfg: config.Tracking{StoreRawPayload: true}}
	if got := on.buildPosition(dev, msg).RawPayload; got != "78781F12" {
		t.Errorf("com POSITIONS_STORE_RAW devia guardar, veio %q", got)
	}
}

func TestPositionDropsImpossibleHeading(t *testing.T) {
	ing := &Ingestor{cfg: config.Tracking{}}
	dev := &devices.Device{ID: uuid.New()}
	for course, want := range map[float64]any{0: 0.0, 181.5: 181.5, 360: 360.0, 361: nil, 1023: nil, -1: nil} {
		msg := protocols.TrackerMessage{Timestamp: time.Now(), Latitude: -23.5, Longitude: -46.6, Heading: course}
		got := ing.buildPosition(dev, msg).Heading
		switch {
		case want == nil && got != nil:
			t.Errorf("rumo %v devia virar nulo, veio %v", course, *got)
		case want != nil && (got == nil || *got != want.(float64)):
			t.Errorf("rumo %v devia ser mantido, veio %v", course, got)
		}
	}
}
