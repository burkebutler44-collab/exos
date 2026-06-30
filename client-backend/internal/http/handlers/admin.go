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

type adminReasonRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) AdminListUsers(c *gin.Context) {
	items, err := h.svc.ListAdminUsers(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminGetUser(c *gin.Context) { adminPending(c, "admin user detail wiring pending") }
func (h *Handler) AdminListUserOrganizations(c *gin.Context) {
	userID, ok := paramID(c, "userId")
	if !ok {
		return
	}
	items, err := h.svc.ListAdminUserOrganizations(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminUpdateUserPlatformRoles(c *gin.Context) {
	adminPending(c, "platform role assignment wiring pending")
}
func (h *Handler) AdminListOrganizations(c *gin.Context) {
	items, err := h.svc.ListAdminOrganizations(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminGetOrganization(c *gin.Context) {
	adminPending(c, "admin organization detail wiring pending")
}
func (h *Handler) AdminUpdateOrganization(c *gin.Context) {
	adminPending(c, "admin organization update wiring pending")
}
func (h *Handler) AdminListBillingAccounts(c *gin.Context) {
	items, err := h.svc.ListAdminBillingAccounts(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminGetBillingAccount(c *gin.Context) {
	adminPending(c, "admin billing account detail wiring pending")
}
func (h *Handler) AdminListInvoices(c *gin.Context) {
	c.JSON(http.StatusOK, []any{})
}
func (h *Handler) AdminListOrders(c *gin.Context) {
	c.JSON(http.StatusOK, []any{})
}
func (h *Handler) AdminListCredits(c *gin.Context) {
	c.JSON(http.StatusOK, []any{})
}
func (h *Handler) AdminListServers(c *gin.Context) {
	items, err := h.svc.ListAdminServers(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminCreateServer(c *gin.Context) {
	var req createAdminServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	locationID, err := uuid.Parse(req.LocationID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	serverFamilyID, err := uuid.Parse(req.ServerFamilyID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}

	var bmc *store.CreateServerBMCParams
	if req.BMC != nil {
		bmc = &store.CreateServerBMCParams{
			ManagementIP: strings.TrimSpace(req.BMC.ManagementIP),
			Username:     strings.TrimSpace(req.BMC.Username),
			Password:     req.BMC.Password,
			Protocol:     strings.TrimSpace(req.BMC.Protocol),
			Vendor:       strings.TrimSpace(req.BMC.Vendor),
		}
	}

	disks := make([]store.CreateServerDiskParams, len(req.Disks))
	for i, d := range req.Disks {
		disks[i] = store.CreateServerDiskParams{
			DeviceName:    strings.TrimSpace(d.DeviceName),
			CapacityGB:    d.CapacityGB,
			MediaType:     strings.TrimSpace(d.MediaType),
			InterfaceType: strings.TrimSpace(d.InterfaceType),
			Manufacturer:  strings.TrimSpace(d.Manufacturer),
			Model:         strings.TrimSpace(d.Model),
			SerialNumber:  strings.TrimSpace(d.SerialNumber),
			BootCapable:   d.BootCapable,
		}
	}

	nicParams := make([]store.CreateServerNICParams, len(req.NetworkInterfaces))
	for i, n := range req.NetworkInterfaces {
		var switchID *uuid.UUID
		if n.SwitchID != nil && strings.TrimSpace(*n.SwitchID) != "" {
			parsed, err := uuid.Parse(strings.TrimSpace(*n.SwitchID))
			if err != nil {
				writeError(c, services.ErrInvalidInput)
				return
			}
			switchID = &parsed
		}
		nicParams[i] = store.CreateServerNICParams{
			Label:        strings.TrimSpace(n.Label),
			MACAddress:   strings.TrimSpace(n.MACAddress),
			SpeedMbps:    n.SpeedMbps,
			IsPublic:     n.IsPublic,
			IPAddress:    n.IPAddress,
			Gateway:      n.Gateway,
			PrefixLength: n.PrefixLength,
			VLANID:       n.VLANID,
			SwitchID:     switchID,
			SwitchPort:   strings.TrimSpace(n.SwitchPort),
			Purpose:      strings.TrimSpace(n.Purpose),
			Notes:        strings.TrimSpace(n.Notes),
		}
	}

	item, err := h.svc.CreateAdminServer(c.Request.Context(), store.CreateAdminServerParams{
		LocationID:       locationID,
		ServerFamilyID:   serverFamilyID,
		Hostname:         strings.TrimSpace(req.Hostname),
		AssetTag:         strings.TrimSpace(req.AssetTag),
		SerialNumber:     strings.TrimSpace(req.SerialNumber),
		RackID:           strings.TrimSpace(req.RackID),
		RackPosition:     strings.TrimSpace(req.RackPosition),
		InstalledMemoryGB: req.InstalledMemoryGB,
		Provisionable:    req.Provisionable,
		Notes:            strings.TrimSpace(req.Notes),
		Disks:            disks,
		NetworkInterfaces: nicParams,
		BMC:              bmc,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (h *Handler) AdminListHardwareOptions(c *gin.Context) {
	items, err := h.svc.ListHardwareOptions(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminCreateHardwareOption(c *gin.Context) {
	var req createHardwareOptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	var locationID *uuid.UUID
	if req.LocationID != nil && strings.TrimSpace(*req.LocationID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.LocationID))
		if err != nil {
			writeError(c, services.ErrInvalidInput)
			return
		}
		locationID = &parsed
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	item, err := h.svc.CreateHardwareOption(c.Request.Context(), store.CreateHardwareOptionParams{
		OptionType:             strings.TrimSpace(req.OptionType),
		Label:                  strings.TrimSpace(req.Label),
		Description:            strings.TrimSpace(req.Description),
		Unit:                   strings.TrimSpace(req.Unit),
		ValueText:              strings.TrimSpace(req.ValueText),
		ValueGB:                req.ValueGB,
		PriceDeltaCents:        req.PriceDeltaCents,
		HourlyPriceDeltaCents:  req.HourlyPriceDeltaCents,
		QuarterlyDeltaCents:    req.QuarterlyDeltaCents,
		YearlyDeltaCents:       req.YearlyDeltaCents,
		Currency:               strings.TrimSpace(req.Currency),
		QuantityAvailable:      req.QuantityAvailable,
		FulfillmentMode:        strings.TrimSpace(req.FulfillmentMode),
		EstimatedReadyMinHours: req.EstimatedReadyMinHours,
		EstimatedReadyMaxHours: req.EstimatedReadyMaxHours,
		LocationID:             locationID,
		HardwareProfileName:    strings.TrimSpace(req.HardwareProfileName),
		CPUModel:               strings.TrimSpace(req.CPUModel),
		Active:                 active,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (h *Handler) AdminListHardwareFulfillmentOrders(c *gin.Context) {
	items, err := h.svc.ListHardwareFulfillmentOrders(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminMarkHardwareFulfillmentReady(c *gin.Context) {
	orderID, ok := paramID(c, "orderId")
	if !ok {
		return
	}
	item, err := h.svc.MarkHardwareFulfillmentReady(c.Request.Context(), orderID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (h *Handler) AdminGetServer(c *gin.Context) {
	adminPending(c, "admin server detail wiring pending")
}
func (h *Handler) AdminUpdateServer(c *gin.Context) {
	adminPending(c, "server inventory update wiring pending")
}
func (h *Handler) AdminSetServerMaintenance(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	adminPending(c, "server maintenance workflow wiring pending")
}
func (h *Handler) AdminGetServerPower(c *gin.Context) {
	adminPending(c, "rack request/reply power status wiring pending")
}
func (h *Handler) AdminProvisionServer(c *gin.Context) {
	adminPending(c, "admin provisioning command wiring pending")
}
func (h *Handler) AdminRescueServer(c *gin.Context) {
	adminPending(c, "admin rescue command wiring pending")
}
func (h *Handler) AdminCreateRack(c *gin.Context) { adminPending(c, "rack create wiring pending") }
func (h *Handler) AdminUpdateRack(c *gin.Context) { adminPending(c, "rack update wiring pending") }
func (h *Handler) AdminListRackServers(c *gin.Context) {
	items, err := h.svc.ListAdminServers(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	rackID := c.Param("rackId")
	filtered := make([]any, 0, len(items))
	for _, item := range items {
		if item.RackID == rackID {
			filtered = append(filtered, item)
		}
	}
	c.JSON(http.StatusOK, filtered)
}
func (h *Handler) AdminListLocations(c *gin.Context) {
	items, err := h.svc.ListAdminLocations(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListCPUProfiles(c *gin.Context) {
	items, err := h.svc.ListAdminCPUProfiles(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListServerFamilies(c *gin.Context) {
	items, err := h.svc.ListAdminServerFamilies(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListSwitches(c *gin.Context) {
	items, err := h.svc.ListAdminSwitches(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListEdgeRouters(c *gin.Context) {
	items, err := h.svc.ListAdminEdgeRouters(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListServerNetworkInterfaces(c *gin.Context) {
	items, err := h.svc.ListAdminServerNetworkInterfaces(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListHypervisors(c *gin.Context) {
	items, err := h.svc.ListAdminHypervisors(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListHypervisorVMs(c *gin.Context) {
	items, err := h.svc.ListAdminHypervisorVMs(c.Request.Context(), c.Param("hypervisorId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListProvisioningJobs(c *gin.Context) {
	items, err := h.svc.ListAdminProvisioningJobs(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminGetProvisioningJob(c *gin.Context) {
	adminPending(c, "admin provisioning job detail wiring pending")
}
func (h *Handler) AdminListProvisioningJobEvents(c *gin.Context) {
	adminPending(c, "admin provisioning job event repository wiring pending")
}
func (h *Handler) AdminListImages(c *gin.Context) {
	items, err := h.svc.ListAdminOSImages(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminCreateImage(c *gin.Context) { adminPending(c, "OS image create wiring pending") }
func (h *Handler) AdminGetImage(c *gin.Context)    { adminPending(c, "OS image detail wiring pending") }
func (h *Handler) AdminUpdateImage(c *gin.Context) { adminPending(c, "OS image update wiring pending") }
func (h *Handler) AdminEnableImage(c *gin.Context) { adminPending(c, "OS image enable wiring pending") }
func (h *Handler) AdminDisableImage(c *gin.Context) {
	adminPending(c, "OS image disable wiring pending")
}
func (h *Handler) AdminMakeDefaultImage(c *gin.Context) {
	adminPending(c, "default OS image wiring pending")
}
func (h *Handler) AdminListAuditLog(c *gin.Context) {
	items, err := h.svc.ListAdminAuditEvents(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListNATSConnections(c *gin.Context) {
	if h.natsMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "nats monitoring is not configured"})
		return
	}
	items, err := h.natsMonitor.ListConnections(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "nats monitoring request failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) AdminListLocationHealth(c *gin.Context) {
	racks, err := h.svc.ListAdminRacks(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	rackHealth := make([]RackHealthRack, 0, len(racks))
	for _, rack := range racks {
		rackHealth = append(rackHealth, RackHealthRack{
			ID:              rack.ID,
			Location:        rack.Location,
			LocationCode:    rack.LocationCode,
			Status:          rack.Status,
			LastHeartbeatAt: rack.LastHeartbeatAt,
		})
	}

	connections := NATSConnectionsResponse{}
	if h.natsMonitor != nil {
		connections, err = h.natsMonitor.ListConnections(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"now":       time.Now().UTC().Format(time.RFC3339Nano),
				"locations": buildLocationHealth(rackHealth, NATSConnectionsResponse{}).Locations,
				"warning":   "nats monitoring request failed: " + err.Error(),
			})
			return
		}
	}
	c.JSON(http.StatusOK, buildLocationHealth(rackHealth, connections))
}
func (h *Handler) AdminGetSettings(c *gin.Context) {
	adminPending(c, "admin settings repository wiring pending")
}
func (h *Handler) AdminUpdateSettings(c *gin.Context) {
	adminPending(c, "admin settings update wiring pending")
}

func (h *Handler) AdminSuspendOrganization(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	adminPending(c, "organization suspension workflow wiring pending")
}

func (h *Handler) AdminResumeOrganization(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	adminPending(c, "organization resume workflow wiring pending")
}

func (h *Handler) AdminManualCredit(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	adminPending(c, "manual credit workflow wiring pending")
}

func (h *Handler) AdminManualDebit(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	adminPending(c, "manual debit workflow wiring pending")
}

func (h *Handler) AdminAssignServer(c *gin.Context) {
	req, ok := bindJSON[adminAssignServerRequest](c)
	if !ok {
		return
	}
	organizationUUID, err := uuid.Parse(req.OrganizationID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	serverUUID, ok := paramID(c, "serverId")
	if !ok {
		return
	}
	if err := h.svc.AdminAssignServer(c.Request.Context(), serverUUID, organizationUUID); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "assigned"})
}

func (h *Handler) AdminReleaseServer(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	var req adminReasonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	serverUUID, ok := paramID(c, "serverId")
	if !ok {
		return
	}
	if err := h.svc.AdminReleaseServer(c.Request.Context(), serverUUID); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "released"})
}

func (h *Handler) AdminReserveServer(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	adminPending(c, "server reservation workflow wiring pending")
}

func (h *Handler) AdminRetireServer(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	var req adminReasonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	serverUUID, ok := paramID(c, "serverId")
	if !ok {
		return
	}
	if err := h.svc.AdminRetireServer(c.Request.Context(), serverUUID); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "retired"})
}

func (h *Handler) AdminPowerServer(c *gin.Context) {
	if h.provisionPublisher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "power command publisher unavailable"})
		return
	}
	req, ok := bindJSON[adminPowerServerRequest](c)
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
	payload := messages.PowerCommand{ServerID: serverID, Action: messages.PowerAction(req.Action)}
	env, err := messages.NewEnvelope(messages.PowerCommandType, rackID, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid power payload"})
		return
	}
	env.ServerID = &serverID
	correlationID := uuid.NewString()
	env.CorrelationID = &correlationID
	subject := messages.SubjectBuilder{}.DataCenterServerPower(location)
	if err := h.provisionPublisher.PublishCommand(c.Request.Context(), subject, env); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "publish power command failed"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message_id": env.MessageID, "correlation_id": correlationID, "subject": subject, "status": "queued"})
}

func (h *Handler) AdminReinstallServer(c *gin.Context) {
	if h.provisionPublisher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provisioning command publisher unavailable"})
		return
	}
	req, ok := bindJSON[adminReinstallServerRequest](c)
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
	payload := messages.ReinstallServerCommand{ServerID: serverID}
	env, err := messages.NewEnvelope(messages.ReinstallServerCommandType, rackID, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reinstall payload"})
		return
	}
	env.ServerID = &serverID
	correlationID := uuid.NewString()
	env.CorrelationID = &correlationID
	subject := messages.SubjectBuilder{}.DataCenterProvisionRequest(location)
	if err := h.provisionPublisher.PublishCommand(c.Request.Context(), subject, env); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "publish reinstall command failed"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message_id": env.MessageID, "correlation_id": correlationID, "subject": subject, "status": "queued"})
}

func (h *Handler) AdminRetryProvisioningJob(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	adminPending(c, "provisioning retry workflow wiring pending")
}

func (h *Handler) AdminCancelProvisioningJob(c *gin.Context) {
	if !requireAdminReason(c) {
		return
	}
	adminPending(c, "provisioning cancellation workflow wiring pending")
}

func requireAdminReason(c *gin.Context) bool {
	req, ok := bindJSON[adminReasonRequest](c)
	if !ok {
		return false
	}
	if strings.TrimSpace(req.Reason) == "" {
		writeError(c, services.ErrInvalidInput)
		return false
	}
	return true
}

func adminPending(c *gin.Context, message string) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": message,
		"todo":  "connect this protected admin endpoint to the corresponding domain service and audit log writer",
	})
}
