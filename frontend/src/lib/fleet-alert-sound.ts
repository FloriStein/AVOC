// Audio-Benachrichtigung für neue Fleet-Alerts (Sprint 25). Reine, testbare Entscheidungslogik
// (Severity-Filter, Debounce, Tonerzeugung über eine Minimal-Schnittstelle statt des vollen
// `AudioContext`-Typs) getrennt von React/WebSocket-Zustand, analog zu fleet-merge.ts.

import type { FleetAlert } from './api-client'

// info-Alerts bleiben stumm (Grill-Me 2026-07-16) — niedrigschwellig genug, dass ein Ton in einer
// 24/7-Leitstelle eher stört als hilft. warning/critical rechtfertigen aktive Aufmerksamkeit.
export function isAudibleSeverity(severity: FleetAlert['severity']): boolean {
  return severity === 'warning' || severity === 'critical'
}

// Verhindert Sound-Storm, wenn mehrere Alerts kurz hintereinander eintreffen (z. B. die
// AlertEngine aus FLEET-07, die für mehrere Fahrzeuge fast gleichzeitig Schwellenwerte auswertet).
// Reine Funktion ohne eigenen Timer — der Aufrufer hält `lastPlayedAtMs` selbst (Ref im Hook).
export function shouldPlayNow(lastPlayedAtMs: number | null, nowMs: number, minIntervalMs = 2000): boolean {
  if (lastPlayedAtMs === null) return true
  return nowMs - lastPlayedAtMs >= minIntervalMs
}

// Minimal-Schnittstellen statt der vollen DOM-Typen (`AudioContext`/`GainNode`/`OscillatorNode`) —
// jsdom (Test-Umgebung) implementiert die Web Audio API nicht, ein einfaches Fake-Objekt muss zum
// Testen genügen. Ein echter `AudioContext` erfüllt diese Interfaces strukturell (Übermenge).
interface ToneAudioParam {
  setValueAtTime(value: number, startTime: number): void
  linearRampToValueAtTime(value: number, endTime: number): void
}

interface ToneGainNode {
  gain: ToneAudioParam
  connect(destination: unknown): void
}

interface ToneOscillatorNode {
  type: string
  frequency: ToneAudioParam
  connect(destination: unknown): void
  start(when?: number): void
  stop(when?: number): void
}

export interface ToneAudioContext {
  currentTime: number
  destination: unknown
  createOscillator(): ToneOscillatorNode
  createGain(): ToneGainNode
  resume?(): Promise<void>
}

// Synthetischer Zwei-Ton-Piepton statt eines Audio-Assets (Grill-Me 2026-07-16) — kein Asset, keine
// Lizenzfrage, keine zusätzliche Bundle-Größe. Kurze Gain-Hüllkurve (Attack/Release) vermeidet
// Knack-Artefakte an den Flanken. Wirft synchron weiter, statt selbst zu fangen — Fehlerbehandlung
// (blockierter/fehlender AudioContext) ist Sache des Aufrufers (useFleetAlertSound).
export function playAlertTone(ctx: ToneAudioContext): void {
  const now = ctx.currentTime

  const gain = ctx.createGain()
  gain.gain.setValueAtTime(0, now)
  gain.gain.linearRampToValueAtTime(0.15, now + 0.01)
  gain.gain.linearRampToValueAtTime(0, now + 0.18)
  gain.connect(ctx.destination)

  const osc = ctx.createOscillator()
  osc.type = 'sine'
  osc.frequency.setValueAtTime(880, now)
  osc.frequency.setValueAtTime(1108, now + 0.09)
  osc.connect(gain)
  osc.start(now)
  osc.stop(now + 0.19)
}
