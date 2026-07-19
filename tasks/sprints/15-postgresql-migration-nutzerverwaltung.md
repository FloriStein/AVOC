# Sprint 15 — PostgreSQL-Migration + Nutzerverwaltung

Ziel: SQLite vollständig durch PostgreSQL ersetzen. Echte Authentifizierung mit bcrypt. ADMIN-Rolle + Nutzerverwaltung im Frontend. Login-Overlay statt Auto-Connect.

Datum: 2026-06-14 | **Status: Alle Tasks ✅ · Edge-Case-Tests dokumentiert**
Vorgänger: Sprint 14 ✅

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| PG-01 | PostgreSQL als primäre DB (ADR-023) | L | ✅ |
| AUTH-02 | Echte Nutzerverwaltung mit bcrypt + ADMIN-Rolle (ADR-024) | L | ✅ |
| UI-02 | LoginPanel + UserManagementPanel + App-Umstellung | M | ✅ |
| TEST-01 | Edge-Case-Tests: Auth-Handler, parseTokenRole, LoginPanel, UserManagementPanel | M | ✅ |

---

## Scope-Details

### PG-01 — PostgreSQL-Migration ✅
- `pkg/db/postgres.go`: `Open(url)` Factory mit Pool (5 open, 2 idle, 30s lifetime)
- `pkg/audit/postgres_writer.go`: ersetzt `sqlite_writer.go`; `ON CONFLICT (event_id) DO NOTHING` statt `INSERT OR IGNORE`; `$N`-Placeholder
- `internal/vehicleregistry/postgres_store.go`: ersetzt `sqlite_store.go`; `SeedDefault()` idempotent via `ON CONFLICT`
- `go.mod`: `github.com/lib/pq v1.10.9` direkt; `modernc.org/sqlite` + Transitive entfernt
- `docker-compose.yml` + `docker-compose.prod.yml`: postgres:16-alpine Service; `depends_on: service_healthy`
- Durabilität: PostgreSQL `synchronous_commit=on` (Default) ≡ `PRAGMA wal_checkpoint(FULL)` — WriteSync() garantiert ohne extra PRAGMA

### AUTH-02 — Nutzerverwaltung ✅
- `internal/authservice/userstore.go`: `UserStore` Interface + `PostgresUserStore`; bcrypt cost=12 (~300ms/Login)
- `SeedAdmin()` idempotent via `ON CONFLICT (id) DO NOTHING` — sicher bei jedem Restart
- `RoleAdmin` neue Rolle im JWT-Claim `role`
- Auth-Service Endpoints: `GET/POST/DELETE/PATCH /auth/users` — alle hinter `RequireAdmin` Middleware
- `OperatorLogin`: kein "accept any" mehr — `Authenticate()` gegen DB, Rolle aus DB in Token

### UI-02 — Frontend-Umstellung ✅
- `LoginPanel.tsx`: Vollbild-Overlay; Submit-Button disabled bis beide Felder ausgefüllt; Loading-State; Fehleranzeige bei 401
- `UserManagementPanel.tsx`: Tabelle + Formular; Löschen eigenen Accounts gesperrt; Confirm-Dialog vor Löschen; Rolle inline per Dropdown
- `useSession.ts`: `connect(id, password)` statt `connect()` ohne Parameter; `OPERATOR_ID`-Konstante entfernt; `operatorId` als State
- `App.tsx`: kein `useEffect`-Auto-Connect mehr; `LoginPanel` wenn `!token`; "Benutzerverwaltung"-Button nur bei Admin-Token; "Abmelden"-Button
- `parseTokenRole(token)`: Base64-Dekodierung des JWT-Payloads ohne externe Library

### TEST-01 — Edge-Case-Tests ✅
Siehe **Edge-Case-Dokumentation** unten.

---

## Edge-Case-Dokumentation

### Auth-Handler (`internal/authservice/handler_test.go`) — 20 Tests

| Test | Erwartetes Verhalten |
|------|---------------------|
| Valide Credentials | 200 + JWT |
| Falsches Passwort | 401 |
| Unbekannter User | 401 |
| Deaktivierter User | 401 |
| Malformed JSON | 400 |
| Rolle aus DB im Token | `role`-Claim entspricht DB-Rolle (ADMIN bleibt ADMIN) |
| RequireAdmin: kein Token | 401 |
| RequireAdmin: ungültiger Token | 401 |
| RequireAdmin: OBSERVER-Token | 403 |
| RequireAdmin: ADMIN-Token | 200 (passiert durch) |
| CreateUser: fehlende Felder | 400 |
| CreateUser: doppelte ID | 409 |
| CreateUser: Role leer | 201 + Default OBSERVER |
| DeleteUser: eigener Account | 403 |
| DeleteUser: nicht existenter User | 404 |
| UpdateRole: leere Rolle | 400 |
| SeedAdmin: zweimal aufgerufen | idempotent, kein Fehler, 1 User |
| ValidateToken: Garbage | `{"valid":false}` |
| RefreshToken: ungültiger Token | 401 |
| VehicleRegister: beliebige ID | 200 (kein DB-Check) |

### parseTokenRole (`frontend/src/lib/api-client.test.ts`) — 8 Tests

| Eingabe | Erwartetes Verhalten |
|---------|---------------------|
| JWT mit `role: "ADMIN"` | `"ADMIN"` |
| JWT mit `role: "OBSERVER"` | `"OBSERVER"` |
| JWT ohne `role`-Claim | `""` |
| Kein gültiges JWT (3 Teile, kein JSON) | `""` |
| Leerer String | `""` |
| Nur ein Token-Teil | `""` |
| Ungültiges Base64 im Payload | `""` |
| Payload ist kein JSON | `""` |

### LoginPanel (`frontend/src/components/LoginPanel.test.tsx`) — 8 Tests

| Szenario | Erwartetes Verhalten |
|----------|---------------------|
| Render | ID-Feld, Passwort-Feld, Button vorhanden |
| Beide Felder leer | Button disabled |
| Nur ID ausgefüllt | Button disabled |
| Nur Passwort ausgefüllt | Button disabled |
| Beide Felder ausgefüllt | Button enabled |
| Submit mit korrekten Daten | `onLogin(id, password)` aufgerufen |
| Login-Fehler (onLogin wirft) | Fehlermeldung "Ungültige Zugangsdaten" |
| Während Login | Button zeigt "Anmelden…", ist disabled |
| Zweiter Versuch nach Fehler | Fehlermeldung verschwindet beim neuen Versuch |

### UserManagementPanel (`frontend/src/components/UserManagementPanel.test.tsx`) — 10 Tests

| Szenario | Erwartetes Verhalten |
|----------|---------------------|
| Laden | Alle API-User werden angezeigt |
| Eigener Account | Kein Löschen-Button |
| Fremder Account | Löschen-Button vorhanden |
| Löschen + Bestätigung | `deleteUser()` aufgerufen |
| Löschen + Ablehnung | `deleteUser()` NICHT aufgerufen |
| Anlegen-Button leer | disabled |
| Anlegen mit Daten | `createUser()` mit korrekten Parametern |
| API-Fehler | Fehlermeldung angezeigt |
| Rolle ändern | `updateUserRole()` aufgerufen |
| Panel schließen | `onClose()` aufgerufen |
