import type { CheckoutSession, Json, Me, Mode, PaymentFilter, PaymentLink, PublicLink, SignupInput } from './model'

export interface AuthPort {
  me(): Promise<Me>
  login(email: string, password: string): Promise<Me>
  signup(input: SignupInput): Promise<Me>
  logout(): Promise<void>
  changePassword(current: string, next: string): Promise<void>
}

export interface MerchantPort {
  updateProfile(patch: Json): Promise<Json>
  kyc(): Promise<Json>
  saveKyc(input: Json): Promise<Json>
  uploadDocument(kind: string, file: File): Promise<Json>
  submitKyc(): Promise<Json>
  bankAccount(): Promise<Json>
  saveBankAccount(input: { bank_name: string; holder: string; nib: string }): Promise<Json>
}

export interface PaymentsPort {
  overview(days: number): Promise<Json>
  list(filter: PaymentFilter): Promise<Json>
  exportUrl(filter: PaymentFilter): string
  get(id: string): Promise<Json>
  refund(id: string, amount: number, reason: string): Promise<Json>
  sendReceipt(id: string): Promise<{ message: string }>
}

export interface LinksPort {
  list(): Promise<{ data: PaymentLink[] }>
  create(input: Json): Promise<PaymentLink>
  update(id: string, patch: Json): Promise<PaymentLink>
}

export interface PayoutsPort {
  balance(): Promise<Json>
  list(): Promise<Json>
  request(amount: number): Promise<Json>
}

export interface DevelopersPort {
  keys(): Promise<Json>
  rollSecretKey(): Promise<Json>
  requestLogs(): Promise<Json>
  settings(): Promise<Json>
  saveRedirects(success_url: string, cancel_url: string): Promise<Json>
  createEndpoint(url: string, events: string[]): Promise<Json>
  updateEndpoint(id: string, patch: Json): Promise<Json>
  deleteEndpoint(id: string): Promise<Json>
  addDomain(domain: string): Promise<Json>
  removeDomain(id: string): Promise<Json>
  deliveries(): Promise<Json>
  resendDelivery(id: string): Promise<void>
  sendTestEvent(): Promise<void>
  signingSecret(): Promise<string>
}

export interface CheckoutPort {
  session(id: string): Promise<CheckoutSession>
  pay(id: string, input: Json): Promise<CheckoutSession>
  link(merchant: string, slug: string): Promise<PublicLink>
  openLink(merchant: string, slug: string, amount: number, channel: string): Promise<{ id: string }>
  pricing(): Promise<{ default_fee_bps: number }>
}

export interface AdminPort {
  overview(): Promise<Json>
  approvals(): Promise<Json>
  decide(merchantId: string, decision: string, note: string): Promise<Json>
  merchants(status: string, q: string): Promise<Json>
  merchant(id: string): Promise<Json>
  setFee(id: string, bps: number | null): Promise<Json>
  documentUrl(id: string): string
  payouts(tab: string): Promise<Json>
  payoutAction(ids: string[], action: string, reason?: string): Promise<Json>
  payoutExportUrl(ids: string[]): string
  settings(): Promise<Json>
  saveSettings(defaultFeeBps: number): Promise<Json>
}

export interface Gateway {
  mode: { get(): Mode; set(m: Mode): void }
  auth: AuthPort
  merchant: MerchantPort
  payments: PaymentsPort
  links: LinksPort
  payouts: PayoutsPort
  developers: DevelopersPort
  checkout: CheckoutPort
  admin: AdminPort
  onUnauthorized(fn: () => void): () => void
}
