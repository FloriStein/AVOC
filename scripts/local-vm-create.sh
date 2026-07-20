#!/bin/bash
# local-vm-create.sh — Erzeugt die lokale Test-VM (Hetzner-Nachbildung) per virt-install +
# cloud-init (LOCALVM-02, Sprint 48). Nutzt libvirt/KVM (bereits auf diesem Rechner aktiv, s.
# tasks/backlog.md EPIC "Lokale Ansible-VM als Hetzner-Nachbildung" — Architektur-Entscheidung).
#
# cloud-init (NoCloud-Datenquelle) ist bewusst gewählt, weil es exakt der Mechanismus ist, den
# Hetzner Cloud selbst für neu angelegte Server nutzt (SSH-Key-Injection, Hostname, User-Setup)
# — die VM startet damit authentisch wie ein frisch angelegter Hetzner-Server.
#
# WICHTIG (Sprint 48 Teil A): Dieses Skript wird in diesem Sprint NICHT ausgeführt — reine
# Autorierung, geprüft nur per 'bash -n' + Shellcheck-artiger Durchsicht. Die tatsächliche
# VM-Erzeugung ist Sprint B (LOCALVM-08), siehe tasks/current-sprint.md /
# tasks/sprints/48-lokale-ansible-vm-teil-a.md.
#
# Verwendung:
#   bash scripts/local-vm-create.sh
#   VM_NAME=avoc-local-vm VM_RAM_MB=4096 VM_VCPUS=3 VM_DISK_GB=25 bash scripts/local-vm-create.sh
#
# Danach (Sprint B): Ansible-Inventory ist bereits mit der ermittelten VM-IP befüllt —
#   cd ansible && ansible-playbook site.yml && ansible-playbook deploy.yml
#
# Voraussetzungen auf dem Host (bereits verifiziert vorhanden, s. EPIC-Vorrecherche):
#   virsh, virt-install, qemu-img, ein ISO-Tool (xorriso oder genisoimage), /dev/kvm,
#   Nutzer in den Gruppen libvirt+kvm.
#
set -euo pipefail

# ─── Konfiguration (per Env-Variable überschreibbar) ──────────────────────────

VM_NAME=${VM_NAME:-avoc-local-vm}
VM_RAM_MB=${VM_RAM_MB:-4096}      # Näherung an Hetzner CPX21 (4 GB), s. hetzner-setup.md Schritt 1
VM_VCPUS=${VM_VCPUS:-3}           # Näherung an Hetzner CPX21 (3 vCPU)
VM_DISK_GB=${VM_DISK_GB:-25}
VM_HOSTNAME=${VM_HOSTNAME:-avoc-local-vm}
VM_USER=${VM_USER:-avoc}
VM_NETWORK=${VM_NETWORK:-default}  # libvirt NAT-Default-Netzwerk

SSH_PUBKEY_PATH=${SSH_PUBKEY_PATH:-$HOME/.ssh/id_ed25519.pub}

