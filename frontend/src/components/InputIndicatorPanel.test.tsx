import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { InputIndicatorPanel } from './InputIndicatorPanel'
import type { TelemetryData } from '@/hooks/useTelemetry'
import type { VehicleAckData } from '@/hooks/useVehicleAck'

const noTelemetry = null
const noAck = null

const baseTelemetry: TelemetryData = {
  speedKmh: 0,
  batteryPct: 85,
  status: 'MOCK_RUNNING',
  steerCommanded: 0,
  throttleCommanded: 0,
  brakeCommanded: 0,
  steerActual: 0,
  throttleActual: 0,
  timestampMs: Date.now() - 800,
  ageSinceUpdateMs: 800,
}

const connectedAck: VehicleAckData = {
  commandEventId: 'EVT-001',
  received: true,
  receivedAtMs: Date.now() - 50,
  vehicleConnected: true,
  ageSinceAckMs: 50,
}

describe('InputIndicatorPanel', () => {
  it('renders without crashing when no data', () => {
    render(<InputIndicatorPanel telemetry={noTelemetry} ack={noAck} />)
    expect(screen.getAllByText('Fahrzeug').length).toBeGreaterThan(0)
  })

  it('shows Nicht verbunden when ack is null', () => {
    render(<InputIndicatorPanel telemetry={noTelemetry} ack={noAck} />)
    expect(screen.getByText('Nicht verbunden')).toBeInTheDocument()
  })

  it('shows Verbunden when vehicle is connected', () => {
    render(<InputIndicatorPanel telemetry={baseTelemetry} ack={connectedAck} />)
    expect(screen.getByText('Verbunden')).toBeInTheDocument()
  })

  it('displays speed from telemetry', () => {
    const tel = { ...baseTelemetry, speedKmh: 12.5 }
    render(<InputIndicatorPanel telemetry={tel} ack={connectedAck} />)
    expect(screen.getByText('12.5')).toBeInTheDocument()
  })

  it('shows steer actual value', () => {
    const tel = { ...baseTelemetry, steerActual: 0.5 }
    render(<InputIndicatorPanel telemetry={tel} ack={connectedAck} />)
    expect(screen.getByText('50%')).toBeInTheDocument()
  })

  it('shows ACK age when received', () => {
    render(<InputIndicatorPanel telemetry={baseTelemetry} ack={connectedAck} />)
    expect(screen.getByText(/ACK vor/)).toBeInTheDocument()
  })

  it('shows Getrennt when vehicle disconnected', () => {
    const disconnectedAck: VehicleAckData = { ...connectedAck, vehicleConnected: false }
    render(<InputIndicatorPanel telemetry={baseTelemetry} ack={disconnectedAck} />)
    expect(screen.getByText('Getrennt')).toBeInTheDocument()
  })

  // OBS-01: "zuletzt gesehen" heartbeat, sourced from telemetry (independent of command ACKs)
  it('shows Zuletzt gesehen heartbeat when telemetry has a timestamp', () => {
    render(<InputIndicatorPanel telemetry={baseTelemetry} ack={connectedAck} />)
    expect(screen.getByText(/Zuletzt gesehen vor/)).toBeInTheDocument()
  })

  it('does not show Zuletzt gesehen when telemetry has no timestamp yet', () => {
    const tel = { ...baseTelemetry, timestampMs: 0, ageSinceUpdateMs: null }
    render(<InputIndicatorPanel telemetry={tel} ack={connectedAck} />)
    expect(screen.queryByText(/Zuletzt gesehen vor/)).not.toBeInTheDocument()
  })

  it('does not show Zuletzt gesehen when telemetry is null', () => {
    render(<InputIndicatorPanel telemetry={noTelemetry} ack={connectedAck} />)
    expect(screen.queryByText(/Zuletzt gesehen vor/)).not.toBeInTheDocument()
  })

  it('shows heartbeat age in seconds when >= 1000ms', () => {
    const tel = { ...baseTelemetry, ageSinceUpdateMs: 2500 }
    render(<InputIndicatorPanel telemetry={tel} ack={connectedAck} />)
    expect(screen.getByText('Zuletzt gesehen vor 2.5s')).toBeInTheDocument()
  })
})
