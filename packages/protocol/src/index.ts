
export enum MessageType {

  HEARTBEAT = 'heartbeat',
  HTTP_RESPONSE = 'http_response',
  HEARTBEAT_ACK = 'heartbeat_ack',
  HTTP_REQUEST = 'http_request',
  ERROR = 'error'
}

export interface HttpRequestMessage {
  type: MessageType.HTTP_REQUEST
  requestId: string        
  method: string         
  path: string            
  headers: Record<string, string>
  body: string | null
}

export interface HttpResponseMessage {
  type: MessageType.HTTP_RESPONSE
  requestId: string       
  status: number           
  headers: Record<string, string>
  body: string | null
}

export interface HeartbeatMessage {
  type: MessageType.HEARTBEAT
  timestamp: number
}

export interface HeartbeatAckMessage {
  type: MessageType.HEARTBEAT_ACK
  timestamp: number
}

export interface ErrorMessage {
  type: MessageType.ERROR
  message: string
}

export type TunnelMessage =
   HttpRequestMessage
  | HttpResponseMessage
  | HeartbeatMessage
  | HeartbeatAckMessage
  | ErrorMessage