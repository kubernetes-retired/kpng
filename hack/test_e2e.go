/*
Test_e2e performs tests on the Kubernetes cluster. 
It creates a kubernetes cluster based in kind + kpng, and performs ginkgo tests on it. 

Usage:

	./test_e2e.go [-i ip_family] [-b backend]

The flags are:

	-i 
		Set ip_family(ipv4/ipv6/dual) name in the e2e test runs.
	-b 
		Set backend (iptables/nft/ipvs/ebpf/userspacelin/not-kpng/) name in the e2e test runs.
		"not-kpng" is used to be able to validate and compare results
*/
package main

import (
	"fmt"
	"os"
	"os/exec"
	"log"
	"bytes"
	"strconv"
	"strings"
	"path"
	"text/template"
) 

const (
	KindVersion="v0.17.0"
	KubernetesVersion="v1.25.3"
	KpngImageTagName="kpng:test_270323_0904" 
	ClusterCidrV4="10.1.0.0/16"
	ServiceClusterIpRangeV4="10.2.0.0/16"
	KindestNodeImage="docker.io/kindest/node"
	E2eK8sVersion="v1.25.3"
	KubeconfigTests="kubeconfig_tests.conf"
)
// System data
const (
	Namespace="kube-system"
	ServiceAccountName="kpng"
	ClusterRoleBindingName="kpng"
	ClusterRoleName="system:node-proxier"
	ConfigMapName="kpng"

	KpngServerAddress="unix:///k8s/proxy.sock"
	KpngDebugLevel="4"
)
// Ginkgo constants
const (
	GinkgoSkipTests="machinery|Feature|Federation|PerformanceDNS|Disruptive|Serial|LoadBalancer|KubeProxy|GCE|Netpol|NetworkPolicy"
	GinkgoFocus="\\[Conformance\\]|\\[sig-network\\]"
	GinkgoNumberOfNodes=25
	GinkgoProvider="local"
	GinkgoDumpLogsOnFailure=false
	GinkgoReportDir="artifacts/reports"
	GinkgoDisableLogDump=true
)

// KpngPath is the path to the kpng folder. TODO: is it possible to make it a constant? 
// ContainerEngine will contain the container engine being used(Currently docker or podman).
var kpngPath, containerEngine string

// ifErrorExit validate if previous command failed and show an error msg (if provided).
//
// If an error message is not provided, it will just exit.
// TODO: This function may no longer be necessary, as we most probable can only use log.Fatal(err). Replace ifErrorExit by log.Fatal
func ifErrorExit(errorMsg error) {
	if errorMsg != nil {
		log.Fatal(errorMsg)
	}
} 

// commandExist check if a binary(cmdTest) exists. It returns True or False 
func commandExist(cmdTest string) bool {
	// Command is a Bash builtin command. 
	// It execute a simple command or display information about commands.
	// "command -v" print a description of COMMAND similar to the `type' builtin
	cmdScript := "command -v " + cmdTest + " > /dev/null 2>&1"
	cmd := exec.Command("bash", "-c", cmdScript)
	err := cmd.Run()
	
	if err == nil {
		return true
	}
	return false
}

// addToPath add a directory to the $PATH env variable.
func addToPath(directory string) {
	_, err := os.Stat(directory)
	if err != nil && os.IsNotExist(err) {
		ifErrorExit(err)
	}

	fmt.Println("Adding the directory to $PATH env variable")
	fmt.Println()
	path := os.Getenv("PATH")
	fmt.Println("Current $PATH: ", path)

	// Check if the directory path is in the $PATH env variable. 
	// I believe the package regexp can be used. Couldn't figure out the right pattern. 
	// For now will use the following approach
	// TODO(Maybe!): Check using the regexp package
	pathSet := strings.Split(path, ":") 
	dirPathExist := false
	for _, s := range pathSet {
		if directory == s {
			dirPathExist = true
			break
		}
	}

	fmt.Println("New directory: ", directory)
	if !dirPathExist {
		fmt.Println("The directory is NOT in the $PATH env variable! :(")
		updatedPath := path + ":" + directory
		err = os.Setenv("PATH", updatedPath)
		ifErrorExit(err)
		fmt.Println(os.Getenv("PATH"))
		fmt.Println("The directory is NOW in the $PATH env variable! hooray:)")
	}

}

