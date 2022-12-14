/*
Copyright 2021 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/exec"
)

const (
	sysctlBase = "/proc/sys"
)

// Interface is an injectable interface for running sysctl commands.
type sysInterface interface {
	// GetSysctl returns the value for the specified sysctl setting
	GetSysctl(sysctl string) (int, error)
	// SetSysctl modifies the specified sysctl flag to the new value
	SetSysctl(sysctl string, newVal int) error
}

// New returns a new Interface for accessing sysctl
func NewSysInterface() sysInterface {
	return &procSysctl{}
}

// procSysctl implements Interface by reading and writing files under /proc/sys
type procSysctl struct {
}

// GetSysctl returns the value for the specified sysctl setting
func (*procSysctl) GetSysctl(sysctl string) (int, error) {
	data, err := ioutil.ReadFile(path.Join(sysctlBase, sysctl))
	if err != nil {
		return -1, err
	}
	val, err := strconv.Atoi(strings.Trim(string(data), " \n"))
	if err != nil {
		return -1, err
	}
	return val, nil
}

// SetSysctl modifies the specified sysctl flag to the new value
func (*procSysctl) SetSysctl(sysctl string, newVal int) error {
	return ioutil.WriteFile(path.Join(sysctlBase, sysctl), []byte(strconv.Itoa(newVal)), 0640)
}

// EnsureSysctl sets a kernel sysctl to a given numeric value.
func EnsureSysctl(sysctl sysInterface, name string, newVal int) error {
	if oldVal, _ := sysctl.GetSysctl(name); oldVal != newVal {
		if err := sysctl.SetSysctl(name, newVal); err != nil {
			return fmt.Errorf("can't set sysctl %s to %d: %v", name, newVal, err)
		}
		klog.V(1).Info("Changed sysctl", "name", name, "before", oldVal, "after", newVal)
	}
	return nil
}

// KernelHandler can handle the current installed kernel modules.
type KernelHandler interface {
	GetModules() ([]string, error)
	GetKernelVersion() (string, error)
}

// LinuxKernelHandler implements KernelHandler interface.
type LinuxKernelHandler struct {
	executor exec.Interface
}

// NewLinuxKernelHandler initializes LinuxKernelHandler with exec.
func NewLinuxKernelHandler() *LinuxKernelHandler {
	return &LinuxKernelHandler{
		executor: exec.New(),
	}
}

// GetModules returns all installed kernel modules.
func (handle *LinuxKernelHandler) GetModules() ([]string, error) {
	// Check whether IPVS required kernel modules are built-in
	kernelVersionStr, err := handle.GetKernelVersion()
	if err != nil {
		return nil, err
	}
	kernelVersion, err := version.ParseGeneric(kernelVersionStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing kernel version %q: %v", kernelVersionStr, err)
	}
	ipvsModules := GetRequiredIPVSModules(kernelVersion)

	var bmods, lmods []string

	// Find out loaded kernel modules. If this is a full static kernel it will try to verify if the module is compiled using /boot/config-KERNELVERSION
	modulesFile, err := os.Open("/proc/modules")
	if err == os.ErrNotExist {
		klog.Error(err, "Failed to read file /proc/modules, assuming this is a kernel without loadable modules support enabled")
		kernelConfigFile := fmt.Sprintf("/boot/config-%s", kernelVersionStr)
		kConfig, err := ioutil.ReadFile(kernelConfigFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read Kernel Config file %s with error %w", kernelConfigFile, err)
		}
		for _, module := range ipvsModules {
			if match, _ := regexp.Match("CONFIG_"+strings.ToUpper(module)+"=y", kConfig); match {
				bmods = append(bmods, module)
			}
		}
		return bmods, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read file /proc/modules with error %w", err)
	}
	defer modulesFile.Close()

	mods, err := getFirstColumn(modulesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to find loaded kernel modules: %v", err)
	}

	builtinModsFilePath := fmt.Sprintf("/lib/modules/%s/modules.builtin", kernelVersionStr)
	b, err := ioutil.ReadFile(builtinModsFilePath)
	if err != nil {
		klog.Error(err, "Failed to read builtin modules file, you can ignore this message when kube-proxy is running inside container without mounting /lib/modules", "filePath", builtinModsFilePath)
	}

	for _, module := range ipvsModules {
		if match, _ := regexp.Match(module+".ko", b); match {
			bmods = append(bmods, module)
		} else {
			// Try to load the required IPVS kernel modules if not built in
			err := handle.executor.Command("modprobe", "--", module).Run()
			if err != nil {
				klog.Info("Failed to load kernel module with modprobe, "+
					"you can ignore this message when kube-proxy is running inside container without mounting /lib/modules", "moduleName", module)
			} else {
				lmods = append(lmods, module)
			}
		}
	}

	mods = append(mods, bmods...)
	mods = append(mods, lmods...)
	return mods, nil
}

// GetKernelVersion returns currently running kernel version.
func (handle *LinuxKernelHandler) GetKernelVersion() (string, error) {
	kernelVersionFile := "/proc/sys/kernel/osrelease"
	fileContent, err := ioutil.ReadFile(kernelVersionFile)
	if err != nil {
		return "", fmt.Errorf("error reading osrelease file %q: %v", kernelVersionFile, err)
	}

	return strings.TrimSpace(string(fileContent)), nil
}

// getFirstColumn reads all the content from r into memory and return a
// slice which consists of the first word from each line.
func getFirstColumn(r io.Reader) ([]string, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")
	words := make([]string, 0, len(lines))
	for i := range lines {
		fields := strings.Fields(lines[i])
		if len(fields) > 0 {
			words = append(words, fields[0])
		}
	}
	return words, nil
}
