// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package instance

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/juju/utils/arch"

	"github.com/juju/juju/network"
	"github.com/juju/juju/status"
)

// An instance Id is a provider-specific identifier associated with an
// instance (physical or virtual machine allocated in the provider).
type Id string

// InstanceStatus represents the status for a provider instance.
type InstanceStatus struct {
	Status  status.Status
	Message string
}

// UnknownId can be used to explicitly specify the instance ID does not matter.
const UnknownId Id = ""

// Instance represents the the realization of a machine in state.
type Instance interface {
	// Id returns a provider-generated identifier for the Instance.
	Id() Id

	// Status returns the provider-specific status for the instance.
	Status() InstanceStatus

	// Addresses returns a list of hostnames or ip addresses
	// associated with the instance.
	Addresses() ([]network.Address, error)
}

// InstanceFirewaller provides instance-level firewall functionality
type InstanceFirewaller interface {
	// OpenPorts opens the given port ranges on the instance, which
	// should have been started with the given machine id.
	OpenPorts(machineId string, rules []network.IngressRule) error

	// ClosePorts closes the given port ranges on the instance, which
	// should have been started with the given machine id.
	ClosePorts(machineId string, rules []network.IngressRule) error

	// IngressRules returns the set of ingress rules for the instance,
	// which should have been applied to the given machine id. The
	// rules are returned as sorted by network.SortIngressRules().
	// It is expected that there be only one ingress rule result for a given
	// port range - the rule's SourceCIDRs will contain all applicable source
	// address rules for that port range.
	IngressRules(machineId string) ([]network.IngressRule, error)
}

// HardwareCharacteristics represents the characteristics of the instance (if known).
// Attributes that are nil are unknown or not supported.
type HardwareCharacteristics struct {
	// Arch is the architecture of the processor.
	Arch *string `json:"arch,omitempty" yaml:"arch,omitempty"`

	// Mem is the size of RAM in megabytes.
	Mem *uint64 `json:"mem,omitempty" yaml:"mem,omitempty"`

	// RootDisk is the size of the disk in megabytes.
	RootDisk *uint64 `json:"root-disk,omitempty" yaml:"rootdisk,omitempty"`

	// CpuCores is the number of logical cores the processor has.
	CpuCores *uint64 `json:"cpu-cores,omitempty" yaml:"cpucores,omitempty"`

	// CpuPower is a relative representation of the speed of the processor.
	CpuPower *uint64 `json:"cpu-power,omitempty" yaml:"cpupower,omitempty"`

	// Tags is a list of strings that identify the machine.
	Tags *[]string `json:"tags,omitempty" yaml:"tags,omitempty"`

	// AvailabilityZone defines the zone in which the machine resides.
	AvailabilityZone *string `json:"availability-zone,omitempty" yaml:"availabilityzone,omitempty"`
}

func (hc HardwareCharacteristics) String() string {
	var strs []string
	if hc.Arch != nil {
		strs = append(strs, fmt.Sprintf("arch=%s", *hc.Arch))
	}
	if hc.CpuCores != nil {
		strs = append(strs, fmt.Sprintf("cores=%d", *hc.CpuCores))
	}
	if hc.CpuPower != nil {
		strs = append(strs, fmt.Sprintf("cpu-power=%d", *hc.CpuPower))
	}
	if hc.Mem != nil {
		strs = append(strs, fmt.Sprintf("mem=%dM", *hc.Mem))
	}
	if hc.RootDisk != nil {
		strs = append(strs, fmt.Sprintf("root-disk=%dM", *hc.RootDisk))
	}
	if hc.Tags != nil && len(*hc.Tags) > 0 {
		strs = append(strs, fmt.Sprintf("tags=%s", strings.Join(*hc.Tags, ",")))
	}
	if hc.AvailabilityZone != nil && *hc.AvailabilityZone != "" {
		strs = append(strs, fmt.Sprintf("availability-zone=%s", *hc.AvailabilityZone))
	}
	return strings.Join(strs, " ")
}

// MustParseHardware constructs a HardwareCharacteristics from the supplied arguments,
// as Parse, but panics on failure.
func MustParseHardware(args ...string) HardwareCharacteristics {
	hc, err := ParseHardware(args...)
	if err != nil {
		panic(err)
	}
	return hc
}

