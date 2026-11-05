import type { ReactNode } from 'react'
import { METHOD_NAME } from './format'

// What each wallet really does on the customer's phone, as seen on 2026-11-05 through
// the bim broker: M-Pesa shows a prompt, and IZI sends an SMS asking to dial *181#.
export type WalletPrompt = {
  before: string
  title: string
  steps: (phone: ReactNode) => ReactNode
  screen: (amount: string) => ReactNode
  actions: [string, string] | null
  stillWaiting: string
}

const CLOSES = 'este pagamento fecha em até 10 minutos e pode tentar de novo nesta página.'

export function walletPrompt(method: string): WalletPrompt {
  if (method === 'mpesa') {
    return {
      before: 'Vai receber um pedido no telemóvel para digitar o PIN M-Pesa. Pode demorar até um minuto.',
      title: 'Confirme no seu telemóvel',
      steps: phone => <>O pedido para {phone} pode demorar até um minuto a chegar. Aparece em nome de <b style={{ color: 'var(--ink)' }}>BIM PAY IZI</b>. Digite o PIN M-Pesa para concluir.</>,
      screen: amount => <>Vais pagar <b>{amount} MT</b> na Entidade BIM PAY IZI referente a transacao … Digita o teu PIN</>,
      actions: ['Cancelar', 'Responder'],
      stillWaiting: `Ainda sem confirmação. Se viu um erro no telemóvel, ou o pedido não chegou, ${CLOSES}`,
    }
  }
  if (method === 'bim') {
    return {
      before: 'Vai receber um SMS. Depois marque *181# no telemóvel para confirmar o pagamento.',
      title: 'Marque *181# no seu telemóvel',
      steps: phone => <>O SMS para {phone} pode demorar até um minuto a chegar. Marque <b style={{ color: 'var(--ink)' }}>*181#</b> e escolha <b style={{ color: 'var(--ink)' }}>1 - Confirm</b> para concluir.</>,
      screen: amount => <>PAY IZI PAYMENT<br />Confirm your payment to merchant MASHTECHNO LDA<br />Amount: <b>{amount} MZN</b><br />1 - Confirm<br />0 - Cancel</>,
      actions: null,
      stillWaiting: `Ainda sem confirmação. Se ainda não marcou, marque *181#. Se viu um erro no telemóvel, ${CLOSES}`,
    }
  }
  const name = METHOD_NAME[method] ?? method
  return {
    before: 'Vai receber um pedido no telemóvel para confirmar o pagamento. A internet pode falhar enquanto o pedido está aberto: mostramos o resultado assim que voltar.',
    title: 'Confirme no seu telemóvel',
    steps: phone => <>O pedido para {phone} pode demorar até um minuto a chegar. Siga as instruções de {name} no telemóvel para concluir.</>,
    screen: amount => <>{name}: confirme o pagamento de <b>{amount} MT</b> no telemóvel.</>,
    actions: null,
    stillWaiting: `Ainda sem confirmação. Se viu um erro no telemóvel, ou o pedido não chegou, ${CLOSES}`,
  }
}