// setSysctl set a sysctl value to an attribute
func setSysctl(attribute string, value int) {
	var buffer bytes.Buffer
	cmd := exec.Command("sysct", "-n", attribute )
	cmd.Stdout = &buffer
	//result, err := cmd.Output() //cmd.Output() returns bytes. How to convert bytes to int?
	err := cmd.Run()
	ifErrorExit(err)

	resultStr := strings.TrimSpace(buffer.String())
	resultInt, err := strconv.Atoi(resultStr)
	
	fmt.Printf("Checking sysctls value: %d vs result: %d\n", value, resultInt)
	if value != resultInt {
		fmt.Printf("Setting: \"sysctl -w %s=%d\"\n", attribute, value)
		attributeValue := attribute + "=" + strconv.Itoa(value)
		cmd = exec.Command("sudo", "sysctl", "-w", attributeValue)
		err = cmd.Run()
		ifErrorExit(err)
	}
}

// setHostNetworkSettings prepare hosts network settings 
func setHostNetworkSettings(ipFamily string) {
	setSysctl("net.ipv4.ip_forward", 1)
	if ipFamily == "ipv6" {
		//TODO 
		fmt.Println("TODO :-)")
	}
}

// verifySysctlSetting verify that a sysctl attribute setting has a value.
func verifySysctlSetting(attribute string, value int) {
	// Get the current value of the attribute and store it in the result variable
	var buffer bytes.Buffer
	cmd := exec.Command("sysctl", "-n", attribute)
	cmd.Stdout = &buffer
	err := cmd.Run()
	ifErrorExit(err)

	result, err := strconv.Atoi(strings.TrimSpace(buffer.String()))
	ifErrorExit(err)
	if value != result {
		fmt.Printf("Failure: \"sysctl -n %s\" returned \"%d\", not \"%d\" as expected.\n", attribute, result, value)	
		os.Exit(1)
	}
}

// verifyHostNetworkSettings verify hosts network settings                                           
func verifyHostNetworkSettings(ipFamily string) {
	verifySysctlSetting("net.ipv4.ip_forward", 1)
} 

// setupKind setup kind in the installDirectory if not available in the operatingSystem.
func setupKind(installDirectory, operatingSystem string) {
	_, err := os.Stat(installDirectory)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(installDirectory + "/kind")
	if err == nil {
		fmt.Println("The kind tool is already set in your system.")
	} else if err != nil && os.IsNotExist(err) {
		fmt.Println()
		fmt.Println("Downloading kind ...")

		tmpFile, err := os.CreateTemp("/tmp", "kind_setup")
		ifErrorExit(err)
		defer os.Remove(tmpFile.Name()) // Clean up. QUESTION: As we will end up moving the temp file, is this necessary? 
	
		url := "https://kind.sigs.k8s.io/dl/" + KindVersion + "/kind-" + operatingSystem + "-amd64"
		fmt.Printf("Temp filename: %s\n", tmpFile.Name())
		cmd := exec.Command("curl", "-L", url, "-o", tmpFile.Name())	
		err = cmd.Run()
		ifErrorExit(err)
		// TODO: Find out how to show progress of the curl ongoing download, on the terminal
	
		cmd = exec.Command("sudo", "mv", tmpFile.Name(), installDirectory + "/kind")
		_ = cmd.Run()
		//ifErrorExit(err) 
		cmd = exec.Command("sudo", "chmod", "+rx", installDirectory + "/kind")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chown", "root.root", installDirectory + "/kind")
		
		fmt.Println("The kind tool is set.")		
	}
}

