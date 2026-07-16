import { describe, it, expect, vi } from 'vitest'
import { isAudibleSeverity, shouldPlayNow, playAlertTone, type ToneAudioContext } from './fleet-alert-sound'

describe('isAudibleSeverity', () => {
  it('warning ist hörbar', () => {
    expect(isAudibleSeverity('warning')).toBe(true)
  })

  it('critical ist hörbar', () => {
    expect(isAudibleSeverity('critical')).toBe(true)
  })

  it('info ist nicht hörbar (bewusste Grill-Me-Entscheidung: zu niedrigschwellig für Ton)', () => {
    expect(isAudibleSeverity('info')).toBe(false)
  })
})

describe('shouldPlayNow', () => {
  it('lastPlayedAtMs === null: darf sofort spielen (erster Ton überhaupt)', () => {
    expect(shouldPlayNow(null, 1000)).toBe(true)
  })

  it('Intervall noch nicht erreicht: darf nicht spielen', () => {
    expect(shouldPlayNow(1000, 1500, 2000)).toBe(false)
  })

  it('Intervall exakt erreicht: darf spielen (Grenzwert inklusiv)', () => {
    expect(shouldPlayNow(1000, 3000, 2000)).toBe(true)
  })

  it('Intervall deutlich überschritten: darf spielen', () => {
    expect(shouldPlayNow(1000, 10000, 2000)).toBe(true)
  })

  it('nowMs vor lastPlayedAtMs (Uhr-Anomalie): darf nicht spielen, kein negativer Wert bricht die Logik', () => {
    expect(shouldPlayNow(5000, 1000, 2000)).toBe(false)
  })

  it('verwendet den Default-Intervall von 2000ms, wenn keiner übergeben wird', () => {
    expect(shouldPlayNow(1000, 2999)).toBe(false)
    expect(shouldPlayNow(1000, 3000)).toBe(true)
  })
})

function makeFakeAudioContext() {
  const gainNode = {
    gain: { setValueAtTime: vi.fn(), linearRampToValueAtTime: vi.fn() },
    connect: vi.fn(),
  }
  const oscNode = {
    type: '',
    frequency: { setValueAtTime: vi.fn(), linearRampToValueAtTime: vi.fn() },
    connect: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
  }
  const ctx: ToneAudioContext = {
    currentTime: 5,
    destination: { id: 'destination' },
    createGain: vi.fn(() => gainNode),
    createOscillator: vi.fn(() => oscNode),
  }
  return { ctx, gainNode, oscNode }
}

describe('playAlertTone', () => {
  it('erzeugt Gain- und Oscillator-Node, verbindet sie über destination', () => {
    const { ctx, gainNode, oscNode } = makeFakeAudioContext()

    playAlertTone(ctx)

    expect(ctx.createGain).toHaveBeenCalledTimes(1)
    expect(ctx.createOscillator).toHaveBeenCalledTimes(1)
    expect(gainNode.connect).toHaveBeenCalledWith(ctx.destination)
    expect(oscNode.connect).toHaveBeenCalledWith(gainNode)
  })

  it('setzt eine Gain-Hüllkurve (0 -> Peak -> 0), keine abrupte Flanke', () => {
    const { ctx, gainNode } = makeFakeAudioContext()

    playAlertTone(ctx)

    expect(gainNode.gain.setValueAtTime).toHaveBeenCalledWith(0, ctx.currentTime)
    const rampCalls = gainNode.gain.linearRampToValueAtTime.mock.calls
    expect(rampCalls[0][0]).toBeGreaterThan(0) // Attack auf einen hörbaren Pegel
    expect(rampCalls[rampCalls.length - 1][0]).toBe(0) // Release zurück auf 0
  })

  it('startet und stoppt den Oscillator mit einer kurzen, endlichen Dauer', () => {
    const { ctx, oscNode } = makeFakeAudioContext()

    playAlertTone(ctx)

    expect(oscNode.start).toHaveBeenCalledWith(ctx.currentTime)
    const stopAt = oscNode.stop.mock.calls[0][0]
    expect(stopAt).toBeGreaterThan(ctx.currentTime)
    expect(stopAt).toBeLessThan(ctx.currentTime + 1) // deutlich unter einer Sekunde, kein Dauerton
  })

  it('wirft weiter, wenn ein Node-Aufruf fehlschlägt (Fehlerbehandlung liegt beim Aufrufer)', () => {
    const { ctx } = makeFakeAudioContext()
    ctx.createOscillator = vi.fn(() => {
      throw new Error('context closed')
    })

    expect(() => playAlertTone(ctx)).toThrow('context closed')
  })
})