UBUNTU_RELEASE=${UBUNTU_RELEASE:-noble}   # Ubuntu 24.04 LTS, s. hetzner-setup.md Schritt 1
UBUNTU_IMG_URL=${UBUNTU_IMG_URL:-https://cloud-images.ubuntu.com/${UBUNTU_RELEASE}/current/${UBUNTU_RELEASE}-server-cloudimg-amd64.img}

WORK_DIR=${WORK_DIR:-$HOME/.local/share/avoc-local-vm}
BASE_IMG="${WORK_DIR}/${UBUNTU_RELEASE}-server-cloudimg-amd64.img"
VM_DISK="${WORK_DIR}/${VM_NAME}.qcow2"
SEED_ISO="${WORK_DIR}/${VM_NAME}-seed.iso"
CLOUDINIT_DIR="${WORK_DIR}/${VM_NAME}-cloudinit"

REPO_ROOT="$(cd "$(dirname "$(realpath "$0")")/.." && pwd)"
INVENTORY_FILE="${REPO_ROOT}/ansible/inventory/hosts.ini"

# ─── Vorbedingungen prüfen ─────────────────────────────────────────────────────

echo "=== AVOC Lokale Test-VM erzeugen: ${VM_NAME} ==="
echo ""

for bin in virsh virt-install qemu-img curl; do
  command -v "$bin" >/dev/null 2>&1 || { echo "ERROR: '$bin' nicht gefunden — s. Skript-Kopfkommentar (Voraussetzungen)."; exit 1; }
done

ISO_TOOL=""
if command -v xorriso >/dev/null 2>&1; then
  ISO_TOOL="xorriso"
elif command -v genisoimage >/dev/null 2>&1; then
  ISO_TOOL="genisoimage"
elif command -v cloud-localds >/dev/null 2>&1; then
  ISO_TOOL="cloud-localds"
else
  echo "ERROR: Kein ISO-Tool gefunden (xorriso, genisoimage oder cloud-localds nötig)."
  exit 1
fi

[ -f "$SSH_PUBKEY_PATH" ] || { echo "ERROR: SSH-Public-Key nicht gefunden: $SSH_PUBKEY_PATH (SSH_PUBKEY_PATH setzen)"; exit 1; }

if virsh dominfo "$VM_NAME" >/dev/null 2>&1; then
  echo "ERROR: VM '${VM_NAME}' existiert in libvirt bereits."
  echo "  Löschen: virsh destroy ${VM_NAME}; virsh undefine ${VM_NAME} --remove-all-storage"
  exit 1
fi

mkdir -p "$WORK_DIR" "$CLOUDINIT_DIR"

# ─── Traversal-Recht für 'libvirt-qemu' sicherstellen ─────────────────────────
# HINWEIS (Sprint 49, LOCALVM-08a, Erkenntnis aus dem ersten echten Lauf): 'virt-install' nutzt
# per Default 'qemu:///system' — der QEMU-Prozess läuft als Systemnutzer 'libvirt-qemu', nicht
# als der aufrufende Nutzer. Liegt WORK_DIR (Standard: $HOME/.local/share/avoc-local-vm) unter
# einem Home-Verzeichnis ohne Traversal-Recht für "andere" (Ubuntu-Default z. B. 750 auf $HOME,
# 700 auf ~/.local), scheitert 'virt-install' mit "Cannot access storage file ... Keine
# Berechtigung". Fix: nur das Traversal-Bit (o+x, bewusst KEIN o+r) auf den Verzeichnissen
# zwischen $HOME und WORK_DIR setzen — erlaubt gezielten Zugriff auf bekannte Dateipfade, ohne
# Verzeichnislisting für andere Nutzer freizugeben. Idempotent, nur relevant falls WORK_DIR unter
# $HOME liegt (Standardfall).
if [[ "$WORK_DIR" == "$HOME"/* ]]; then
  chmod o+x "$HOME" 2>/dev/null || true
  _walk="$HOME"
  _rel="${WORK_DIR#"$HOME"/}"
  IFS='/' read -ra _parts <<< "$_rel"
  for _part in "${_parts[@]}"; do
    _walk="${_walk}/${_part}"
    [ -d "$_walk" ] && { chmod o+x "$_walk" 2>/dev/null || true; }
  done
fi

# ─── Ubuntu-24.04-Cloud-Image herunterladen (einmalig, danach wiederverwendet) ─

if [ ! -f "$BASE_IMG" ]; then
  echo "[1/6] Lade Ubuntu ${UBUNTU_RELEASE} Cloud-Image..."
  curl -fSL --progress-bar -o "$BASE_IMG" "$UBUNTU_IMG_URL"
else
  echo "[1/6] Ubuntu-Cloud-Image bereits vorhanden: $BASE_IMG"
fi

# ─── VM-Disk als eigenständige Kopie des Basis-Images anlegen ─────────────────
# HINWEIS (Sprint 49, LOCALVM-08a, Erkenntnis aus dem ersten echten Lauf): Ursprünglich war hier
# ein Backing-File-Overlay vorgesehen (qemu-img create -b), das ist aber gegen die
# System-AppArmor-Konfiguration von libvirtd auf diesem Host NICHT lauffähig — virt-aa-helper
# generiert das Pro-Domain-AppArmor-Profil nur aus den in der Domain-XML direkt referenzierten
# Disk-Pfaden (Overlay-Datei + Seed-ISO), läuft die qcow2-Backing-Chain aber NICHT ab, um das
# Basis-Image mit aufzunehmen. Ergebnis: QEMU (läuft konfiniert unter dem
# 'libvirt-qemu'-AppArmor-Profil) bekommt beim Öffnen des Basis-Images "Permission denied", obwohl
# die regulären Unix-Dateirechte passen. Eine Korrektur der System-AppArmor-Policy bräuchte
# Root-Rechte (hier nicht verfügbar, s. EPIC-Kontext). Deshalb: eigenständige Kopie statt Overlay
# — dadurch referenziert die Domain-XML nur noch die eine Datei, für die virt-aa-helper das Profil
# ohnehin korrekt setzt. Kostet mehr Plattenplatz als ein dünnes Overlay (volle Kopie statt
# Differenz), ist aber die einzige praktikable Lösung ohne Root-Zugriff auf die AppArmor-Config.
echo "[2/6] Erzeuge VM-Disk (${VM_DISK_GB}G, eigenständige Kopie des Basis-Images)..."
qemu-img convert -f qcow2 -O qcow2 "$BASE_IMG" "$VM_DISK"
qemu-img resize "$VM_DISK" "${VM_DISK_GB}G"

# ─── cloud-init NoCloud-Datenquelle erzeugen ───────────────────────────────────
# Analog zu Hetzner Cloud selbst: SSH-Key-Injection, Hostname, dedizierter User (hier direkt
# 'avoc' statt root+späterer Bootstrap-User-Anlage — s. ansible/roles/bootstrap-Kommentar zu
# authorized_keys für den Unterschied zum echten Hetzner-Erstbootstrap).

SSH_PUBKEY_CONTENT=$(cat "$SSH_PUBKEY_PATH")

cat > "${CLOUDINIT_DIR}/user-data" <<EOF
#cloud-config
hostname: ${VM_HOSTNAME}
manage_etc_hosts: true

users:
  - name: ${VM_USER}
    groups: [sudo]
    shell: /bin/bash
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    ssh_authorized_keys:
      - ${SSH_PUBKEY_CONTENT}

package_update: true

# Docker wird bewusst NICHT hier per cloud-init installiert, sondern über
# ansible/roles/bootstrap — identischer, idempotenter Weg wie beim echten Hetzner-Server
# (kein zweiter, abweichender Installationspfad nur für die lokale VM).

runcmd:
  - [ systemctl, enable, ssh ]
  - [ systemctl, start, ssh ]
EOF

cat > "${CLOUDINIT_DIR}/meta-data" <<EOF
instance-id: ${VM_NAME}-$(date +%s)
local-hostname: ${VM_HOSTNAME}
EOF

echo "[3/6] Baue cloud-init NoCloud-Seed-ISO..."
case "$ISO_TOOL" in
  xorriso)
    xorriso -as genisoimage -output "$SEED_ISO" -volid cidata -joliet -rock \
      "${CLOUDINIT_DIR}/user-data" "${CLOUDINIT_DIR}/meta-data"
    ;;
  genisoimage)
    genisoimage -output "$SEED_ISO" -volid cidata -joliet -rock \
      "${CLOUDINIT_DIR}/user-data" "${CLOUDINIT_DIR}/meta-data"
    ;;
  cloud-localds)
    cloud-localds "$SEED_ISO" "${CLOUDINIT_DIR}/user-data" "${CLOUDINIT_DIR}/meta-data"
    ;;
esac

# ─── VM anlegen (virt-install --import, kein OS-Installer nötig) ──────────────

echo "[4/6] Erzeuge VM per virt-install..."
virt-install \
  --name "$VM_NAME" \
  --memory "$VM_RAM_MB" \
  --vcpus "$VM_VCPUS" \
  --disk "path=${VM_DISK},format=qcow2,bus=virtio" \
  --disk "path=${SEED_ISO},device=cdrom" \
  --os-variant ubuntu24.04 \
  --network "network=${VM_NETWORK},model=virtio" \
  --graphics none \
  --import \
  --noautoconsole

echo "[5/6] Warte auf Boot + IP-Vergabe (bis zu 120s)..."
# HINWEIS (Sprint 49, LOCALVM-08a, Erkenntnis aus dem ersten echten Lauf): Die ursprüngliche
# Fassung dieser Schleife nutzte 'cmd1 && cmd2'/'cmd1 || cmd2' als alleinstehende Anweisungen.
# Unter 'set -euo pipefail' (Skript-Kopf) beendet das die gesamte Domäne bereits beim ersten
# Schleifendurchlauf STILL (ohne Fehlermeldung!): '--source agent' schlägt praktisch immer fehl
# (kein qemu-guest-agent in einem frischen Cloud-Image installiert), und diese Fehlschläge
# propagieren durch 'pipefail' in die Zuweisung; ebenso ist '[ -n "$VM_IP" ] && break' als
# alleinstehende Anweisung ein Fehlschlag (Exit 1), solange die IP noch nicht gefunden wurde —
# 'errexit' beendet das Skript dann sofort. Deshalb: 'if'-Blöcke statt bare '&&'/'||' als
# Anweisung, plus '|| true' an den Pipelines selbst, damit ein Fehlschlag von 'virsh domifaddr'
# nicht die Zuweisung selbst zum Scheitern bringt.
VM_IP=""
for _ in $(seq 1 24); do
  VM_IP=$(virsh domifaddr "$VM_NAME" --source agent 2>/dev/null | awk '/ipv4/{print $4}' | cut -d/ -f1 | head -n1 || true)
  if [ -z "$VM_IP" ]; then
    VM_IP=$(virsh domifaddr "$VM_NAME" 2>/dev/null | awk '/ipv4/{print $4}' | cut -d/ -f1 | head -n1 || true)
  fi
  if [ -n "$VM_IP" ]; then
    break
  fi
  sleep 5
done

if [ -z "$VM_IP" ]; then
  echo "WARNUNG: Konnte VM-IP nicht automatisch ermitteln (VM bootet ggf. noch)."
  echo "  Manuell prüfen: virsh domifaddr ${VM_NAME}"
  echo "  Danach ansible/inventory/hosts.ini manuell mit der IP befüllen."
  exit 0
fi

echo "  VM-IP ermittelt: ${VM_IP}"

# ─── Ansible-Inventory befüllen ────────────────────────────────────────────────

echo "[6/6] Trage IP in ${INVENTORY_FILE} ein..."
if [ -f "$INVENTORY_FILE" ]; then
  sed -i.bak "s/<VM_IP_PLACEHOLDER>/${VM_IP}/" "$INVENTORY_FILE"
  echo "  Inventory aktualisiert (Backup: ${INVENTORY_FILE}.bak)."
else
  echo "  WARNUNG: ${INVENTORY_FILE} nicht gefunden — Inventory manuell anlegen."
fi

echo ""
echo "=== VM '${VM_NAME}' erzeugt — IP: ${VM_IP} ==="
echo ""
echo "Nächste Schritte:"
echo "  ssh ${VM_USER}@${VM_IP}"
echo "  cd ${REPO_ROOT}/ansible && ansible-playbook site.yml"
echo "  cd ${REPO_ROOT}/ansible && ansible-playbook deploy.yml"