// setupKubectl setup kubectl for k8sVersion, in the installDirectory, if not available in the operatingSystem.
func setupKubectl(installDirectory, k8sVersion, operatingSystem string) {
 	// Check if the installation directory exist
	_, err := os.Stat(installDirectory)
	ifErrorExit(err)
	// If kubectl is not installed, install it.
	_, err = os.Stat(installDirectory + "/kubectl")
	if err == nil {
		fmt.Println("Kubectl is already installed in the System.")
	} else if err != nil && os.IsNotExist(err) {
		// Show message "Downloading kubectl ..."
		fmt.Println("Downloading kubectl ...") 
		// Create tem file
		tmpFile, err := os.CreateTemp(".", "kubectl_setup")
		ifErrorExit(err)
		// Download kubectl
		url := "https://dl.k8s.io/" + k8sVersion + "/bin/" + operatingSystem + "/amd64/kubectl"
		cmd := exec.Command("curl", "-L", url, "-o", tmpFile.Name())
		err = cmd.Run()
		ifErrorExit(err)
		//mv, chmod, chown 
		cmd = exec.Command("sudo", "mv", tmpFile.Name(), installDirectory + "/kubectl")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chmod", "+rx", installDirectory + "/kubectl")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chown", "root.root", installDirectory + "/kubectl")
		_ = cmd.Run()

		fmt.Println("The Kubectl tool is set.")
	}
}

// setupGinkgo setup ginkgo and e2e.test for k8sVersion in the installDirectory, if not available on the operatingSystem.
func setupGinkgo(installDirectory, k8sVersion, operatingSystem string) {
	//Create temp directory
	tmpDir, err := os.MkdirTemp(".", "ginkgo_setup_")  //I think this should only happen in case ginkgo and e2e.test are not installed. Fix later
	ifErrorExit(err)
	defer os.RemoveAll(tmpDir) //Clean up

	_, ginkgoExist := os.Stat(installDirectory + "/ginkgo")
	_, e2eTestExist := os.Stat(installDirectory + "/e2e.test")

	if os.IsNotExist(ginkgoExist) || os.IsNotExist(e2eTestExist) {
		fmt.Println("Downloading ginkgo and e2e.test ...")
		url := "https://dl.k8s.io/" + k8sVersion + "/kubernetes-test-" + operatingSystem + "-amd64.tar.gz"
		outFile := tmpDir + "/kubernetes-test-" + operatingSystem + "-amd64.tar.gz"

		cmd := exec.Command("curl", "-L", url, "-o", outFile)
		err = cmd.Run()
		ifErrorExit(err)

		tarFile := tmpDir + "/kubernetes-test-" + operatingSystem + "-amd64.tar.gz" 
		cmdString := "tar xvzf " + tarFile + " --directory " + installDirectory + " --strip-components=3 kubernetes/test/bin/ginkgo kubernetes/test/bin/e2e.test &> /dev/null"
		cmd = exec.Command("bash", "-c", cmdString)
		err = cmd.Run()
		ifErrorExit(err)

		fmt.Println("The tools ginko and e2e.test have been set up.")

	} else {
		fmt.Println("The tools ginko and e2e.test have already been set up.")
	}
}

// setupBpf2go install bpf2go binary.
func setupBpf2go(installDirectory string) {
	if ! commandExist("bpf2go") {
		if ! commandExist("go") {
			fmt.Println("Dependency not met: 'bpf2go' not installed and cannot install with 'go'")
			os.Exit(1)
		} 
		
		fmt.Println("'bpf2go' not found, installing with 'go'")
		_, err := os.Stat(installDirectory)
		if err != nil && os.IsNotExist(err) {
			log.Fatal(err)
		}
		// set GOBIN to binDirectory to ensure that binary is in search path	
		_ = os.Setenv("GOBIN", installDirectory)
		// remove GOPATH just to be sure
		_ = os.Setenv("GOPATH", "")

		cmd := exec.Command("go", "install", "github.com/cilium/ebpf/cmd/bpf2go@v0.9.2")
		err = cmd.Run()
		ifErrorExit(err)
	
		var buffer bytes.Buffer
		cmd = exec.Command("which", "bpf2go")
		cmd.Stdout = &buffer
		err = cmd.Run()
		ifErrorExit(err)
		fmt.Printf("The tool bpf2go is installed at: %s\n", buffer.String())	
	} 
} 

