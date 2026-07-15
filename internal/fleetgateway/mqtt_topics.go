package fleetgateway

// MQTT topic convention shared between vehicle-mock's fleet simulation (FLEET-04, publisher)
// and fleet-service's future MQTT-backed FleetGateway implementation (FLEET-05, subscriber).
// This is the concrete, testable stand-in for the still-unspecified ROS2/DDS transport
// (ADR-027) — reuses Mosquitto, already deployed for Direct-Teleop telemetry (ADR-003), on a
// separate topic namespace ("fleet/...") so it never collides with the existing
// "vehicle/{id}/telemetry" topic.

// StatusTopic is where VehicleStatusEvent (as JSON) is published for the given vehicle.
func StatusTopic(vehicleID string) string { return "fleet/" + vehicleID + "/status" }

// AlertTopic is where VehicleAlertEvent (as JSON) is published for the given vehicle.
func AlertTopic(vehicleID string) string { return "fleet/" + vehicleID + "/alert" }

// TaskTopic is where TaskAssignment (as JSON) is published when fleet-service dispatches a task
// to a vehicle (FLEET-05, MQTTGateway.DispatchTask). No real vehicle-mock consumer subscribes to
// this yet — establishes the topic/format now, consistent with ADR-027's "adapter, not
// application logic, changes later" strategy.
func TaskTopic(vehicleID string) string { return "fleet/" + vehicleID + "/task" }

// StatusTopicWildcard subscribes to status updates from all fleet vehicles.
const StatusTopicWildcard = "fleet/+/status"

// AlertTopicWildcard subscribes to alerts from all fleet vehicles.
const AlertTopicWildcard = "fleet/+/alert"
