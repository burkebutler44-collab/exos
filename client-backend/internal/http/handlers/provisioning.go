package handlers

import (
	"net/http"
	"strings"
	"time"

	"relay/client-backend/internal/provisioning/messages"
	"relay/client-backend/internal/services"
	"relay/client-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ProvisionServer(c *gin.Context) {
	if h.provisionPublisher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provisioning command publisher unavailable"})
		return
	}
	req, ok := bindJSON[provisionServerRequest](c)
	if !ok {
		return
	}
	organizationID := c.Param("organizationId")
	organizationUUID, err := uuid.Parse(organizationID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	serverID := c.Param("serverId")
	serverUUID, err := uuid.Parse(serverID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	inventory, err := h.svc.GetProvisioningServerInventory(c.Request.Context(), organizationUUID, serverUUID)
	if err != nil {
		writeError(c, err)
		return
	}
	hostname := req.Hostname
	if hostname == "" {
		hostname = inventory.Hostname
	}
	location := normalizeProvisionLocation(firstNonEmptyString(req.Location, inventory.RackLocation))
	rackID := req.RackID
	if rackID == "" {
		rackID = firstNonEmptyString(inventory.RackID, location)
	} else {
		rackID = strings.ToLower(strings.TrimSpace(rackID))
	}
	projectID := req.ProjectID
	if projectID == nil && inventory.ProjectID != nil {
		project := inventory.ProjectID.String()
		projectID = &project
	}
	networkConfig := provisionNetworkConfig(req.NetworkConfig, inventory)
	hardwareMetadata := provisionHardwareMetadata(req.HardwareMetadata, inventory)
	payload := messages.ProvisionServerCommand{
		OrganizationID:   organizationID,
		ProjectID:        projectID,
		ServerID:         serverID,
		ImageID:          req.ImageID,
		Hostname:         hostname,
		SSHKeys:          req.SSHKeys,
		NetworkConfig:    messages.NetworkConfig(networkConfig),
		HardwareMetadata: hardwareMetadata,
	}
	env, err := messages.NewEnvelope(messages.ProvisionServerCommandType, rackID, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provision payload"})
		return
	}
	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	correlationID := uuid.NewString()
	env.ServerID = &serverID
	env.CorrelationID = &correlationID
	env.ExpiresAt = &expiresAt
	env.Metadata = map[string]string{"organization_id": organizationID}
	subject := messages.SubjectBuilder{}.DataCenterProvisionRequest(location)
	if err := h.provisionPublisher.PublishCommand(c.Request.Context(), subject, env); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "publish provision command failed"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message_id": env.MessageID, "correlation_id": correlationID, "subject": subject, "status": "queued"})
}

func (h *Handler) ReinstallServer(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "reinstall orchestrator wiring pending", "server_id": c.Param("serverId")})
}

func (h *Handler) GetProvisioningJob(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "provisioning job repository wiring pending", "job_id": c.Param("jobId")})
}

func (h *Handler) ListProvisioningJobEvents(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "provisioning job event repository wiring pending", "job_id": c.Param("jobId")})
}

func (h *Handler) PowerServer(c *gin.Context) {
	if h.provisionPublisher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "power command publisher unavailable"})
		return
	}
	req, ok := bindJSON[powerServerRequest](c)
	if !ok {
		return
	}
	serverID := c.Param("serverId")
	location := normalizeProvisionLocation(req.Location)
	rackID := req.RackID
	if rackID == "" {
		rackID = location
	} else {
		rackID = strings.ToLower(strings.TrimSpace(rackID))
	}
	payload := messages.PowerCommand{OrganizationID: c.Param("organizationId"), ServerID: serverID, Action: messages.PowerAction(req.Action)}
	env, err := messages.NewEnvelope(messages.PowerCommandType, rackID, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid power payload"})
		return
	}
	env.ServerID = &serverID
	correlationID := uuid.NewString()
	env.CorrelationID = &correlationID
	env.Metadata = map[string]string{"organization_id": c.Param("organizationId")}
	subject := messages.SubjectBuilder{}.DataCenterServerPower(location)
	if err := h.provisionPublisher.PublishCommand(c.Request.Context(), subject, env); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "publish power command failed"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message_id": env.MessageID, "correlation_id": correlationID, "subject": subject, "status": "queued"})
}

func (h *Handler) GetServerPower(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "rack request/reply client wiring pending", "server_id": c.Param("serverId")})
}

func normalizeProvisionLocation(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "ny"
	}
	return value
}

func provisionNetworkConfig(input map[string]any, inventory store.ProvisioningServerInventory) map[string]any {
	out := copyAnyMap(input)
	setNonEmpty(out, "mac", inventory.MACAddress)
	setNonEmpty(out, "address", derefString(inventory.IPAddress))
	setNonEmpty(out, "ip", derefString(inventory.IPAddress))
	setNonEmpty(out, "ip_address", derefString(inventory.IPAddress))
	setNonEmpty(out, "gateway", derefString(inventory.Gateway))
	setNonEmpty(out, "netmask", derefString(inventory.SubnetMask))
	setNonEmpty(out, "mask", derefString(inventory.SubnetMask))
	return out
}

func provisionHardwareMetadata(input map[string]any, inventory store.ProvisioningServerInventory) map[string]any {
	out := copyAnyMap(inventory.Metadata)
	for key, value := range input {
		out[key] = value
	}
	setNonEmpty(out, "bmc_endpoint", inventory.BMCAddress)
	setNonEmpty(out, "bmc_address", inventory.BMCAddress)
	setNonEmpty(out, "mac", inventory.MACAddress)
	setNonEmpty(out, "disk_name", inventory.DiskName)
	setNonEmpty(out, "rack_id", inventory.RackID)
	setNonEmpty(out, "location", inventory.RackLocation)
	return out
}

func copyAnyMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}

func setNonEmpty(values map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		values[key] = value
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (h *Handler) ListRacks(c *gin.Context) {
	items, err := h.svc.ListAdminRacks(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) GetRack(c *gin.Context) {
	items, err := h.svc.ListAdminRacks(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	rackID := c.Param("rackId")
	for _, item := range items {
		if item.ID == rackID {
			c.JSON(http.StatusOK, item)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func (h *Handler) GetRackHealth(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "rack request/reply client wiring pending", "rack_id": c.Param("rackId")})
}

func (h *Handler) ListRackAgents(c *gin.Context) {
	c.JSON(http.StatusOK, []any{})
}

func (h *Handler) SetRackMaintenance(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	c.JSON(http.StatusNotImplemented, gin.H{"error": "platform admin rack status wiring pending", "rack_id": c.Param("rackId")})
}

func (h *Handler) ResumeRack(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	c.JSON(http.StatusNotImplemented, gin.H{"error": "platform admin rack status wiring pending", "rack_id": c.Param("rackId")})
}
