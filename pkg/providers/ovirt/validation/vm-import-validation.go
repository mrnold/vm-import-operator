package validation

import (
	"fmt"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/pkg/utils"

	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	otemplates "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/templates"
	validators "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"

	ovirtsdk "github.com/ovirt/go-ovirt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type action int

var logger = logf.Log.WithName("validation")

const (
	log   = 0
	warn  = 1
	block = 2

	warnReason  = string(v2vv1.MappingRulesVerificationReportedWarnings)
	errorReason = string(v2vv1.MappingRulesVerificationFailed)
	okReason    = string(v2vv1.MappingRulesVerificationCompleted)

	incompleteMappingRulesReason     = string(v2vv1.IncompleteMappingRules)
	validationReportedWarningsReason = string(v2vv1.ValidationReportedWarnings)
	validationCompletedReason        = string(v2vv1.ValidationCompleted)
)

var checkToAction = map[validators.CheckID]action{
	// NIC rules
	validators.NicInterfaceCheckID:       block,
	validators.NicOnBootID:               log,
	validators.NicPluggedID:              warn,
	validators.NicVNicPortMirroringID:    warn,
	validators.NicVNicCustomPropertiesID: warn,
	validators.NicVNicNetworkFilterID:    warn,
	validators.NicVNicQosID:              log,
	// Storage rules
	validators.DiskAttachmentsExistID:              block,
	validators.DiskAttachmentInterfaceID:           block,
	validators.DiskAttachmentLogicalNameID:         log,
	validators.DiskAttachmentPassDiscardID:         log,
	validators.DiskAttachmentUsesScsiReservationID: block,
	validators.DiskInterfaceID:                     block,
	validators.DiskLogicalNameID:                   log,
	validators.DiskUsesScsiReservationID:           block,
	validators.DiskBackupID:                        warn,
	validators.DiskLunStorageID:                    block,
	validators.DiskPropagateErrorsID:               log,
	validators.DiskWipeAfterDeleteID:               log,
	validators.DiskStatusID:                        block,
	validators.DiskStoragaTypeID:                   block,
	validators.DiskSgioID:                          block,
	// VM rules
	validators.VMBiosBootMenuID:                  log,
	validators.VMStatusID:                        block,
	validators.VMBiosTypeID:                      block,
	validators.VMBiosTypeQ35SecureBootID:         warn,
	validators.VMCpuArchitectureID:               block,
	validators.VMCpuTuneID:                       warn,
	validators.VMCpuSharesID:                     log,
	validators.VMCustomEmulatedMachine:           log,
	validators.VMCustomPropertiesID:              warn,
	validators.VMDisplayTypeID:                   log,
	validators.VMHasIllegalImagesID:              block,
	validators.VMHighAvailabilityPriorityID:      log,
	validators.VMIoThreadsID:                     warn,
	validators.VMMemoryPolicyBallooningID:        log,
	validators.VMMemoryPolicyOvercommitPercentID: log,
	validators.VMMemoryPolicyGuaranteedID:        log,
	validators.VMMemoryTemplateLimitID:           block,
	validators.VMMigrationID:                     log,
	validators.VMMigrationDowntimeID:             log,
	validators.VMNumaTuneModeID:                  warn,
	validators.VMOriginID:                        block,
	validators.VMPlacementPolicyAffinityID:       block,
	validators.VMRngDeviceSourceID:               log,
	validators.VMSoundcardEnabledID:              warn,
	validators.VMStartPausedID:                   log,
	validators.VMStorageErrorResumeBehaviourID:   log,
	validators.VMTunnelMigrationID:               warn,
	validators.VMUsbID:                           block,
	validators.VMGraphicConsolesID:               log,
	validators.VMHostDevicesID:                   log,
	validators.VMReportedDevicesID:               log,
	validators.VMQuotaID:                         log,
	validators.VMWatchdogsID:                     block,
	validators.VMCdromsID:                        log,
	validators.VMFloppiesID:                      log,
	validators.VMTimezoneID:                      block,
	// Network mapping validation
	validators.NetworkConfig:               block,
	validators.NetworkTargetID:             block,
	validators.NetworkMappingID:            block,
	validators.NetworkMultiplePodTargetsID: block,
	validators.NetworkTypeID:               block,
	// Storage mapping validation
	validators.StorageTargetID:           block,
	validators.DiskTargetID:              block,
	validators.StorageTargetDefaultClass: warn,
}

// Validator validates different properties of a VM
type Validator interface {
	ValidateVM(vm *ovirtsdk.Vm, finder *otemplates.TemplateFinder) []validators.ValidationFailure
	ValidateDiskStatus(diskAttachment ovirtsdk.DiskAttachment) bool
	ValidateDiskAttachments(diskAttachments []*ovirtsdk.DiskAttachment) []validators.ValidationFailure
	ValidateNics(nics []*ovirtsdk.Nic) []validators.ValidationFailure
	ValidateNetworkMapping(nics []*ovirtsdk.Nic, mapping *[]v2vv1.NetworkResourceMappingItem, crNamespace string) []validators.ValidationFailure
	ValidateStorageMapping(
		attachments []*ovirtsdk.DiskAttachment,
		storageMapping *[]v2vv1.StorageResourceMappingItem,
		diskMappings *[]v2vv1.StorageResourceMappingItem,
	) []validators.ValidationFailure
}

// VirtualMachineImportValidator validates VirtualMachineImport object
type VirtualMachineImportValidator struct {
	Validator Validator
}

// NewVirtualMachineImportValidator creates ready-to-use NewVirtualMachineImportValidator
func NewVirtualMachineImportValidator(validator Validator) VirtualMachineImportValidator {
	return VirtualMachineImportValidator{
		Validator: validator,
	}
}

// Validate validates whether VM described in VirtualMachineImport can be imported
func (validator *VirtualMachineImportValidator) Validate(vm *ovirtsdk.Vm, vmiCrName *types.NamespacedName, mappings *v2vv1.OvirtMappings, finder *otemplates.TemplateFinder) []v2vv1.VirtualMachineImportCondition {
	var validationResults []v2vv1.VirtualMachineImportCondition
	mappingsCheckResult := validator.validateMappings(vm, mappings, vmiCrName)
	validationResults = append(validationResults, mappingsCheckResult)

	failures := validator.Validator.ValidateVM(vm, finder)
	if nics, ok := vm.Nics(); ok {

		failures = append(failures, validator.Validator.ValidateNics(nics.Slice())...)
	}
	if das, ok := vm.DiskAttachments(); ok {
		failures = append(failures, validator.Validator.ValidateDiskAttachments(das.Slice())...)
	}
	rulesCheckResult := validator.processValidationFailures(failures, vmiCrName)
	validationResults = append(validationResults, rulesCheckResult)
	return validationResults
}

func (validator *VirtualMachineImportValidator) validateMappings(vm *ovirtsdk.Vm, mappings *v2vv1.OvirtMappings, vmiCrName *types.NamespacedName) v2vv1.VirtualMachineImportCondition {
	var failures []validators.ValidationFailure

	if nics, ok := vm.Nics(); ok {
		nSlice := nics.Slice()
		failures = append(failures, validator.Validator.ValidateNetworkMapping(nSlice, mappings.NetworkMappings, vmiCrName.Namespace)...)
	}
	if attachments, ok := vm.DiskAttachments(); ok {
		das := attachments.Slice()
		failures = append(failures, validator.Validator.ValidateStorageMapping(das, mappings.StorageMappings, mappings.DiskMappings)...)
	}

	return validator.processMappingValidationFailures(failures, vmiCrName)
}

func (validator *VirtualMachineImportValidator) processMappingValidationFailures(failures []validators.ValidationFailure, vmiCrName *types.NamespacedName) v2vv1.VirtualMachineImportCondition {
	warnMessage, errorMessage := validator.processFailures(failures, vmiCrName)
	if errorMessage != "" {
		return conditions.NewCondition(v2vv1.Valid, incompleteMappingRulesReason, errorMessage, v1.ConditionFalse)
	} else if warnMessage != "" {
		return conditions.NewCondition(v2vv1.Valid, validationReportedWarningsReason, warnMessage, v1.ConditionTrue)
	}
	return conditions.NewCondition(v2vv1.Valid, validationCompletedReason, "Validation completed successfully", v1.ConditionTrue)
}

func (validator *VirtualMachineImportValidator) processValidationFailures(failures []validators.ValidationFailure, vmiCrName *types.NamespacedName) v2vv1.VirtualMachineImportCondition {
	warnMessage, errorMessage := validator.processFailures(failures, vmiCrName)
	if errorMessage != "" {
		return conditions.NewCondition(v2vv1.MappingRulesVerified, errorReason, errorMessage, v1.ConditionFalse)
	} else if warnMessage != "" {
		return conditions.NewCondition(v2vv1.MappingRulesVerified, warnReason, warnMessage, v1.ConditionTrue)
	}
	return conditions.NewCondition(v2vv1.MappingRulesVerified, okReason, "All mapping rules checks passed", v1.ConditionTrue)
}

func (validator *VirtualMachineImportValidator) processFailures(failures []validators.ValidationFailure, vmiCrName *types.NamespacedName) (string, string) {
	var warnMessage, errorMessage string
	for _, failure := range failures {
		switch checkToAction[failure.ID] {
		case log:
			logger.Info(fmt.Sprintf("Validation information for %v: %v", vmiCrName, failure))
		case warn:
			warnMessage = utils.WithMessage(warnMessage, failure.Message)
		case block:
			errorMessage = utils.WithMessage(errorMessage, failure.Message)
		}
	}
	return warnMessage, errorMessage
}