// ParseHardware constructs a HardwareCharacteristics from the supplied arguments,
// each of which must contain only spaces and name=value pairs. If any
// name is specified more than once, an error is returned.
func ParseHardware(args ...string) (HardwareCharacteristics, error) {
	hc := HardwareCharacteristics{}
	for _, arg := range args {
		raws := strings.Split(strings.TrimSpace(arg), " ")
		for _, raw := range raws {
			if raw == "" {
				continue
			}
			if err := hc.setRaw(raw); err != nil {
				return HardwareCharacteristics{}, err
			}
		}
	}
	return hc, nil
}

// setRaw interprets a name=value string and sets the supplied value.
func (hc *HardwareCharacteristics) setRaw(raw string) error {
	eq := strings.Index(raw, "=")
	if eq <= 0 {
		return fmt.Errorf("malformed characteristic %q", raw)
	}
	name, str := raw[:eq], raw[eq+1:]
	var err error
	switch name {
	case "arch":
		err = hc.setArch(str)
	case "cores":
		err = hc.setCpuCores(str)
	case "cpu-power":
		err = hc.setCpuPower(str)
	case "mem":
		err = hc.setMem(str)
	case "root-disk":
		err = hc.setRootDisk(str)
	case "tags":
		err = hc.setTags(str)
	case "availability-zone":
		err = hc.setAvailabilityZone(str)
	default:
		return fmt.Errorf("unknown characteristic %q", name)
	}
	if err != nil {
		return fmt.Errorf("bad %q characteristic: %v", name, err)
	}
	return nil
}

func (hc *HardwareCharacteristics) setArch(str string) error {
	if hc.Arch != nil {
		return fmt.Errorf("already set")
	}
	if str != "" && !arch.IsSupportedArch(str) {
		return fmt.Errorf("%q not recognized", str)
	}
	hc.Arch = &str
	return nil
}

func (hc *HardwareCharacteristics) setCpuCores(str string) (err error) {
	if hc.CpuCores != nil {
		return fmt.Errorf("already set")
	}
	hc.CpuCores, err = parseUint64(str)
	return
}

func (hc *HardwareCharacteristics) setCpuPower(str string) (err error) {
	if hc.CpuPower != nil {
		return fmt.Errorf("already set")
	}
	hc.CpuPower, err = parseUint64(str)
	return
}

func (hc *HardwareCharacteristics) setMem(str string) (err error) {
	if hc.Mem != nil {
		return fmt.Errorf("already set")
	}
	hc.Mem, err = parseSize(str)
	return
}

func (hc *HardwareCharacteristics) setRootDisk(str string) (err error) {
	if hc.RootDisk != nil {
		return fmt.Errorf("already set")
	}
	hc.RootDisk, err = parseSize(str)
	return
}

func (hc *HardwareCharacteristics) setTags(str string) (err error) {
	if hc.Tags != nil {
		return fmt.Errorf("already set")
	}
	hc.Tags = parseTags(str)
	return
}

func (hc *HardwareCharacteristics) setAvailabilityZone(str string) error {
	if hc.AvailabilityZone != nil {
		return fmt.Errorf("already set")
	}
	if str != "" {
		hc.AvailabilityZone = &str
	}
	return nil
}

// parseTags returns the tags in the value s
func parseTags(s string) *[]string {
	if s == "" {
		return &[]string{}
	}
	tags := strings.Split(s, ",")
	return &tags
}

func parseUint64(str string) (*uint64, error) {
	var value uint64
	if str != "" {
		if val, err := strconv.ParseUint(str, 10, 64); err != nil {
			return nil, fmt.Errorf("must be a non-negative integer")
		} else {
			value = uint64(val)
		}
	}
	return &value, nil
}

func parseSize(str string) (*uint64, error) {
	var value uint64
	if str != "" {
		mult := 1.0
		if m, ok := mbSuffixes[str[len(str)-1:]]; ok {
			str = str[:len(str)-1]
			mult = m
		}
		val, err := strconv.ParseFloat(str, 64)
		if err != nil || val < 0 {
			return nil, fmt.Errorf("must be a non-negative float with optional M/G/T/P suffix")
		}
		val *= mult
		value = uint64(math.Ceil(val))
	}
	return &value, nil
}

var mbSuffixes = map[string]float64{
	"M": 1,
	"G": 1024,
	"T": 1024 * 1024,
	"P": 1024 * 1024 * 1024,
}
