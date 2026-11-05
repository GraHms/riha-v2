import type { Gateway } from '../../core/ports'
import type { CheckoutSession, Json, Me, PaymentFilter, PaymentLink, PublicLink } from '../../core/model'
import { Transport } from './transport'

const qs = (o: Record<string, string | number | undefined>) => new URLSearchParams(Object.entries(o).filter(([, v]) => v !== undefined && v !== '').map(([k, v]) => [k, String(v)])).toString()
const paymentQuery = (f: PaymentFilter) => qs({ status: f.status === 'all' ? '' : f.status, q: f.q, limit: f.limit })

export function httpGateway(t = new Transport()): Gateway {
  const get = <T = Json>(p: string) => t.request<T>(p)
  const post = <T = Json>(p: string, body?: unknown) => t.request<T>(p, { method: 'POST', body })
  const send = <T = Json>(method: string, p: string, body?: unknown) => t.request<T>(p, { method, body })
  return {
    mode: { get: t.getMode, set: t.setMode },
    onUnauthorized: t.onUnauthorized,
    auth: {
      me: () => get<Me>('/auth/me'),
      login: (email, password) => post<Me>('/auth/login', { email, password }),
      signup: input => post<Me>('/auth/signup', input),
      logout: async () => { await post('/auth/logout') },
      changePassword: async (current_password, new_password) => { await post('/auth/password', { current_password, new_password }) },
      forgotPassword: async email => { await post('/auth/forgot', { email }) },
      resetPassword: async (token, password) => { await post('/auth/reset', { token, password }) },
    },
    merchant: {
      updateProfile: patch => send('PATCH', '/merchant', patch),
      kyc: () => get('/merchant/kyc'),
      saveKyc: input => post('/merchant/kyc', input),
      uploadDocument: (kind, file) => { const fd = new FormData(); fd.append('kind', kind); fd.append('file', file); return t.request('/merchant/kyc/documents', { form: fd }) },
      submitKyc: () => post('/merchant/kyc/submit'),
      bankAccount: () => get('/bank-account'),
      saveBankAccount: input => send('PUT', '/bank-account', input),
    },
    payments: {
      overview: days => get(`/overview?days=${days}`),
      list: f => get('/payments?' + paymentQuery(f)),
      exportUrl: f => t.url('/payments/export.csv?' + paymentQuery(f)),
      get: id => get('/payments/' + encodeURIComponent(id)),
      refund: (id, amount, reason) => post(`/payments/${encodeURIComponent(id)}/refund`, { amount, reason }),
      sendReceipt: (id, email) => post(`/payments/${encodeURIComponent(id)}/receipt`, { email: email ?? '' }),
    },
    links: {
      list: () => get<{ data: PaymentLink[] }>('/links'),
      create: input => post<PaymentLink>('/links', input),
      update: (id, patch) => send<PaymentLink>('PATCH', '/links/' + encodeURIComponent(id), patch),
    },
    payouts: {
      balance: () => get('/balance'),
      list: () => get('/payouts'),
      request: amount => post('/payouts', { amount }),
      statementUrl: id => t.url(`/payouts/${encodeURIComponent(id)}/statement.pdf`),
      receiptUrl: id => t.url(`/payouts/${encodeURIComponent(id)}/receipt`),
    },
    developers: {
      keys: () => get('/keys'),
      rollSecretKey: () => post('/keys/roll'),
      requestLogs: () => get('/api-logs'),
      settings: () => get('/developers/settings'),
      saveRedirects: (success_url, cancel_url) => send('PUT', '/developers/redirects', { success_url, cancel_url }),
      createEndpoint: (url, enabled_events) => post('/developers/endpoints', { url, enabled_events }),
      updateEndpoint: (id, patch) => send('PATCH', '/developers/endpoints/' + encodeURIComponent(id), patch),
      deleteEndpoint: id => send('DELETE', '/developers/endpoints/' + encodeURIComponent(id)),
      addDomain: domain => post('/developers/domains', { domain }),
      removeDomain: id => send('DELETE', '/developers/domains/' + encodeURIComponent(id)),
      deliveries: () => get('/developers/deliveries'),
      resendDelivery: async id => { await post(`/developers/deliveries/${encodeURIComponent(id)}/resend`) },
      sendTestEvent: async () => { await post('/developers/test-event') },
      signingSecret: async () => (await get<{ secret: string }>('/developers/secret')).secret,
    },
    checkout: {
      // A status check is small and repeats: a slow one is better given up and tried again.
      session: (id, signal) => t.request<CheckoutSession>('/public/checkout/' + encodeURIComponent(id), { timeoutMs: 8_000, signal }),
      pay: (id, input) => post<CheckoutSession>(`/public/checkout/${encodeURIComponent(id)}/pay`, input),
      link: (m, s) => get<PublicLink>(`/public/links/${encodeURIComponent(m)}/${encodeURIComponent(s)}`),
      openLink: (m, s, amount, channel) => post<{ id: string }>(`/public/links/${encodeURIComponent(m)}/${encodeURIComponent(s)}/sessions`, { amount, channel }),
      pricing: () => get<{ default_fee_bps: number; coming_soon?: string[] | null }>('/public/pricing'),
    },
    subscriptions: {
      overview: () => get('/subscriptions/overview'),
      settings: () => get('/subscriptions/settings'),
      saveSettings: input => send('PUT', '/subscriptions/settings', input),
      plans: () => get('/subscriptions/plans'),
      plan: id => get('/subscriptions/plans/' + encodeURIComponent(id)),
      createPlan: input => post('/subscriptions/plans', input),
      updatePlan: (id, input) => send('PATCH', '/subscriptions/plans/' + encodeURIComponent(id), input),
      changePrice: (id, amount, scope, subscriptions = []) => post(`/subscriptions/plans/${encodeURIComponent(id)}/price`, { amount, scope, subscriptions }),
      archivePlan: id => post(`/subscriptions/plans/${encodeURIComponent(id)}/archive`),
      catalog: () => get('/subscriptions/catalog'),
      saveCatalog: (entries, multi, page) => send('PUT', '/subscriptions/catalog', { entries, multi, page }),
      checkCatalogSlug: value => get<{ ok: boolean; slug: string; message?: string }>('/subscriptions/catalog/slug?' + qs({ value })),
      uploadCover: file => { const fd = new FormData(); fd.append('file', file); return t.request('/subscriptions/catalog/cover', { form: fd }) },
      removeCover: () => send('DELETE', '/subscriptions/catalog/cover'),
      members: f => get('/subscriptions/members?' + qs(f)),
      membersExportUrl: f => t.url('/subscriptions/members/export.csv?' + qs(f)),
      invite: input => post('/subscriptions/members', input),
      previewImport: (file, planId) => { const fd = new FormData(); fd.append('file', file); fd.append('plan_id', planId); fd.append('dry_run', 'true'); return t.request('/subscriptions/members/import', { form: fd }) },
      importMembers: (file, planId) => { const fd = new FormData(); fd.append('file', file); fd.append('plan_id', planId); return t.request('/subscriptions/members/import', { form: fd }) },
      resendInvites: () => post('/subscriptions/members/resend-invites'),
      member: id => get('/subscriptions/members/' + encodeURIComponent(id)),
      updateMember: (id, input) => send('PATCH', '/subscriptions/members/' + encodeURIComponent(id), input),
      resend: id => post(`/subscriptions/members/${encodeURIComponent(id)}/resend`),
      markPaidOutside: (id, i) => {
        const fd = new FormData(); fd.append('charge_ids', i.charge_ids.join(',')); fd.append('method', i.method); fd.append('date', i.date); fd.append('reference', i.reference)
        if (i.proof) fd.append('proof', i.proof)
        return t.request(`/subscriptions/members/${encodeURIComponent(id)}/paid-outside`, { form: fd })
      },
      proofUrl: id => t.url(`/subscriptions/charges/${encodeURIComponent(id)}/proof`),
      action: (id, action, body = {}) => post(`/subscriptions/subs/${encodeURIComponent(id)}/${action}`, body),
    },
    publicSubs: {
      plan: (m, s) => get(`/public/subscriptions/plans/${encodeURIComponent(m)}/${encodeURIComponent(s)}`),
      catalog: m => get('/public/subscriptions/catalog/' + encodeURIComponent(m)),
      agreement: (m, plans, phone, channel) => get<{ texts: string[] }>(`/public/subscriptions/agreement/${encodeURIComponent(m)}?` + qs({ plans: plans.join(','), phone, channel })),
      subscribe: (m, input) => post<{ token: string }>('/public/subscriptions/subscribe/' + encodeURIComponent(m), input),
      invoice: tok => get('/public/invoices/' + encodeURIComponent(tok)),
      pay: (tok, input) => post(`/public/invoices/${encodeURIComponent(tok)}/pay`, input),
    },
    admin: {
      overview: () => get('/admin/overview'),
      approvals: tab => get('/admin/approvals?' + qs({ tab: tab === 'leads' ? 'leads' : '' })),
      decide: (id, decision, note) => post(`/admin/merchants/${encodeURIComponent(id)}/decision`, { decision, note }),
      merchants: (status, q) => get('/admin/merchants?' + qs({ status, q })),
      merchant: id => get('/admin/merchants/' + encodeURIComponent(id)),
      setFee: (id, bps) => send('PATCH', '/admin/merchants/' + encodeURIComponent(id), bps === null ? { reset_fee: true } : { fee_bps: bps }),
      documentUrl: id => t.url('/admin/documents/' + encodeURIComponent(id)),
      payouts: tab => get('/admin/payouts?' + qs({ tab })),
      payoutAction: (ids, action, reason = '') => post('/admin/payouts/action', { ids, action, reason }),
      payoutExportUrl: ids => t.url('/admin/payouts/export.csv' + (ids.length ? '?ids=' + ids.join(',') : '')),
      payoutInstructionsUrl: ids => t.url('/admin/payouts/instructions.pdf' + (ids.length ? '?ids=' + ids.join(',') : '')),
      completePayouts: (ids, receipt) => { const fd = new FormData(); fd.append('ids', ids.join(',')); fd.append('receipt', receipt); return t.request('/admin/payouts/complete', { form: fd }) },
      payoutStatementUrl: id => t.url(`/admin/payouts/${encodeURIComponent(id)}/statement.pdf`),
      payoutReceiptUrl: id => t.url(`/admin/payouts/${encodeURIComponent(id)}/receipt`),
      emailPayoutStatement: id => post<{ to: string }>(`/admin/payouts/${encodeURIComponent(id)}/email`),
      settings: () => get('/admin/settings'),
      saveSettings: default_fee_bps => send('PUT', '/admin/settings', { default_fee_bps }),
    },
  }
}