// InstallBinaries copy binaries from the net to the binaries directory.
// BinDirectory is the bin directory that will be created in the hackDir and where the binaries will be installed
// HackDirectory is the directory that contains the script test_e2e.go, and the binDirectory 
func installBinaries(binDirectory, k8sVersion, operatingSystem, hackDirectory string, ) {
	wd, err := os.Getwd()
	ifErrorExit(err)
	
	if wd != hackDirectory {
		err = os.Chdir(hackDirectory)
		ifErrorExit(err)
	}
	err = os.MkdirAll(binDirectory, 0755) 

	addToPath(binDirectory) 

	setupKind(binDirectory, operatingSystem)
	setupKubectl(binDirectory, k8sVersion, operatingSystem)
	setupGinkgo(binDirectory, k8sVersion, operatingSystem)
	setupBpf2go(binDirectory)
}

// detectContainerEngine detect container engine, by default it is docker but developers might   
// use real alternatives like podman. The project welcome both.
func detectContainerEngine() {
	containerEngine = "docker"
	cmd := exec.Command(containerEngine) 
	err := cmd.Run()
	if err != nil {
		// If docker is not available, let's check if podman exists
		containerEngine = "podman"
		cmd = exec.Command(containerEngine, "--help")
		err = cmd.Run() 
		if err != nil {
			fmt.Println("The e2e tests currently support docker and podman as the container engine. Please install either of them")
			os.Exit(1)
		}
	}
	fmt.Println("Detected Container Engine:", containerEngine)
}

// containerBuild build a container image for KPNG.
func containerBuild(containerFile string, ciMode bool) {
	// QUESTION to Jay: Is it necessary to have this variables in capital letters?
	
	// Running locally it's not necessary to show all info
	QuietMode := "--quiet"
	if ciMode == true {
		QuietMode = ""
	}

	_, err := os.Stat(containerFile)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	err = os.Chdir(kpngPath)
	if err != nil {
		log.Fatal(err)
	}

	if QuietMode == "" {
		cmd := exec.Command(containerEngine, "build", "-t", KpngImageTagName, "-f", containerFile, ".")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		cmd := exec.Command(containerEngine, "build", QuietMode, "-t", KpngImageTagName, "-f", containerFile, ".")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}

	err = os.Chdir(kpngPath + "/hack")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Image build and tag %s is set.\n", KpngImageTagName)
}

// setE2eDir set E2E directory.
func setE2eDir(e2eDir string) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	err = os.Chdir(kpngPath + "/hack")
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(e2eDir)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(e2eDir + "/artifacts") 
	if err == nil {
		fmt.Println("The directory \"artifacts\" already exist!")
	} else if err != nil && os.IsNotExist(err) {
		err = os.Mkdir(e2eDir + "/artifacts", 0755)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = os.Chdir(wd)
	if err != nil {
		log.Fatal(err)
	}
}

// prepareContainer prepare container based on the dockerfile. 
func prepareContainer(dockerfile string, ciMode bool) {
	detectContainerEngine()
	containerBuild(dockerfile, ciMode) 
}

