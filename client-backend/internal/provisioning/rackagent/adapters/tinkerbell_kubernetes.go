package adapters

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"relay/client-backend/internal/provisioning/messages"
)

//go:embed templates/*.yaml
var tinkerbellTemplates embed.FS

type ManifestApplier interface {
	Apply(ctx context.Context, manifest []byte) error
	GetJSON(ctx context.Context, resource, name, namespace string) ([]byte, error)
}

type KubectlApplier struct {
	Binary string
}

func NewKubectlApplier(binary string) KubectlApplier {
	if binary == "" {
		binary = "kubectl"
	}
	return KubectlApplier{Binary: binary}
}

func (a KubectlApplier) Apply(ctx context.Context, manifest []byte) error {
	cmd := exec.CommandContext(ctx, a.Binary, "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (a KubectlApplier) GetJSON(ctx context.Context, resource, name, namespace string) ([]byte, error) {
	args := []string{"get", resource, name}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "-o", "json")
	cmd := exec.CommandContext(ctx, a.Binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl get %s/%s: %w: %s", resource, name, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type KubernetesTinkerbellAdapter struct {
	Applier         ManifestApplier
	Secrets         RackSecretProvider
	Namespace       string
	ArtifactServer  string
	InstallPassword string
}

func NewKubernetesTinkerbellAdapter(applier ManifestApplier, secrets RackSecretProvider, namespace string) *KubernetesTinkerbellAdapter {
	if namespace == "" {
		namespace = "tink"
	}
	return &KubernetesTinkerbellAdapter{
		Applier:   applier,
		Secrets:   secrets,
		Namespace: namespace,
	}
}

func (a *KubernetesTinkerbellAdapter) EnsureHardware(ctx context.Context, command messages.ProvisionServerCommand) error {
	data, err := a.templateData(ctx, command, "")
	if err != nil {
		return err
	}
	return a.applyTemplates(ctx, data, "machine-auth.yaml", "machine.yaml", "hardware.yaml")
}

func (a *KubernetesTinkerbellAdapter) CreateWorkflow(ctx context.Context, command messages.ProvisionServerCommand) (string, error) {
	workflowID := dnsName("workflow-" + command.ServerID)
	data, err := a.templateData(ctx, command, workflowID)
	if err != nil {
		return "", err
	}
	if err := a.applyTemplates(ctx, data, "workflow.yaml"); err != nil {
		return "", err
	}
	return workflowID, nil
}

func (a *KubernetesTinkerbellAdapter) StartWorkflow(ctx context.Context, workflowID string) error {
	// Creating the Workflow CRD is the start signal for Tinkerbell.
	return nil
}

func (a *KubernetesTinkerbellAdapter) WorkflowStatus(ctx context.Context, workflowID string) (string, error) {
	raw, err := a.Applier.GetJSON(ctx, "workflows.tinkerbell.org", workflowID, a.Namespace)
	if err != nil {
		return "unknown", err
	}
	var body struct {
		Status map[string]any `json:"status"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "unknown", err
	}
	for _, key := range []string{"state", "status", "phase"} {
		if value, ok := body.Status[key].(string); ok && value != "" {
			return strings.ToLower(value), nil
		}
	}
	return "running", nil
}

func (a *KubernetesTinkerbellAdapter) Health(ctx context.Context) (string, error) {
	if _, err := a.Applier.GetJSON(ctx, "namespace", a.Namespace, ""); err != nil {
		return "degraded", err
	}
	return "online", nil
}

func (a *KubernetesTinkerbellAdapter) applyTemplates(ctx context.Context, data tinkerbellTemplateData, names ...string) error {
	rendered, err := renderTemplates(data, names...)
	if err != nil {
		return err
	}
	return a.Applier.Apply(ctx, rendered)
}

func (a *KubernetesTinkerbellAdapter) templateData(ctx context.Context, command messages.ProvisionServerCommand, workflowID string) (tinkerbellTemplateData, error) {
	creds, err := a.Secrets.GetBMCCredentials(ctx, command.ServerID)
	if err != nil {
		return tinkerbellTemplateData{}, err
	}
	serverName := dnsName(command.ServerID)
	ssh := strings.Join(command.SSHKeys, "\n")
	if workflowID == "" {
		workflowID = dnsName("workflow-" + command.ServerID)
	}
	bmcEndpoint := firstNonEmpty(
		stringValue(command.HardwareMetadata, "bmc_endpoint", ""),
		stringValue(command.HardwareMetadata, "bmc_address", ""),
		creds.Endpoint,
	)

	return tinkerbellTemplateData{
		Namespace:                   a.Namespace,
		ServerName:                  serverName,
		WorkflowName:                workflowID,
		TemplateName:                stringValue(command.HardwareMetadata, "template_name", command.ImageID),
		AuthName:                    serverName + "-bmc-auth",
		IpmiIp:                      cleanEndpoint(bmcEndpoint),
		IpmiUsername:                creds.Username,
		IpmiPassword:                creds.Password,
		DiskName:                    stringValue(command.HardwareMetadata, "disk_name", "/dev/nvme0n1"),
		Address:                     networkString(command.NetworkConfig, "address", "ip", "ip_address"),
		Gateway:                     networkString(command.NetworkConfig, "gateway"),
		Netmask:                     networkString(command.NetworkConfig, "netmask", "mask"),
		MAC:                         firstNonEmpty(stringValue(command.HardwareMetadata, "mac", ""), networkString(command.NetworkConfig, "mac")),
		Binary:                      stringValue(command.HardwareMetadata, "ipxe_binary", "ipxe.efi"),
		CustomIpxe:                  stringValue(command.HardwareMetadata, "custom_ipxe", ""),
		ArtifactServer:              firstNonEmpty(a.ArtifactServer, stringValue(command.HardwareMetadata, "artifact_server", "")),
		ImageName:                   firstNonEmpty(stringValue(command.HardwareMetadata, "image_name", ""), command.ImageID),
		Mask:                        networkString(command.NetworkConfig, "mask", "netmask"),
		IpAddress:                   networkString(command.NetworkConfig, "ip", "address", "ip_address"),
		Password:                    firstNonEmpty(a.InstallPassword, stringValue(command.HardwareMetadata, "install_password", "")),
		SSH:                         ssh,
		Hostname:                    command.Hostname,
		HypervisorID:                firstNonEmpty(stringValue(command.HardwareMetadata, "hypervisor_id", ""), command.ServerID),
		ControlPlaneGRPCEndpoint:    stringValue(command.HardwareMetadata, "control_plane_grpc_endpoint", "172.200.0.1:9090"),
		ControlPlaneGRPCTLS:         stringValue(command.HardwareMetadata, "control_plane_grpc_tls", "false"),
		ControlPlaneGRPCAuthority:   stringValue(command.HardwareMetadata, "control_plane_grpc_authority", ""),
		ControlPlaneGRPCCAFile:      stringValue(command.HardwareMetadata, "control_plane_grpc_ca_file", "/etc/exos/pki/control-plane-ca.crt"),
		HypervisorClientTLSCertFile: stringValue(command.HardwareMetadata, "hypervisor_client_tls_cert_file", "/etc/exos/pki/hypervisor.crt"),
		HypervisorClientTLSKeyFile:  stringValue(command.HardwareMetadata, "hypervisor_client_tls_key_file", "/etc/exos/pki/hypervisor.key"),
		HypervisorAgentToken:        stringValue(command.HardwareMetadata, "hypervisor_agent_token", ""),
		WGInterface:                 stringValue(command.HardwareMetadata, "wg_interface", "wg0"),
		WGAddress:                   stringValue(command.HardwareMetadata, "wg_address", ""),
		WGListenPort:                stringValue(command.HardwareMetadata, "wg_listen_port", "51820"),
		WGPrivateKey:                stringValue(command.HardwareMetadata, "wg_private_key", ""),
		WGPeerPublicKey:             stringValue(command.HardwareMetadata, "wg_peer_public_key", ""),
		WGPeerEndpoint:              stringValue(command.HardwareMetadata, "wg_peer_endpoint", "vpn-1.exos.tech:51820"),
		WGPeerAllowedIPs:            stringValue(command.HardwareMetadata, "wg_peer_allowed_ips", "172.200.0.1/32"),
		SSHAllowedCIDR:              stringValue(command.HardwareMetadata, "ssh_allowed_cidr", "34.48.27.200/32"),
		ControlPlaneCAB64:           stringValue(command.HardwareMetadata, "control_plane_ca_pem_b64", ""),
		HypervisorClientCertB64:     stringValue(command.HardwareMetadata, "hypervisor_client_cert_pem_b64", ""),
		HypervisorClientKeyB64:      stringValue(command.HardwareMetadata, "hypervisor_client_key_pem_b64", ""),
	}, nil
}

type KubernetesBMCAdapter struct {
	Applier   ManifestApplier
	Namespace string
}

func NewKubernetesBMCAdapter(applier ManifestApplier, namespace string) *KubernetesBMCAdapter {
	if namespace == "" {
		namespace = "tink"
	}
	return &KubernetesBMCAdapter{Applier: applier, Namespace: namespace}
}

func (a *KubernetesBMCAdapter) PowerOn(ctx context.Context, serverID string) error {
	return a.applyJob(ctx, serverID, "turnon.yaml", "power-on")
}

func (a *KubernetesBMCAdapter) PowerOff(ctx context.Context, serverID string) error {
	return a.applyJob(ctx, serverID, "turnoff.yaml", "power-off")
}

func (a *KubernetesBMCAdapter) PowerCycle(ctx context.Context, serverID string) error {
	return a.applyJob(ctx, serverID, "restart.yaml", "power-cycle")
}

func (a *KubernetesBMCAdapter) Reset(ctx context.Context, serverID string) error {
	return a.PowerCycle(ctx, serverID)
}

func (a *KubernetesBMCAdapter) SetOneTimePXEBoot(ctx context.Context, serverID string) error {
	return a.applyJob(ctx, serverID, "provision.yaml", "pxe-boot")
}

func (a *KubernetesBMCAdapter) PowerState(ctx context.Context, serverID string) (string, error) {
	return "unknown", nil
}

func (a *KubernetesBMCAdapter) CheckBMC(ctx context.Context, serverID string) (bool, error) {
	_, err := a.Applier.GetJSON(ctx, "machines.bmc.tinkerbell.org", dnsName(serverID), a.Namespace)
	return err == nil, err
}

func (a *KubernetesBMCAdapter) applyJob(ctx context.Context, serverID, templateName, action string) error {
	data := tinkerbellTemplateData{
		Namespace:  a.Namespace,
		ServerName: dnsName(serverID),
		JobName:    dnsName(fmt.Sprintf("%s-%s-%d", action, serverID, time.Now().UTC().Unix())),
	}
	rendered, err := renderTemplates(data, templateName)
	if err != nil {
		return err
	}
	return a.Applier.Apply(ctx, rendered)
}

type tinkerbellTemplateData struct {
	Namespace                   string
	ServerName                  string
	WorkflowName                string
	TemplateName                string
	AuthName                    string
	IpmiIp                      string
	IpmiUsername                string
	IpmiPassword                string
	DiskName                    string
	Address                     string
	Gateway                     string
	Netmask                     string
	MAC                         string
	Binary                      string
	CustomIpxe                  string
	ArtifactServer              string
	ImageName                   string
	Mask                        string
	IpAddress                   string
	Password                    string
	SSH                         string
	Hostname                    string
	JobName                     string
	HypervisorID                string
	ControlPlaneGRPCEndpoint    string
	ControlPlaneGRPCTLS         string
	ControlPlaneGRPCAuthority   string
	ControlPlaneGRPCCAFile      string
	HypervisorClientTLSCertFile string
	HypervisorClientTLSKeyFile  string
	HypervisorAgentToken        string
	WGInterface                 string
	WGAddress                   string
	WGListenPort                string
	WGPrivateKey                string
	WGPeerPublicKey             string
	WGPeerEndpoint              string
	WGPeerAllowedIPs            string
	SSHAllowedCIDR              string
	ControlPlaneCAB64           string
	HypervisorClientCertB64     string
	HypervisorClientKeyB64      string
}

func renderTemplates(data tinkerbellTemplateData, names ...string) ([]byte, error) {
	var out bytes.Buffer
	for i, name := range names {
		if i > 0 {
			out.WriteString("\n---\n")
		}
		tpl, err := template.ParseFS(tinkerbellTemplates, "templates/"+name)
		if err != nil {
			return nil, err
		}
		if err := tpl.Execute(&out, data); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

var dnsInvalid = regexp.MustCompile(`[^a-z0-9-]+`)

func dnsName(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "_", "-")
	value = dnsInvalid.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "server"
	}
	if len(value) > 63 {
		value = value[:63]
		value = strings.TrimRight(value, "-")
	}
	return value
}

func stringValue(values map[string]any, key, fallback string) string {
	if values == nil {
		return fallback
	}
	if value, ok := values[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func networkString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func cleanEndpoint(value string) string {
	value = strings.TrimPrefix(value, "redfish://")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	return strings.TrimSuffix(value, "/")
}
