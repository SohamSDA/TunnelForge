import express from 'express'
import { WebSocketServer, WebSocket } from 'ws'
import { createServer } from 'http'
import { MessageType, TunnelMessage, RegisterMessage } from '@tunnelforge/protocol'
import { TunnelRegistry } from './registry'
import { handlePublicRequest } from './handler'

const app = express()
app.use(express.json())
app.use(express.text({ type: '*/*' }))  


const server = createServer(app)
const wss = new WebSocketServer({ server, path: '/tunnel' })


const registry = new TunnelRegistry()

// --- WebSocket Handler ---
// This runs every time a CLI client connects
wss.on('connection', (ws: WebSocket) => {
  console.log('CLI client connected')
  
  let assignedSubdomain: string | null = null

  ws.on('message', (data: Buffer) => {
    let message: TunnelMessage
    
    try {
      message = JSON.parse(data.toString()) as TunnelMessage
    } catch {
      console.error('Invalid message received, ignoring')
      return
    }

    switch (message.type) {

      case MessageType.REGISTER: {
        // CLI wants to create a tunnel
        // For now we ignore the API key — we'll add auth later
        // Generate a random subdomain and register it
        const subdomain = registry.register(ws)
        assignedSubdomain = subdomain

        const publicUrl = `http://${subdomain}.localhost:8080`
        // In production this will be https://${subdomain}.tunnelforge.xyz

        ws.send(JSON.stringify({
          type: MessageType.REGISTERED,
          subdomain,
          publicUrl
        }))

        console.log(`Tunnel registered: ${subdomain} → CLI client`)
        break
      }

      case MessageType.HEARTBEAT: {
        // CLI is checking connection is alive
        // We echo back immediately
        ws.send(JSON.stringify({
          type: MessageType.HEARTBEAT_ACK,
          timestamp: message.timestamp
        }))
        break
      }

      case MessageType.HTTP_RESPONSE: {
        // CLI has forwarded our request to localhost and got a response
        // We need to find the pending HTTP request waiting for this response
        // and resolve it
        registry.resolveRequest(message.requestId, message)
        break
      }

    }
  })

  ws.on('close', () => {
    // CLI disconnected — clean up registry
    if (assignedSubdomain) {
      registry.unregister(assignedSubdomain)
      console.log(`Tunnel closed: ${assignedSubdomain}`)
    }
  })

  ws.on('error', (err) => {
    console.error('WebSocket error:', err)
  })
})

app.use(async (req, res) => {
  await handlePublicRequest(req, res, registry)
})

const PORT = 8080
server.listen(PORT, () => {
  console.log(`Relay server running on port ${PORT}`)
})