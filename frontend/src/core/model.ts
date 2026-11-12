export type Mode = 'live' | 'test'

export type ApiError = { type?: string; code: string; message: string; param?: string; request_id?: string; status: number }

export const errMsg = (e: unknown) => (e as ApiError)?.message || 'Algo correu mal. Tente de novo.'

export type Merchant = {
  id: string; slug: string; name: string; legal_name: string; business_type: string; nuit: string; phone: string; email: string
  city: string; owner_name: string; status: string; fee_bps: number; fee_negotiated: boolean; fee_label: string
  statement_descriptor: string; avatar_color: string; success_url: string; cancel_url: string; sms_notifications: boolean; email_notifications: boolean; coming_soon: string[]; test_coming_soon?: string[]; live_refunds: boolean
  beta?: boolean; min_amount?: number; plan_min_amount?: number
  approved_at: string | null; kyc_submitted_at: string | null; review_note: string; expected_volume: string; created_at: string
}

export type Me = { user: { id: string; email: string; name: string; role: string; title: string; permissions: string[] }; merchant?: Merchant; partner?: { code: string; name: string }; default_fee_bps: number }

// can says whether a staff or partner user has a permission.
export const can = (me: Me | null | undefined, perm: string) => !!me && (me.user.permissions?.includes('*') || me.user.permissions?.includes(perm))

export type PaymentLink = {
  id: string; name: string; slug: string; amount: number | null; custom_amount: boolean; reusable: boolean; url: string; active: boolean
  type: string; payment_method_types: string[]; expires_at: number | null; payments_count: number; total_received: number; livemode: boolean; location?: string
}

export type CheckoutPayment = {
  id: string; reference: string; attempt_id?: string; status: string; method: string; phone_prefix: string; phone_last4: string; card_brand: string; card_last4: string
  failure_code: string; failure_message: string; created_at: string; succeeded_at: string | null; country?: string; redirect_url?: string
}

export type CheckoutSession = {
  id: string; status: string; amount: number; description: string; livemode: boolean
  line_items: { name: string; quantity: number; unit_amount: number }[]; payment_method_types: string[]
  success_url: string; cancel_url: string; return_host: string; client_reference_id: string; customer_phone: string; customer_name: string; email_on_file: boolean; coming_soon?: string[]
  expires_at: string; merchant: { name: string; initials: string; avatar_color: string; descriptor: string }
  payment: CheckoutPayment | null
}

export type PublicLink = { name: string; amount: number; custom_amount: boolean; active: boolean; type: string; livemode: boolean; details?: string; image_url?: string; suggested_amount: number; min_amount: number; max_amount: number; submit_type?: string; merchant: { name: string; initials: string; avatar_color: string } }

export type Json = any

export type PaymentFilter = { status?: string; q?: string; limit?: number; link?: string }

export type SignupInput = { business_name: string; name: string; email: string; phone: string; password: string }
