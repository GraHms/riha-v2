import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './styles.css'
import App from './App'
import { SessionProvider } from './lib/session'
import { GatewayProvider } from './core/gateway'
import { httpGateway } from './adapters/http/gateway'

const gateway = httpGateway()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <GatewayProvider gateway={gateway}>
      <SessionProvider>
        <App />
      </SessionProvider>
      </GatewayProvider>
    </BrowserRouter>
  </StrictMode>,
)
