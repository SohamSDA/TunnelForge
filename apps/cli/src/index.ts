import { WebSocket } from 'ws'
import http from 'http'
import { MessageType, TunnelMessage } from '@tunnelforge/protocol'

// Read port from command line argument
// Usage: ts-node src/index.ts 3000
const LOCAL_PORT = parseInt(process.argv[2] || '3000')
const RELAY_URL = 'ws://localhost:8080/tunnel'

let ws: WebSocket
let reconnectDelay = 1000
const MAX_DELAY = 60000

function connect() {
  console.log(`Connecting to relay server...`)
  ws = new WebSocket(RELAY_URL)

  ws.on('open', () => {
    console.log('Connected to relay server')
    reconnectDelay = 1000  // reset backoff on successful connection

    // Register our tunnel immediately on connect
    ws.send(JSON.stringify({
      type: MessageType.REGISTER,
      apiKey: 'dev-key'  // hardcoded for now, will be real later
    }))

    // Start heartbeat — send ping every 30 seconds
    startHeartbeat()
  })

  ws.on('message', async (data: Buffer) => {
    const message = JSON.parse(data.toString()) as TunnelMessage

    switch (message.type) {

      case MessageType.REGISTERED: {
        // Relay confirmed our tunnel
        console.log(`\n✓ Tunnel active!`)
        console.log(`  Forwarding: ${message.publicUrl} → localhost:${LOCAL_PORT}`)
        console.log(`\nReady to receive requests...\n`)
        break
      }

      case MessageType.HTTP_REQUEST: {
        // Relay is forwarding a public request to us
        // We need to forward it to our local app and send back the response
        console.log(`→ ${message.method} ${message.path}`)
        
        try {
          const response = await forwardToLocal(message)
          
          ws.send(JSON.stringify({
            type: MessageType.HTTP_RESPONSE,
            requestId: message.requestId,
            status: response.status,
            headers: response.headers,
            body: response.body
          }))

          console.log(`← ${response.status} ${message.path}`)

        } catch (err) {
          // Local app is probably not running
          console.error(`Local app error:`, err)
           
          ws.send(JSON.stringify({
            type: MessageType.HTTP_RESPONSE,
            requestId: message.requestId,
            status: 502,
            headers: {},
            body: `Could not reach localhost:${LOCAL_PORT}. Is your app running?`
          }))
        }
        break
      }

      case MessageType.HEARTBEAT_ACK: {
        // Server confirmed it's alive, nothing to do
        break
      }

      case MessageType.ERROR: {
        console.error('Error from relay:', message.message)
        break
      }
    }
  })

  ws.on('close', () => {
    console.log(`Disconnected. Reconnecting in ${reconnectDelay / 1000}s...`)
    setTimeout(connect, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 2, MAX_DELAY)
  })

  ws.on('error', (err) => {
    console.error('Connection error:', err.message)
    // 'close' event fires after 'error', so reconnect logic runs there
  })
}

// Forward an incoming request to our local app
async function forwardToLocal(message: any): Promise<{
  status: number
  headers: Record<string, string>
  body: string
}> {
  return new Promise((resolve, reject) => {
    
    const options = {
      hostname: 'localhost',
      port: LOCAL_PORT,
      path: message.path,
      method: message.method,
      headers: {
        ...message.headers,
        host: `localhost:${LOCAL_PORT}`  // override host header
      }
    }

    const req = http.request(options, (res) => {
      let body = ''
      
      res.on('data', chunk => body += chunk)
      
      res.on('end', () => {
        resolve({
          status: res.statusCode || 200,
          headers: res.headers as Record<string, string>,
          body
        })
      })
    })

    req.on('error', reject)

    // Send request body if present
    if (message.body) {
      req.write(message.body)
    }

    req.end()
  })
}

// Heartbeat to keep connection alive
function startHeartbeat() {
  const interval = setInterval(() => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type: MessageType.HEARTBEAT,
        timestamp: Date.now()
      }))
    } else {
      clearInterval(interval)
    }
  }, 30000)
}

// Start connecting
connect()