package fulfillment

import (
	"context"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/mail"
)

// MailNotifier manda os e-mails dos marcos pelo ShipmentMailer.
type MailNotifier struct{ Mailer *mail.ShipmentMailer }

func (n MailNotifier) shipment(f *Fulfillment) mail.Shipment {
	return mail.Shipment{
		To: f.CustomerEmail, Name: f.CustomerName, Vehicle: f.VehicleName,
		Carrier: f.ShippingService, TrackingCode: f.TrackingCode,
	}
}

func (n MailNotifier) Shipped(ctx context.Context, f *Fulfillment) error {
	return n.Mailer.Shipped(ctx, n.shipment(f))
}

func (n MailNotifier) Delivered(ctx context.Context, f *Fulfillment) error {
	return n.Mailer.Delivered(ctx, n.shipment(f))
}
