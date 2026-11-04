package postgres

import "riha/internal/core/ports"

type Repositories struct {
	Tx           ports.Tx
	Users        ports.Users
	AuthSessions ports.AuthSessions
	Merchants    ports.Merchants
	Documents    ports.Documents
	Banks        ports.BankAccounts
	Keys         ports.APIKeys
	Links        ports.Links
	Sessions     ports.CheckoutSessions
	Payments     ports.Payments
	Refunds      ports.Refunds
	Ledger       ports.Ledger
	Payouts      ports.Payouts
	Endpoints    ports.Endpoints
	Domains      ports.Domains
	Events       ports.Events
	Deliveries   ports.Deliveries
	RequestLogs  ports.RequestLogs
	Idempotency  ports.Idempotency
	Audit        ports.Audit
	Settings     ports.Settings
	Reports      ports.Reports
}

func (s *Store) Repositories() Repositories {
	return Repositories{
		Tx: s, Users: Users{s}, AuthSessions: AuthSessions{s}, Merchants: Merchants{s}, Documents: Documents{s},
		Banks: Banks{s}, Keys: Keys{s}, Links: Links{s}, Sessions: CheckoutSessions{s}, Payments: Payments{s},
		Refunds: Refunds{s}, Ledger: Ledger{s}, Payouts: Payouts{s}, Endpoints: Endpoints{s}, Domains: Domains{s},
		Events: Events{s}, Deliveries: Deliveries{s}, RequestLogs: RequestLogs{s}, Idempotency: Idempotency{s},
		Audit: Audit{s}, Settings: Settings{s}, Reports: Reports{s},
	}
}
