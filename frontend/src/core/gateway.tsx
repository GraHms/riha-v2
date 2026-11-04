import { createContext, useContext, type ReactNode } from 'react'
import type { Gateway } from './ports'

const C = createContext<Gateway | null>(null)

export function GatewayProvider({ gateway, children }: { gateway: Gateway; children: ReactNode }) {
  return <C.Provider value={gateway}>{children}</C.Provider>
}

export function useGateway(): Gateway {
  const g = useContext(C)
  if (!g) throw new Error('GatewayProvider missing')
  return g
}
