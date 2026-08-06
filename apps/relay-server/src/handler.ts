import { Request, Response } from 'express'
import { MessageType, HttpResponseMessage } from '@tunnelforge/protocol'
import { TunnelRegistry } from './registry'
import { randomUUID } from 'crypto'

export async function handlePublicRequest( registry: TunnelRegistry) {
 return async (req: Request, res: Response) => {

 
  const host = req.hostname 
  const subdomain = host.split('.')[0]

  console.log(`Incoming request: ${req.method} ${req.path} ----- tunnel: ${subdomain}`)
  const clientWs = registry.getConnection(subdomain)

  if (!clientWs) {
    res.status(404).json({ error: `No tunnel found for subdomain: ${subdomain}` })
    return
  }

  const requestId = randomUUID()

  const normalizedHeaders: Record<string, string> = {}
  Object.entries(req.headers).forEach(([key, value]) => {
    if (value !== undefined) {
      normalizedHeaders[key] = Array.isArray(value) ? value.join(', ') : value
    }
  })

  clientWs.send(
    JSON.stringify({
      type: MessageType.HTTP_REQUEST,
      requestId,
      method: req.method,
      path: req.originalUrl, 
      headers: normalizedHeaders,
      body: req.body || null,
    }),
  );

  try {
    const response = await registry.waitForResponse(requestId)

    
    res.status(response.status)
    
    Object.entries(response.headers || {}).forEach(([key, value]) => {
      if (!['transfer-encoding', 'connection'].includes(key.toLowerCase())) {
        res.setHeader(key, value)
      }
    })

    res.send(response.body)

  } catch (err) {
    console.error(`Request failed for ${requestId}:`, err)
    res.status(502).send('Tunnel error: ' + (err as Error).message)
  }
}}