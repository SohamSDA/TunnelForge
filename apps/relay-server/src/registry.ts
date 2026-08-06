import { WebSocket } from 'ws'
import { HttpResponseMessage } from '@tunnelforge/protocol'


interface PendingRequest {
  resolve: (response: HttpResponseMessage) => void
  reject: (error: Error) => void
  timer: NodeJS.Timeout
}

export class TunnelRegistry {
  
  private tunnels = new Map<string, WebSocket>()
  private pendingRequests = new Map<string, PendingRequest>()

  register(ws: WebSocket): string {
    const subdomain = this.generateSubdomain()
    this.tunnels.set(subdomain, ws)
    return subdomain
  }

  unregister(subdomain: string): void {
    this.tunnels.delete(subdomain)
  }

  getConnection(subdomain: string): WebSocket | undefined {
    return this.tunnels.get(subdomain)
  }

  waitForResponse(requestId: string, timeoutMs = 30000): Promise<HttpResponseMessage> {
    return new Promise((resolve, reject) => {
      
   
      const timer = setTimeout(() => {
        this.pendingRequests.delete(requestId)
        reject(new Error('Request timed out'))
      }, timeoutMs)

      this.pendingRequests.set(requestId, { resolve, reject, timer })
    })
  }


  resolveRequest(requestId: string, response: HttpResponseMessage): void {
    const pending = this.pendingRequests.get(requestId)
    if (!pending) {
      console.warn(`No pending request found for ID: ${requestId}`)
      return
    }
    clearTimeout(pending.timer)
    this.pendingRequests.delete(requestId)
    pending.resolve(response)
  }

  private generateSubdomain(): string {
    return Math.random().toString(36).substring(2, 10)
  }
}