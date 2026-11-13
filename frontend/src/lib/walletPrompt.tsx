import type { ReactNode } from 'react'
import { METHOD_NAME } from './format'

// What each wallet does on the customer's phone: M-Pesa and e-Mola show a prompt for
// the PIN, and Bim IZI sends an SMS asking to dial *181#. The example names the
// company being paid, never a bank or broker in between.
export type WalletPrompt = {
  before: string
  title: string
  steps: (phone: ReactNode) => ReactNode
  screen: (amount: string) => ReactNode
  actions: [string, string] | null
  stillWaiting: string
  // Short lines the waiting screen shows one after another while the request is on the phone.
  cues: (phone: string) => string[]
}

const CLOSES = 'este pagamento fecha em até 10 minutos e pode tentar de novo nesta página.'

// company is who the customer pays, shown in the example instead of any bank or
// broker in between.
export function walletPrompt(method: string, company = 'a loja'): WalletPrompt {
  const who = <b style={{ color: 'var(--ink)' }}>{company}</b>
  if (method === 'mpesa') {
    return {
      before: 'Vamos enviar ao M-Pesa um pedido para o seu telemóvel. Digite o PIN M-Pesa para confirmar.',
      title: 'Pedido enviado ao M-Pesa',
      steps: phone => <>Enviámos o pedido ao M-Pesa para {phone}. Pode demorar alguns segundos a chegar. Confirme o pagamento a {who} com o seu PIN M-Pesa.</>,
      screen: amount => <>Vais pagar <b>{amount} MT</b> a {company}. Digita o teu PIN M-Pesa para confirmar</>,
      actions: ['Cancelar', 'Responder'],
      stillWaiting: `Ainda sem confirmação do M-Pesa. Se viu um erro no telemóvel, ou o pedido não chegou, ${CLOSES}`,
      cues: phone => [`Confirme no telemóvel ${phone}`, `Pagamento a ${company}`, 'Digite o seu PIN M-Pesa para pagar', 'Se o M-Pesa falhar, enviamos um novo pedido'],
    }
  }
  if (method === 'emola') {
    return {
      before: 'Vamos enviar ao e-Mola um pedido para o seu telemóvel. Digite o PIN e-Mola para confirmar. Tem até 5 minutos.',
      title: 'Pedido enviado ao e-Mola',
      steps: phone => <>Enviámos o pedido ao e-Mola para {phone}. Pode demorar alguns segundos a chegar, e tem até 5 minutos. Confirme o pagamento a {who} com o seu PIN e-Mola.</>,
      screen: amount => <>Pagamento de <b>{amount} MT</b> a {company}. Introduza o PIN e-Mola para confirmar</>,
      actions: ['Cancelar', 'Confirmar'],
      stillWaiting: `Ainda sem confirmação do e-Mola. Se viu um erro no telemóvel, ou o pedido não chegou, ${CLOSES}`,
      cues: phone => [`Confirme no telemóvel ${phone}`, `Pagamento a ${company}`, 'Digite o seu PIN e-Mola para pagar', 'Tem até 5 minutos para responder'],
    }
  }
  if (method === 'bim') {
    return {
      before: 'Vai receber um SMS. Depois marque *181# no telemóvel para confirmar o pagamento. Tem até 5 minutos.',
      title: 'Marque *181# no seu telemóvel',
      steps: phone => <>O SMS para {phone} pode demorar alguns segundos a chegar, e tem até 5 minutos. Marque <b style={{ color: 'var(--ink)' }}>*181#</b> e confirme o pagamento a {who}.</>,
      screen: amount => <>Confirme o pagamento a {company}<br />Valor: <b>{amount} MZN</b><br />1 - Confirmar<br />0 - Cancelar</>,
      actions: null,
      stillWaiting: `Ainda sem confirmação. Se ainda não marcou, marque *181#. Se viu um erro no telemóvel, ${CLOSES}`,
      cues: phone => [`Vai receber um SMS no ${phone}`, 'Marque *181# no telemóvel', `Confirme o pagamento a ${company}`, 'Tem até 5 minutos para confirmar'],
    }
  }
  const name = METHOD_NAME[method] ?? method
  return {
    before: 'Vai receber um pedido no telemóvel para confirmar o pagamento. A internet pode falhar enquanto o pedido está aberto: mostramos o resultado assim que voltar.',
    title: `Pedido enviado ao ${name}`,
    steps: phone => <>O pedido para {phone} pode demorar alguns segundos a chegar, e tem até 5 minutos para responder. Confirme o pagamento a {who} no telemóvel.</>,
    screen: amount => <>{name}: confirme o pagamento de <b>{amount} MT</b> a {company}.</>,
    actions: null,
    stillWaiting: `Ainda sem confirmação. Se viu um erro no telemóvel, ou o pedido não chegou, ${CLOSES}`,
    cues: phone => [`Confirme no telemóvel ${phone}`, `Pagamento a ${company}`, `Siga as instruções de ${name} no telemóvel`, 'Tem até 5 minutos para responder'],
  }
}

// failedText is what the customer is told when a payment failed. When the
// wallet itself failed the transaction, after RIHA tried again, it says so.
export function failedText(p: { method: string; failure_code: string; failure_message: string; tries?: number }): string {
  if (p.failure_code === 'operator_timeout') {
    const name = METHOD_NAME[p.method] ? `O ${METHOD_NAME[p.method]}` : 'A carteira'
    const times = p.tries && p.tries > 1 ? `, mesmo depois de ${p.tries} tentativas` : ''
    return `${name} falhou a transação${times}. Nada foi cobrado. Tente de novo dentro de minutos, ou escolha outro método.`
  }
  return p.failure_message || 'O pagamento falhou.'
}
