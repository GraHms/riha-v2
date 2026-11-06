import { useState } from 'react'
import { Drawer, Spinner } from '../../components/ui'
import { SharePanel, useLinkForm, type LinkObj } from './NewLink'

export default function NewLinkDrawer({ onClose }: { onClose: () => void }) {
  const [done, setDone] = useState<LinkObj | null>(null)
  const f = useLinkForm(setDone)
  if (done) return <Drawer title="Link criado" onClose={onClose}><div style={{ padding: '28px 26px' }}><SharePanel link={done} compact /></div></Drawer>
  return (
    <Drawer title="Novo link de pagamento" onClose={onClose} footer={<>
      <button className="btn btn-outline" style={{ flex: 1, height: 50, borderRadius: 14, fontSize: 15 }} onClick={onClose}>Cancelar</button>
      <button className="btn btn-green" style={{ flex: 2, height: 50, borderRadius: 14, fontSize: 15 }} onClick={f.submit} disabled={f.busy || !f.valid}>{f.busy ? <Spinner /> : 'Criar link'}</button>
    </>}>
      <div style={{ padding: '20px 26px 24px', display: 'flex', flexDirection: 'column', gap: 16 }}>
        {f.tabs}
        {f.tab === 'edit' ? (
          <>
            <div className="card" style={{ borderRadius: 18, padding: 18 }}>{f.amountBlock(false)}</div>
            {f.fields}
          </>
        ) : f.preview}
      </div>
    </Drawer>
  )
}
