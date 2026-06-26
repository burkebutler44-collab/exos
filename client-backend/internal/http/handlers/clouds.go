package handlers

import (
	"net/http"

	"relay/client-backend/internal/services"
	"relay/client-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ListClouds(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	items, err := h.svc.ListClouds(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateCloud(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[createCloudRequest](c)
	if !ok {
		return
	}
	locationID, ok := optionalID(c, req.LocationID)
	if !ok {
		return
	}
	cloud, err := h.svc.CreateCloud(c.Request.Context(), organizationID, req.Name, locationID, req.Description, req.CreateDefaultNetwork, req.DefaultCIDR)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, cloud)
}

func (h *Handler) GetCloud(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	cloud, err := h.svc.GetCloud(c.Request.Context(), organizationID, cloudID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, cloud)
}

func (h *Handler) UpdateCloud(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	req, ok := bindJSON[updateCloudRequest](c)
	if !ok {
		return
	}
	cloud, err := h.svc.UpdateCloud(c.Request.Context(), organizationID, cloudID, req.Name, req.Description)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, cloud)
}

func (h *Handler) DeleteCloud(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteCloud(c.Request.Context(), organizationID, cloudID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetCloudOverview(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	overview, err := h.svc.GetCloudOverview(c.Request.Context(), organizationID, cloudID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, overview)
}

func (h *Handler) ListCloudServers(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	items, err := h.svc.ListCloudServers(c.Request.Context(), organizationID, cloudID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) ListOrganizationServers(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	items, err := h.svc.ListOrganizationServers(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) AssignServerToCloud(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	serverID, ok := paramID(c, "serverId")
	if !ok {
		return
	}
	item, err := h.svc.AssignServerToCloud(c.Request.Context(), organizationID, cloudID, serverID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) UnassignServerFromCloud(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	serverID, ok := paramID(c, "serverId")
	if !ok {
		return
	}
	item, err := h.svc.UnassignServerFromCloud(c.Request.Context(), organizationID, cloudID, serverID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) ChangeServerMode(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	serverID, ok := paramID(c, "serverId")
	if !ok {
		return
	}
	req, ok := bindJSON[changeServerModeRequest](c)
	if !ok {
		return
	}
	item, err := h.svc.ChangeServerMode(c.Request.Context(), organizationID, serverID, req.Mode)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) ListPrivateNetworks(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	items, err := h.svc.ListPrivateNetworks(c.Request.Context(), organizationID, cloudID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreatePrivateNetwork(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	req, ok := bindJSON[createPrivateNetworkRequest](c)
	if !ok {
		return
	}
	item, err := h.svc.CreatePrivateNetwork(c.Request.Context(), organizationID, cloudID, req.Name, req.Description, req.CIDR, req.GatewayIP)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) GetPrivateNetwork(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	networkID, ok := paramID(c, "networkId")
	if !ok {
		return
	}
	item, err := h.svc.GetPrivateNetwork(c.Request.Context(), organizationID, cloudID, networkID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeletePrivateNetwork(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	networkID, ok := paramID(c, "networkId")
	if !ok {
		return
	}
	if err := h.svc.DeletePrivateNetwork(c.Request.Context(), organizationID, cloudID, networkID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListNetworkAttachments(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	networkID, ok := paramID(c, "networkId")
	if !ok {
		return
	}
	items, err := h.svc.ListNetworkAttachments(c.Request.Context(), organizationID, cloudID, networkID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateNetworkAttachment(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	networkID, ok := paramID(c, "networkId")
	if !ok {
		return
	}
	req, ok := bindJSON[createNetworkAttachmentRequest](c)
	if !ok {
		return
	}
	resourceID, err := uuid.Parse(req.ResourceID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	item, err := h.svc.CreateNetworkAttachment(c.Request.Context(), store.CreateNetworkAttachmentParams{
		OrganizationID:   organizationID,
		CloudID:          cloudID,
		PrivateNetworkID: networkID,
		ResourceType:     req.ResourceType,
		ResourceID:       resourceID,
		PrivateIP:        req.PrivateIP,
		MACAddress:       req.MACAddress,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) DetachNetworkAttachment(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	networkID, ok := paramID(c, "networkId")
	if !ok {
		return
	}
	attachmentID, ok := paramID(c, "attachmentId")
	if !ok {
		return
	}
	if err := h.svc.DetachNetworkAttachment(c.Request.Context(), organizationID, cloudID, networkID, attachmentID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListVirtualMachines(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	items, err := h.svc.ListVirtualMachines(c.Request.Context(), organizationID, cloudID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateVirtualMachine(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	req, ok := bindJSON[createVirtualMachineRequest](c)
	if !ok {
		return
	}
	hostID, ok := optionalID(c, req.HostServerID)
	if !ok {
		return
	}
	networkID, ok := optionalID(c, req.PrivateNetworkID)
	if !ok {
		return
	}
	item, err := h.svc.CreateVirtualMachine(c.Request.Context(), store.CreateVirtualMachineParams{
		OrganizationID:   organizationID,
		CloudID:          cloudID,
		HostServerID:     hostID,
		Name:             req.Name,
		Hostname:         req.Hostname,
		CPUCores:         req.CPUCores,
		MemoryMB:         req.MemoryMB,
		DiskGB:           req.DiskGB,
		ImageID:          req.ImageID,
		OSImage:          req.OSImage,
		PrivateNetworkID: networkID,
		PrivateIP:        req.PrivateIP,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) GetVirtualMachine(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	vmID, ok := paramID(c, "vmId")
	if !ok {
		return
	}
	item, err := h.svc.GetVirtualMachine(c.Request.Context(), organizationID, cloudID, vmID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) UpdateVirtualMachine(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	vmID, ok := paramID(c, "vmId")
	if !ok {
		return
	}
	req, ok := bindJSON[updateVirtualMachineRequest](c)
	if !ok {
		return
	}
	hostID, ok := optionalID(c, req.HostServerID)
	if !ok {
		return
	}
	item, err := h.svc.UpdateVirtualMachine(c.Request.Context(), organizationID, cloudID, vmID, store.UpdateVirtualMachineParams{
		HostServerID: hostID,
		Name:         req.Name,
		Hostname:     req.Hostname,
		CPUCores:     req.CPUCores,
		MemoryMB:     req.MemoryMB,
		DiskGB:       req.DiskGB,
		ImageID:      req.ImageID,
		OSImage:      req.OSImage,
		PrivateIP:    req.PrivateIP,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteVirtualMachine(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	vmID, ok := paramID(c, "vmId")
	if !ok {
		return
	}
	if err := h.svc.DeleteVirtualMachine(c.Request.Context(), organizationID, cloudID, vmID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) PowerVirtualMachine(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	vmID, ok := paramID(c, "vmId")
	if !ok {
		return
	}
	item, err := h.svc.PowerVirtualMachine(c.Request.Context(), organizationID, cloudID, vmID, c.Param("action"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) ListManagedServices(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	items, err := h.svc.ListManagedServices(c.Request.Context(), organizationID, cloudID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreatePostgresService(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	req, ok := bindJSON[createPostgresRequest](c)
	if !ok {
		return
	}
	hostID, ok := optionalID(c, req.HostServerID)
	if !ok {
		return
	}
	networkID, ok := optionalID(c, req.PrivateNetworkID)
	if !ok {
		return
	}
	item, err := h.svc.CreatePostgresService(c.Request.Context(), store.CreateManagedServiceParams{
		OrganizationID:      organizationID,
		CloudID:             cloudID,
		HostServerID:        hostID,
		Name:                req.Name,
		CPUCores:            req.CPUCores,
		MemoryMB:            req.MemoryMB,
		StorageGB:           req.StorageGB,
		Version:             req.Version,
		PrivateNetworkID:    networkID,
		PrivateIP:           req.PrivateIP,
		BackupEnabled:       req.BackupEnabled,
		BackupRetentionDays: req.BackupRetentionDays,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) GetManagedService(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	serviceID, ok := paramID(c, "serviceId")
	if !ok {
		return
	}
	item, err := h.svc.GetManagedService(c.Request.Context(), organizationID, cloudID, serviceID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteManagedService(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	serviceID, ok := paramID(c, "serviceId")
	if !ok {
		return
	}
	if err := h.svc.DeleteManagedService(c.Request.Context(), organizationID, cloudID, serviceID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ActOnManagedService(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	serviceID, ok := paramID(c, "serviceId")
	if !ok {
		return
	}
	item, err := h.svc.ActOnManagedService(c.Request.Context(), organizationID, cloudID, serviceID, c.Param("action"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) GetCloudCapacity(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	item, err := h.svc.GetCloudCapacity(c.Request.Context(), organizationID, cloudID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) ListPlacementOptions(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	items, err := h.svc.ListPlacementOptions(c.Request.Context(), organizationID, cloudID, c.Query("resourceType"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) ListCloudActivity(c *gin.Context) {
	organizationID, cloudID, ok := cloudParams(c)
	if !ok {
		return
	}
	items, err := h.svc.ListResourceActions(c.Request.Context(), organizationID, &cloudID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) AdminListCloudResources(c *gin.Context) {
	clouds, vms, services, networks, actions, err := h.svc.ListAdminCloudResources(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"clouds":           clouds,
		"vms":              vms,
		"managed_services": services,
		"private_networks": networks,
		"actions":          actions,
	})
}

func cloudParams(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	organizationID, ok := orgID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	cloudID, ok := paramID(c, "cloudId")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return organizationID, cloudID, true
}

func optionalID(c *gin.Context, raw *string) (*uuid.UUID, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return nil, false
	}
	return &id, true
}
