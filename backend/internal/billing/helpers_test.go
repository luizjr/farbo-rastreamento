package billing

import "github.com/pedrofarbo/farbo-rastreamento/backend/internal/config"

func configWithSuspension(days int) config.Billing {
	return config.Billing{SuspendAfterDays: days, InvoiceLeadDays: 10, Timezone: "UTC"}
}
