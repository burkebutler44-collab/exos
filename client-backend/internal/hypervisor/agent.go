package hypervisor

import (
	"context"
	"log"
	"time"

	relayv1 "relay/client-backend/gen/go/relay/v1"
)

type Agent struct {
	Config     Config
	Hypervisor Hypervisor
	Reporter   Reporter
	Security   *HostSecurityManager
	Commands   CommandStateStore
}

func (a *Agent) Run(ctx context.Context) error {
	if a.Security != nil {
		if err := a.Security.Apply(ctx); err != nil {
			return err
		}
	}
	initial, err := a.Hypervisor.Snapshot(ctx)
	if err != nil {
		log.Printf("initial hypervisor snapshot failed: %v", err)
	} else if err := a.Reporter.ReportSnapshot(ctx, initial); err != nil {
		log.Printf("initial hypervisor report failed: %v", err)
	}
	if initial.HypervisorID != "" {
		if streamer, ok := a.Reporter.(CommandStreamer); ok {
			go func() {
				if err := streamer.RunCommandStream(ctx, initial, a.handleProtoCommand); err != nil && ctx.Err() == nil {
					log.Printf("hypervisor command stream ended: %v", err)
				}
			}()
		}
	}
	ticker := time.NewTicker(a.Config.ReportInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.reportOnce(ctx); err != nil {
				log.Printf("hypervisor report failed: %v", err)
			}
		}
	}
}

func (a *Agent) handleProtoCommand(ctx context.Context, command *relayv1.HypervisorCommand) *relayv1.VMCommandResult {
	if command == nil {
		return nil
	}
	commandID := command.GetCommandId()
	if a.Commands != nil && commandID != "" {
		if stored, ok, err := a.Commands.Get(commandID); err == nil && ok {
			return &relayv1.VMCommandResult{
				CommandId: commandID,
				Name:      stored.Name,
				Status:    stored.Status,
				Message:   stored.Message,
			}
		} else if err != nil {
			log.Printf("hypervisor command state lookup failed id=%s: %v", commandID, err)
		}
	}
	var result VMCommandResult
	var err error
	switch payload := command.GetCommand().(type) {
	case *relayv1.HypervisorCommand_CreateVm:
		create := payload.CreateVm
		result, err = a.CreateVM(ctx, VMCreateRequest{
			Name:             create.GetName(),
			VCPUs:            int(create.GetVcpus()),
			MemoryMiB:        int(create.GetMemoryMib()),
			DiskGiB:          int(create.GetDiskGib()),
			ImagePath:        create.GetImagePath(),
			CloudInitISOPath: create.GetCloudInitIsoPath(),
			NetworkName:      create.GetNetworkName(),
			MACAddress:       create.GetMacAddress(),
			Metadata:         create.GetMetadata(),
		})
	case *relayv1.HypervisorCommand_DeleteVm:
		deleteVM := payload.DeleteVm
		result, err = a.DeleteVM(ctx, VMDeleteRequest{
			Name:          deleteVM.GetName(),
			RemoveDisk:    deleteVM.GetRemoveDisk(),
			ForcePowerOff: deleteVM.GetForcePowerOff(),
		})
	case *relayv1.HypervisorCommand_PowerVm:
		power := payload.PowerVm
		switch power.GetAction() {
		case "start":
			result, err = a.StartVM(ctx, power.GetName())
		case "stop", "shutdown":
			result, err = a.StopVM(ctx, power.GetName())
		default:
			err = errUnsupportedPowerAction(power.GetAction())
			result = VMCommandResult{Name: power.GetName(), Status: "failed"}
		}
	default:
		result = VMCommandResult{Status: "ignored", Message: "unknown command payload"}
	}
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
	}
	protoResult := &relayv1.VMCommandResult{
		CommandId: command.GetCommandId(),
		Name:      result.Name,
		Status:    result.Status,
		Message:   result.Message,
	}
	if a.Commands != nil && commandID != "" {
		if err := a.Commands.Put(commandID, StoredCommandResult{
			CommandID: commandID,
			Name:      protoResult.GetName(),
			Status:    protoResult.GetStatus(),
			Message:   protoResult.GetMessage(),
		}); err != nil {
			log.Printf("hypervisor command state save failed id=%s: %v", commandID, err)
		}
	}
	return protoResult
}

type unsupportedPowerAction string

func (e unsupportedPowerAction) Error() string { return "unsupported power action: " + string(e) }

func errUnsupportedPowerAction(action string) error { return unsupportedPowerAction(action) }

func (a *Agent) reportOnce(ctx context.Context) error {
	snapshot, err := a.Hypervisor.Snapshot(ctx)
	if err != nil {
		return err
	}
	return a.Reporter.ReportSnapshot(ctx, snapshot)
}

func (a *Agent) CreateVM(ctx context.Context, req VMCreateRequest) (VMCommandResult, error) {
	result, err := a.Hypervisor.CreateVM(ctx, req)
	if err != nil {
		return result, err
	}
	_ = a.reportOnce(ctx)
	return result, nil
}

func (a *Agent) DeleteVM(ctx context.Context, req VMDeleteRequest) (VMCommandResult, error) {
	result, err := a.Hypervisor.DeleteVM(ctx, req)
	if err != nil {
		return result, err
	}
	_ = a.reportOnce(ctx)
	return result, nil
}

func (a *Agent) StartVM(ctx context.Context, name string) (VMCommandResult, error) {
	result, err := a.Hypervisor.StartVM(ctx, name)
	if err != nil {
		return result, err
	}
	_ = a.reportOnce(ctx)
	return result, nil
}

func (a *Agent) StopVM(ctx context.Context, name string) (VMCommandResult, error) {
	result, err := a.Hypervisor.StopVM(ctx, name)
	if err != nil {
		return result, err
	}
	_ = a.reportOnce(ctx)
	return result, nil
}