// createCluster create a kind cluster.
func createCluster(clusterName, binDir, ipFamily, artifactsDirectory string, ciMode bool) {
	type KindConfigData struct {
		IpFamily string
		ClusterCidr string
		ServiceClusterIpRange string
	}

	const KindConfigTemplate = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
    ipFamily: {{ .IpFamily }}
    podSubnet: {{ .ClusterCidr }}
    serviceSubnet: {{ .ServiceClusterIpRange }}
nodes:
- role: control-plane
- role: worker
- role: worker	
`	
	var (
		kind = binDir + "/kind"
		kubectl = binDir + "/kubectl"
	)


	_, err := os.Stat(binDir)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(kind)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(kubectl)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	cmdString := kind + " get clusters | grep -q " + clusterName
	cmd := exec.Command("bash", "-c", cmdString)
	err = cmd.Run()
	if err == nil {
		cmdString = kind + " delete cluster --name " + clusterName
		cmd = exec.Command("bash", "-c", cmdString)
		err = cmd.Run()
		if err != nil {
			log.Fatalf("Cannot delete cluster %s\n", clusterName, err)
		}
		fmt.Printf("Previous cluster %s deleted.\n", clusterName)
	}

	// Default Log level for all components in test clusters
	kindClusterLogLevel := os.Getenv("KIND_CLUSTER_LOG_LEVEL")
	if strings.TrimSpace(kindClusterLogLevel) == "" {
		kindClusterLogLevel = "4"
	}
	kindLogLevel := "-v3"
	if ciMode == true {
		kindLogLevel = "-v7"
	}
/*
	// Potentially enable --logging-format
	scheduler_extra_args := "\"v\": \"" + kindClusterLogLevel + "\""
	controllerManager_extra_args := "\"v\": \"" + kindClusterLogLevel + "\""
	apiServer_extra_args := "\"v\": \"" + kindClusterLogLevel + "\""
*/
	var clusterCidr, serviceClusterIpRange string
	
	switch ipFamily {
	case "ipv4":
		clusterCidr = ClusterCidrV4
        serviceClusterIpRange = ServiceClusterIpRangeV4
	}

	fmt.Printf("Preparing to setup %s cluster ...\n", clusterName)
	// Create cluster
	// Create the config file
	tmpl, err := template.New("kind_config_template").Parse(KindConfigTemplate)	
	if err != nil {
		log.Fatalf("Unable to create template %v", err)
	}
	kindConfigTemplateData := KindConfigData {
		IpFamily:			 	ipFamily,
		ClusterCidr:			clusterCidr,
		ServiceClusterIpRange:	serviceClusterIpRange,
	}

	yamlDestPath := artifactsDirectory + "/kind-config.yaml"
	f, err := os.Create(yamlDestPath)
	if err != nil {
		log.Fatal(err)
	}
	err = tmpl.Execute(f, kindConfigTemplateData)
	if err != nil {
		log.Fatal(err)
	}
	
	cmdStringSet := []string {
		kind + " create cluster ",
		"--name " + clusterName,
		" --image " + KindestNodeImage + ":" + E2eK8sVersion,
		" --retain",
		" --wait=20m ",
		kindLogLevel,
		" --config=" + artifactsDirectory + "/kind-config.yaml",
	}
	cmdString = ""
	for _, s := range cmdStringSet {
		cmdString += s
	}
	
	cmd = exec.Command("bash", "-c", cmdString)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Can not create kind cluster %s\n", clusterName, err)
	} 

	// Patch kube-proxy to set the verbosity level
	cmdStringSet = []string {
		kubectl + " patch -n kube-system daemonset/kube-proxy ",
		"--type='json' ",
		"-p='[{\"op\": \"add\", \"path\": \"/spec/template/spec/containers/0/command/-\", \"value\": \"--v=" + kindClusterLogLevel + "\" }]'",
	}
	cmdString = ""
	for _, s := range cmdStringSet {
		cmdString += s
	}
	cmd = exec.Command("bash", "-c", cmdString)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Cannot patch kube-proxy.\n", err)
	}
	fmt.Println("Kube-proxy patched! Guys how can I test this? To find out if it was really successful.")

	// Generate the file kubeconfig.conf on the artifacts directory
	cmdString = kind + " get kubeconfig --internal --name " + clusterName +" > " + artifactsDirectory + "/kubeconfig.conf"
	cmd = exec.Command("bash", "-c", cmdString)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to create the file kubeconfig.conf.\n", err)
	}

	// Generate the file KubeconfigTests on the artifacts directory
	cmdString = kind + " get kubeconfig --name " + clusterName +" > " + artifactsDirectory + "/" + KubeconfigTests
	cmd = exec.Command("bash", "-c", cmdString)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to create the file KubeconfigTests.\n", err)
	}

	fmt.Printf("Cluster %s is created.\n", clusterName)

}

// waitUntilClusterIsReady wait pods with selector k8s-app=kube-dns be ready and operational.
func waitUntilClusterIsReady(clusterName, binDir string, ciMode bool) {

	k8sContext := "kind-" + clusterName
	kubectl := binDir + "/kubectl"

	_, err := os.Stat(binDir)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(kubectl)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	cmdString := "kubectl --context " + k8sContext + " wait --for=condition=ready pods --namespace=" + Namespace + " --selector k8s-app=kube-dns 1> /dev/null"
	cmd := exec.Command("bash", "-c", cmdString)
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	
	if ciMode == true {
		cmd = exec.Command("kubectl", "--context", k8sContext, "get", "nodes", "-o", "wide")
		err = cmd.Run()
		if err != nil {
			log.Fatal("Unable to show nodes.", err)
		}

		cmd = exec.Command("kubectl", "--context", k8sContext, "get", "pods", "--all-namespaces")
		err = cmd.Run()
		if err != nil {
			log.Fatal("Error getting pods from all namespaces.", err)
		}
	}
	fmt.Printf("%s is operational.\n",clusterName)
}

// installKpng install KPNG following these steps:
//   - removes existing kube-proxy
//   - load kpng container image
//   - create service account, clusterbinding and configmap for kpng
//   - deploy kpng from the template generated
func installKpng(clusterName, binDir string) {

	k8sContext := "kind-" + clusterName
	kubectl 	:= binDir + "/kubectl"
	kind		:= binDir + "/kind"
	
	artifactsDirectory := os.Getenv("E2E_ARTIFACTS")
	e2eBackend := os.Getenv("E2E_BACKEND")
	e2eDeploymentModel := os.Getenv("E2E_DEPLOYMENT_MODEL")

	_, err := os.Stat(binDir)
	if err !=nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(kubectl)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	// Remove existing kube-proxy
	cmd := exec.Command(kubectl, "--context", k8sContext, "delete", "--namespace", Namespace, "daemonset.apps/kube-proxy")
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Cannot delete delete daemonset.apps kube-proxy\n", err)
	}
	fmt.Println("Removed daemonset.apps/kube-proxy.")

	// Load kpng container image
	// Preload kpng image 
	cmd = exec.Command(kind, "load", "docker-image", KpngImageTagName, "--name", clusterName)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error loading image to kind.\n", err)
	}
	fmt.Println("Loaded " + KpngImageTagName + " container image.")

	// Create service account, clusterbinding and configmap for kpng
	cmd = exec.Command(kubectl, "--context", k8sContext, "create", "serviceaccount", "--namespace", Namespace, ServiceAccountName)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating serviceaccount %s.\n", ServiceAccountName, err)
	}
	fmt.Println("Created service account", ServiceAccountName)

	cmd = exec.Command(kubectl, "--context", k8sContext, "create", "clusterrolebinding", ClusterRoleBindingName, "--clusterrole", ClusterRoleName, "--serviceaccount", Namespace + ":"+ ServiceAccountName)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating clusterrolebinding %s.\n", ClusterRoleBindingName, err)
	}
	fmt.Println("Created clusterrolebinding", ClusterRoleBindingName)

	cmd = exec.Command(kubectl, "--context", k8sContext, "create", "configmap", ConfigMapName, "--namespace", Namespace, "--from-file", artifactsDirectory + "/kubeconfig.conf")
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating configmap", ConfigMapName)
	}
	fmt.Println("Created configmap", ConfigMapName)

	// Deploy kpng from the template generated
	e2eServerArgs := "'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', 'to-api', '--listen=unix:///k8s/proxy.sock'"
    e2eBackendArgs := "'local', '--api=" + KpngServerAddress + "', 'to-" + e2eBackend + "', '--v=" + KpngDebugLevel + "'"

	if e2eDeploymentModel == "single-process-per-node" {
		e2eBackendArgs="'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', 'to-local', 'to-" + e2eBackend + "', '--v=" + KpngDebugLevel + "'"
	} 

	e2eServerArgs = "[" + e2eServerArgs + "]"
	e2eBackendArgs = "[" + e2eBackendArgs + "]"

	// Setting vars for generate the kpng deployment based on template
	_ = os.Setenv("kpng_image", KpngImageTagName) 
    _ = os.Setenv("image_pull_policy", "IfNotPresent") 
    _ = os.Setenv("backend", e2eBackend) 
    _ = os.Setenv("config_map_name", ConfigMapName) 
    _ = os.Setenv("service_account_name", ServiceAccountName) 
    _ = os.Setenv("namespace", Namespace) 
    _ = os.Setenv("e2e_backend_args", e2eBackendArgs)
    _ = os.Setenv("deployment_model", e2eDeploymentModel)
    _ = os.Setenv("e2e_server_args", e2eServerArgs)

	// TODO: Change kpngPath to scriptDir
	//go run "${scriptDir}"/kpng-ds-yaml-gen.go "${scriptDir}"/kpng-deployment-ds-template.txt  "${artifactsDirectory}"/kpng-deployment-ds.yaml	
	//ifErrorExit "error generating kpng deployment YAML"
	scriptDir := kpngPath + "/hack"

	cmd = exec.Command("go", "run", scriptDir + "/kpng-ds-yaml-gen.go", scriptDir + "/kpng-deployment-ds-template.txt", artifactsDirectory + "/kpng-deployment-ds.yaml")
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error generating kpng deployment YAML.", err)
	}

	cmd = exec.Command(kubectl, "--context", k8sContext, "create", "-f", artifactsDirectory + "/kpng-deployment-ds.yaml")
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating kpng deployment \n", err)
	}

	cmd = exec.Command(kubectl, "--context", k8sContext, "--namespace", Namespace, "rollout", "status", "daemonset", "kpng", "-w", "--request-timeout", "3m")	
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Timeout waiting kpng rollout\n", err)
	}

	fmt.Println("Installation of kpng is done.")

}

// runTests execute the tests with ginkgo.
func runTests(e2eDir, binDir string, parallelGinkgoTests bool, ipFamily, backend string, includeSpecificFailedTests bool) {

	artifactsDirectory := e2eDir + "/artifacts"
	ginkgo := binDir + "/ginkgo"
	e2eTest := binDir + "/e2e.test"

	_, err := os.Stat(artifactsDirectory + "/" + KubeconfigTests)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}
	_, err = os.Stat(binDir)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}
	_, err = os.Stat(binDir + "/e2e.test")
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}
	_, err = os.Stat(binDir + "/ginkgo")
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	// Ginkgo regexes
	ginkgoSkip := GinkgoSkipTests
	ginkgoFocus := ""
	if strings.TrimSpace(GinkgoFocus) == "" {
		ginkgoFocus = "\\[Conformance\\]"
	} else {
		ginkgoFocus = GinkgoFocus
	}
	
	var skipSetName, skipSetRef string

	if includeSpecificFailedTests == false {
	// Find ip_type and backend specific skip sets	
		skipSetName = "GINKGO_SKIP_" + ipFamily + "_" + backend + "_TEST"
		skipSetRef = skipSetName
		if len(strings.TrimSpace(skipSetRef)) > 0 {
			ginkgoSkip = ginkgoSkip + "|" + skipSetRef
		}	
	}

	// setting this env prevents ginkgo e2e from trying to run provider setup
	_ = os.Setenv("KUBERNETES_CONFORMANCE_TEST", "'y'")
	// setting these is required to make RuntimeClass tests work ... :/
	_ = os.Setenv("KUBE_CONTAINER_RUNTIME", "remote")
	_ = os.Setenv("KUBE_CONTAINER_RUNTIME_ENDPOINT", "unix:///run/containerd/containerd.sock")
	_ = os.Setenv("KUBE_CONTAINER_RUNTIME_NAME", "containerd")

	cmd := exec.Command(ginkgo, "--nodes", strconv.Itoa(GinkgoNumberOfNodes), "--focus", ginkgoFocus, "--skip", ginkgoSkip, e2eTest, 
		"--kubeconfig", artifactsDirectory + "/" + KubeconfigTests, "--provider", GinkgoProvider, "--dump-logs-on-failure", strconv.FormatBool(GinkgoDumpLogsOnFailure), 
	 	"--report-dir", GinkgoReportDir, "--disable-log-dump", strconv.FormatBool(GinkgoDisableLogDump))
	err = cmd.Run()
	
	if err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("Ginkgo Tests were executed")
	}

}

// createInfrastructureAndRunTests create_infrastructure_and_run_tests.
func createInfrastructureAndRunTests(e2eDir, ipFamily, backend, binDir, suffix string, developerMode, ciMode bool, deploymentModel string, exportMetrics, includeSpecificFailedTests bool) {

	artifactsDirectory := e2eDir + "/artifacts"
	clusterName := "kpng-e2e-" + ipFamily + "-" + backend + suffix

	_ = os.Setenv("E2E_DIR", e2eDir)
    _ = os.Setenv("E2E_ARTIFACTS", artifactsDirectory)
    _ = os.Setenv("E2E_CLUSTER_NAME", clusterName)
    _ = os.Setenv("E2E_IP_FAMILY", ipFamily)
    _ = os.Setenv("E2E_BACKEND", backend)
    _ = os.Setenv("E2E_DEPLOYMENT_MODEL", deploymentModel)
    _ = os.Setenv("E2E_EXPORT_METRICS", strconv.FormatBool(exportMetrics))
	
	_, err := os.Stat(artifactsDirectory)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(binDir)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	fmt.Println("Cluster name: ", clusterName)

	createCluster(clusterName, binDir, ipFamily, artifactsDirectory, ciMode)
	waitUntilClusterIsReady(clusterName, binDir, ciMode)

	err = os.WriteFile(e2eDir + "/clustername", []byte(clusterName), 0664)
	if err != nil {
		log.Fatal(err)
	}

	if backend != "not-kpng" {
		installKpng(clusterName, binDir)
	}
/*
	if developerMode == true {
		runTests(e2eDir, binDir, false, ipFamily, backend, includeSpecificFailedTests)
	}
*/
}

func main() {
	fmt.Println("Hello :)")

	var ciMode bool = true
	var dockerfile string
	var e2eDir string = ""
	var binDir string = ""	
	var (
		ipFamily		 				= "ipv4"
		backend							= "iptables"
		suffix							= ""
		developerMode					= true
		deploymentModel				= "single-process-per-node"
		exportMetrics					= false 
		includeSpecificFailedTests	= true
		cluster_count					= 1
	)
	

	wd, err := os.Getwd()
	ifErrorExit(err)
	hackDir := wd // TODO: How can I have this variable, hackDir, as a constant??? 
	kpngPath = path.Dir(wd)


	if e2eDir == "" {
		pwd, err := os.Getwd()
		ifErrorExit(err)
		e2eDir = pwd + "/temp_go/e2e"
	} 
	if binDir == "" {
		binDir = e2eDir + "/bin"
	}

	// Get the OS
	var buffer bytes.Buffer 
	cmdString := "uname | tr '[:upper:]' '[:lower:]'"
	cmd := exec.Command("bash", "-c", cmdString) // I need to better understand the -c option! And also try to implement it using Cmd.StdinPipe
	cmd.Stdout = &buffer
	err = cmd.Run()
	ifErrorExit(err)
	OS := strings.TrimSpace(buffer.String())


	// Get the path to the Dockerfile
	cmd = exec.Command("dirname", hackDir)
	buffer.Reset()
	cmd.Stdout = &buffer
	err = cmd.Run()
	ifErrorExit(err)
	dockerfile = strings.TrimSpace(buffer.String()) + "/Dockerfile"

	installBinaries(binDir, KubernetesVersion, OS, hackDir)
	prepareContainer(dockerfile, ciMode)

	tmpSuffix := ""
	if cluster_count == 1 {
		if len(suffix) > 0 {
			tmpSuffix = "-" + suffix
		}
		setE2eDir(e2eDir + tmpSuffix)
	}

	createInfrastructureAndRunTests(e2eDir, ipFamily, backend, binDir, suffix, developerMode, ciMode, deploymentModel, exportMetrics, includeSpecificFailedTests)
}