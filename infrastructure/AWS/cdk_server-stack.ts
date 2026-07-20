import * as cdk from "aws-cdk-lib";
import { Construct } from "constructs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as iam from "aws-cdk-lib/aws-iam";
import * as s3 from "aws-cdk-lib/aws-s3";
import * as ssm from "aws-cdk-lib/aws-ssm";

export class StreamingStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // =========================
    // VPC
    // =========================
    const vpc = new ec2.Vpc(this, "Vpc", {
      maxAzs: 2,
      natGateways: 0,
    });

    // =========================
    // Security Group
    // =========================
    const sg = new ec2.SecurityGroup(this, "StreamingSG", {
      vpc,
      allowAllOutbound: true,
    });

    // SSH — Zugang zusätzlich zu SSM Session Manager (Key Pair siehe unten)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(22), "SSH");

    // HTTP / HTTPS — für zukünftigen Reverse Proxy (z. B. Caddy/nginx auf 80/443)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(80));
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(443));

    // NEU: Operator-UI — nginx Frontend (docker-compose: 3000:80)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3000), "Frontend nginx");

    // NEU: Control Server — Operator-WS + Vehicle-WS + REST (docker-compose: 8080:8080)
    // Vehicle verbindet sich direkt hier (nicht über nginx) für /vehicle/ws
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(8080), "Control Server");

    // NEU: MQTT Broker — Fahrzeug-Telemetrie pub/sub (docker-compose: 1883:1883)
    // ACHTUNG: Mosquitto läuft ohne Auth — in Produktion auf Vehicle-IP einschränken
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(1883), "MQTT Broker");

    // coturn STUN/TURN — network_mode: host → bindet direkt an Port 3478 (kein Bridge-Mapping)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3478), "coturn TCP");
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(3478), "coturn UDP");

    // TURN relay ports — volle Range für coturn relay allocation (Sprint 10: 49160-49200 war zu eng)
    sg.addIngressRule(
      ec2.Peer.anyIpv4(),
      ec2.Port.udpRange(49152, 65535),
      "TURN relay",
    );

    // NEU: WebRTC SFU RTP Media — Vehicle → SFU → Operator (docker-compose: 10000-10050:10000-10050/udp)
    sg.addIngressRule(
      ec2.Peer.anyIpv4(),
      ec2.Port.udpRange(10000, 10050),
      "SFU RTP media",
    );

    // NEU: Grafana Monitoring (docker-compose: 3001:3000)
    // ACHTUNG: In Produktion auf eigene IP einschränken: ec2.Peer.ipv4("DEINE_IP/32")
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3001), "Grafana");

    // MediaMTX WHIP/WHEP — HTTP Signaling (ADR-020)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(8889), "MediaMTX WHIP/WHEP HTTP");
    // MediaMTX ICE UDP mux — separater Port vom HTTP Signaling (Sprint 10)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(8189), "MediaMTX ICE mux UDP");

    // =========================
    // S3 Bucket
    // =========================
    const bucket = new s3.Bucket(this, "AppBucket", {
      versioned: true,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
      // AUDITBACKUP-01: tägliche pg_dump-Backups verfallen nach 30 Tagen automatisch,
      // damit der Bucket nicht unbegrenzt wächst (scripts/backup-audit-store.sh).
      lifecycleRules: [
        {
          prefix: "backups/postgres/",
          expiration: cdk.Duration.days(30),
        },
      ],
    });

    // AUDITBACKUP-01: Bucket-Name per SSM statt hartkodiert in scripts/backup-audit-store.sh —
    // konsistent mit dem bestehenden SSM-Secret-Verteilungsmuster in scripts/deploy.sh.
    new ssm.StringParameter(this, "BackupBucketNameParam", {
      parameterName: "/avoc/prod/backup-bucket-name",
      stringValue: bucket.bucketName,
    });

    // =========================
    // SSH Key Pair
    // =========================
    // Ermöglicht direkten SSH-Zugang zusätzlich zu SSM Session Manager.
    // Privater Schlüssel wird automatisch als SecureString in SSM abgelegt
    // (Pfad: /ec2/keypair/<KeyPairId> — siehe Output "KeyPairId" nach dem Deploy).
    const keyPair = new ec2.KeyPair(this, "AvocKeyPair", {
      keyPairName: "avoc-ec2-keypair",
      type: ec2.KeyPairType.ED25519,
    });

    // =========================
    // EC2
    // =========================
    // HINWEIS: t3.micro (1 GB RAM) ist knapp für 10+ Container.
    // Für stabilen Betrieb: ec2.InstanceSize.SMALL empfohlen.
    // Achtung: Änderung des InstanceType erzwingt CloudFormation-Instance-Replacement.
    const instance = new ec2.Instance(this, "StreamingInstance", {
      vpc,
      instanceType: ec2.InstanceType.of(
        ec2.InstanceClass.T3,
        ec2.InstanceSize.MICRO,
      ),
      machineImage: ec2.MachineImage.latestAmazonLinux2023(),
      vpcSubnets: {
        subnetType: ec2.SubnetType.PUBLIC,
      },
      securityGroup: sg,
      keyPair,
    });

    // =========================
    // IAM
    // =========================
    // BEHALTEN: SSM Session Manager für Remote-Zugriff ohne SSH
    instance.role.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName(
        "AmazonSSMManagedInstanceCore",
      ),
    );

    // NEU: SSM Parameter Store — deploy.sh holt Secrets zur Laufzeit aus /avoc/prod/*
    // Kein .env auf der Instanz, kein Docker Secret — nur SSM (ADR-019)
    instance.role.addToPrincipalPolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: [
          "ssm:GetParameter",
          "ssm:GetParameters",
          "ssm:GetParametersByPath",
        ],
        resources: [
          `arn:aws:ssm:${this.region}:${this.account}:parameter/avoc/*`,
        ],
      }),
    );

    // GEÄNDERT: grantRead → grantReadWrite (für zukünftige Audit-Log-Backups, ADR-018)
    bucket.grantReadWrite(instance.role);

    // =========================
    // Elastic IP
    // =========================
    const eip = new ec2.CfnEIP(this, "ElasticIP");

    new ec2.CfnEIPAssociation(this, "EIPAssoc", {
      eip: eip.ref,
      instanceId: instance.instanceId,
    });

    // =========================
    // UserData
    // =========================
    // GEÄNDERT: Docker Compose Plugin v2 (docker compose) statt altem Standalone-Binary
    // 'docker compose' ist der aktuelle Standard — kein separater 'docker-compose' Binary mehr
    // HINWEIS: UserData läuft nur beim ersten Start. Auf bestehender Instanz manuell ausführen:
    //   sudo mkdir -p /usr/local/lib/docker/cli-plugins
    //   sudo curl -SL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 \
    //     -o /usr/local/lib/docker/cli-plugins/docker-compose
    //   sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
    instance.addUserData(`
#!/bin/bash
set -euxo pipefail

dnf update -y
dnf install -y docker aws-cli

# Docker Compose Plugin v2 — 'docker compose' (kein Bindestrich)
mkdir -p /usr/local/lib/docker/cli-plugins
curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64" \\
  -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

systemctl enable docker
systemctl start docker

usermod -aG docker ec2-user || true
usermod -aG docker ssm-user || true

# Dedizierter Admin-User (statt Standard-ec2-user) für Deployment-Zugriff
useradd -m -s /bin/bash -G docker,wheel ec2-admin || true
echo 'ec2-admin ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/ec2-admin

# SSH-Key des Key Pairs gilt standardmäßig nur für ec2-user (AWS-Key-Injection) —
# authorized_keys zusätzlich auf ec2-admin kopieren, damit SSH-Login als ec2-admin funktioniert.
mkdir -p /home/ec2-admin/.ssh
cp /home/ec2-user/.ssh/authorized_keys /home/ec2-admin/.ssh/authorized_keys
chown -R ec2-admin:ec2-admin /home/ec2-admin/.ssh
chmod 700 /home/ec2-admin/.ssh
chmod 600 /home/ec2-admin/.ssh/authorized_keys

mkdir -p /home/ec2-admin/app
chown ec2-admin:ec2-admin /home/ec2-admin/app
`);

    // =========================
    // Outputs
    // =========================
    new cdk.CfnOutput(this, "PublicIP", {
      value: eip.ref,
    });

    new cdk.CfnOutput(this, "BucketName", {
      value: bucket.bucketName,
    });

    new cdk.CfnOutput(this, "InstanceId", {
      value: instance.instanceId,
    });

    new cdk.CfnOutput(this, "KeyPairId", {
      value: keyPair.keyPairId,
      description: "Privater Schlüssel: aws ssm get-parameter --name /ec2/keypair/<dieser-Wert> --with-decryption --query Parameter.Value --output text",
    });
  }
}
