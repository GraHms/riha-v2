import type { CheckoutSession, Json, Me, Mode, PaymentFilter, PaymentLink, PublicLink, SignupInput } from './model'

export interface AuthPort {
  me(): Promise<Me>
  login(email: string, password: string): Promise<Me>
  signup(input: SignupInput): Promise<Me>
  logout(): Promise<void>
  changePassword(current: string, next: string): Promise<void>
  forgotPassword(email: string): Promise<void>
  resetPassword(token: string, password: string): Promise<void>
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
  sendReceipt(id: string, email?: string): Promise<{ message: string }>
}

export interface LinksPort {
  list(): Promise<{ data: PaymentLink[] }>
  get(id: string): Promise<PaymentLink>
  create(input: Json): Promise<PaymentLink>
  update(id: string, patch: Json): Promise<PaymentLink>
  uploadImage(id: string, file: File): Promise<PaymentLink>
  removeImage(id: string): Promise<PaymentLink>
}

export interface PayoutsPort {
  balance(): Promise<Json>
  list(): Promise<Json>
  request(amount: number): Promise<Json>
  statementUrl(id: string): string
  receiptUrl(id: string): string
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
  session(id: string, signal?: AbortSignal): Promise<CheckoutSession>
  pay(id: string, input: Json): Promise<CheckoutSession>
  link(merchant: string, slug: string): Promise<PublicLink>
  openLink(merchant: string, slug: string, amount: number, channel: string): Promise<{ id: string }>
  pricing(): Promise<{ default_fee_bps: number; coming_soon?: string[] | null }>
}

export interface AdminPort {
  overview(): Promise<Json>
  trailSearch(q: string): Promise<Json>
  payments(f: { q?: string; view?: string; method?: string; merchant?: string; mode?: string; page?: number }): Promise<Json>
  checkPayment(id: string, apply: boolean): Promise<Json>
  trailThread(id: string): Promise<Json>
  trailEvents(filter: Record<string, string | number | undefined>): Promise<Json>
  metrics(days: number, live: boolean, merchant?: string): Promise<Json>
  approvals(tab: 'review' | 'leads', o?: { q?: string; sort?: string; page?: number }): Promise<Json>
  decide(merchantId: string, decision: string, note: string): Promise<Json>
  merchants(status: string, q: string): Promise<Json>
  merchant(id: string): Promise<Json>
  setFee(id: string, bps: number | null): Promise<Json>
  setBeta(id: string, beta: boolean): Promise<Json>
  documentUrl(id: string): string
  payouts(tab: string): Promise<Json>
  payoutAction(ids: string[], action: string, reason?: string): Promise<Json>
  payoutExportUrl(ids: string[]): string
  payoutInstructionsUrl(ids: string[]): string
  completePayouts(ids: string[], receipt: File): Promise<Json>
  payoutStatementUrl(id: string): string
  payoutReceiptUrl(id: string): string
  emailPayoutStatement(id: string): Promise<{ to: string }>
  settings(): Promise<Json>
  saveSettings(defaultFeeBps: number): Promise<Json>
  saveTransferFee(fee: number, freeFrom: number): Promise<Json>
  integrations(): Promise<Json>
  editIntegration(code: string, patch: Json): Promise<Json>
  routing(merchantId: string): Promise<Json>
  setRoute(merchantId: string, method: string, integration: string): Promise<Json>
  setDeal(merchantId: string, code: string, shareBps: number | null): Promise<Json>
  settlement(from?: string, to?: string): Promise<Json>
  settlementExportUrl(from?: string, to?: string): string
  recordSettlement(input: { integration: string; amount: number; reference: string; note: string; received_on: string }): Promise<Json>
  deleteSettlement(id: string): Promise<Json>
  team(): Promise<Json>
  invite(input: Json): Promise<Json>
  partners(): Promise<Json>
  createPartner(input: Json): Promise<Json>
  renamePartner(code: string, input: Json): Promise<Json>
  invitePartnerUser(code: string, input: Json): Promise<Json>
  setAccess(userId: string, patch: Json): Promise<Json>
  resendInvite(userId: string): Promise<Json>
  payFunding(id: string, reference: string, receipt: File): Promise<Json>
  fundingReceiptUrl(id: string): string
}

export interface PartnerPort {
  home(from?: string, to?: string): Promise<Json>
  payouts(tab: string): Promise<Json>
  payFunding(id: string, reference: string, receipt: File): Promise<Json>
  fundingReceiptUrl(id: string): string
  team(): Promise<Json>
  invite(input: Json): Promise<Json>
  setAccess(userId: string, patch: Json): Promise<Json>
  resendInvite(userId: string): Promise<Json>
}

export interface SubscriptionsPort {
  overview(): Promise<Json>
  settings(): Promise<Json>
  saveSettings(input: Json): Promise<Json>
  plans(): Promise<Json>
  plan(id: string): Promise<Json>
  createPlan(input: Json): Promise<Json>
  updatePlan(id: string, input: Json): Promise<Json>
  changePrice(id: string, amount: number, scope: string, subscriptions?: string[]): Promise<Json>
  archivePlan(id: string): Promise<Json>
  catalog(): Promise<Json>
  saveCatalog(entries: { plan_id: string; in_catalog: boolean }[], multi: boolean, page?: Json): Promise<Json>
  checkCatalogSlug(value: string): Promise<{ ok: boolean; slug: string; message?: string }>
  uploadCover(file: File): Promise<Json>
  removeCover(): Promise<Json>
  members(filter: Record<string, string | number | undefined>): Promise<Json>
  membersExportUrl(filter: Record<string, string | number | undefined>): string
  invite(input: Json): Promise<Json>
  previewImport(file: File, planId: string): Promise<Json>
  importMembers(file: File, planId: string): Promise<Json>
  resendInvites(): Promise<Json>
  member(id: string): Promise<Json>
  updateMember(id: string, input: Json): Promise<Json>
  resend(id: string): Promise<Json>
  markPaidOutside(id: string, input: { charge_ids: string[]; method: string; date: string; reference: string; proof?: File | null }): Promise<Json>
  proofUrl(chargeId: string): string
  action(subId: string, action: 'skip' | 'pause' | 'resume' | 'cancel' | 'change-plan' | 'send-now', body?: Json): Promise<Json>
}

export interface PublicSubscriptionsPort {
  plan(merchant: string, slug: string): Promise<Json>
  catalog(merchant: string): Promise<Json>
  agreement(merchant: string, planIds: string[], phone: string, channel: string): Promise<{ texts: string[] }>
  subscribe(merchant: string, input: Json): Promise<{ token: string }>
  invoice(token: string): Promise<Json>
  pay(token: string, input: Json): Promise<Json>
  member(token: string): Promise<Json>
  cancel(token: string, subscriptionId: string): Promise<Json>
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
  partner: PartnerPort
  subscriptions: SubscriptionsPort
  publicSubs: PublicSubscriptionsPort
  onUnauthorized(fn: () => void): () => void
}
