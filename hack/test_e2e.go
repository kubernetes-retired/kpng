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
// Ginkgo
const (
	GinkgoSkipTests="machinery|Feature|Federation|PerformanceDNS|Disruptive|Serial|LoadBalancer|KubeProxy|GCE|Netpol|NetworkPolicy"
	GinkgoFocus="\\[Conformance\\]|\\[sig-network\\]"
	GinkgoNumberOfNodes=25
	GinkgoProvider="local"
	GinkgoDumpLogsOnFailure=false
	GinkgoReportDir="artifacts/reports"
	GinkgoDisableLogDump=true
)

var kpngDir, containerEngine string

// ifErrorExit validate if previous command failed and show an error msg (if provided).
//
// If an error message is not provided, it will just exit.
func ifErrorExit(errorMsg error) {
	if errorMsg != nil {
		log.Fatal(errorMsg)
	}
} 

// commandExist check if a binary(cmdTest) exists. It returns True or False 
func commandExist(cmdTest string) bool {
	cmd_script := "command -v " + cmdTest + " > /dev/null 2>&1"
	cmd := exec.Command("bash", "-c", cmd_script)
	err := cmd.Run()
	
	if err == nil {
		return true
	}
	return false
}

// addToPath add directory to path.
func addToPath(directory string) {
	_, err := os.Stat(directory)
	if err != nil && os.IsNotExist(err) {
		ifErrorExit(err)
	}

	fmt.Println("Adding the directory to $PATH env variable")
	fmt.Println()
	path := os.Getenv("PATH")
	fmt.Println("Current $PATH: ", path)

	// Check if the directory path is in the $PATH env variable 
	// I believe the package regexp can be used. Couldn't figure out the right pattern. For now 
	// will use the following approach
	// TODO(Maybe!): Check using the regexp package
	path_set := strings.Split(path, ":") 
	dir_path_exist := false
	for _, s := range path_set {
		if directory == s {
			dir_path_exist = true
			break
		}
	}

	fmt.Println("New directory: ", directory)
	if !dir_path_exist {
		fmt.Println("The directory is NOT in the $PATH env variable! :(")
		updated_path := path + ":" + directory
		err = os.Setenv("PATH", updated_path)
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

	result_str := strings.TrimSpace(buffer.String())
	result_int, err := strconv.Atoi(result_str)
	
	fmt.Printf("Checking sysctls value: %d vs result: %d\n", value, result_int)
	if value != result_int {
		fmt.Printf("Setting: \"sysctl -w %s=%d\"\n", attribute, value)
		variable_value := attribute + "=" + strconv.Itoa(value)
		cmd = exec.Command("sudo", "sysctl", "-w", variable_value)
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

		tmp_file, err := os.CreateTemp("/tmp", "kind_setup")
		ifErrorExit(err)
		defer os.Remove(tmp_file.Name()) // Clean up. QUESTION: As we will end up moving the temp file, is this necessary? 
	
		url := "https://kind.sigs.k8s.io/dl/" + KindVersion + "/kind-" + operatingSystem + "-amd64"
		fmt.Printf("Temp filename: %s\n", tmp_file.Name())
		cmd := exec.Command("curl", "-L", url, "-o", tmp_file.Name())	
		err = cmd.Run()
		ifErrorExit(err)
		// TODO: Need to find out how to display, the curl ongoing download details, on the terminal
	
		cmd = exec.Command("sudo", "mv", tmp_file.Name(), installDirectory + "/kind")
		_ = cmd.Run()
		//ifErrorExit(err) 
		cmd = exec.Command("sudo", "chmod", "+rx", installDirectory + "/kind")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chown", "root.root", installDirectory + "/kind")
		
		fmt.Println("The kind tool is set.")		
	}
}

// setupKubectl setup kubectl for k8sVersion, in the installDirectory if not available in the operatingSystem.
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
		tmp_file, err := os.CreateTemp(".", "kubectl_setup")
		ifErrorExit(err)
		// Download kubectl
		url := "https://dl.k8s.io/" + k8sVersion + "/bin/" + operatingSystem + "/amd64/kubectl"
		cmd := exec.Command("curl", "-L", url, "-o", tmp_file.Name())
		err = cmd.Run()
		ifErrorExit(err)
		//mv, chmod, chown 
		cmd = exec.Command("sudo", "mv", tmp_file.Name(), installDirectory + "/kubectl")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chmod", "+rx", installDirectory + "/kubectl")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chown", "root.root", installDirectory + "/kubectl")
		_ = cmd.Run()

		fmt.Println("The Kubectl tool is set.")
	}
}

// setupGinkgo setup ginkgo and e2e.test, for k8sVersion, in the installDirectory, if not available on the operatingSystem.
func setupGinkgo(installDirectory, k8sVersion, operatingSystem string) {
	//Create temp directory
	tmp_dir, err := os.MkdirTemp(".", "ginkgo_setup_")  //I think this should only happen in case ginkgo and e2e.test are not installed. Fix later
	ifErrorExit(err)
	defer os.RemoveAll(tmp_dir) //Clean up

	_, ginkgo_exist := os.Stat(installDirectory + "/ginkgo")
	_, e2e_test_exist := os.Stat(installDirectory + "/e2e.test")

	if os.IsNotExist(ginkgo_exist) || os.IsNotExist(e2e_test_exist) {
		fmt.Println("Downloading ginkgo and e2e.test ...")
		url := "https://dl.k8s.io/" + k8sVersion + "/kubernetes-test-" + operatingSystem + "-amd64.tar.gz"
		out_file := tmp_dir + "/kubernetes-test-" + operatingSystem + "-amd64.tar.gz"

		cmd := exec.Command("curl", "-L", url, "-o", out_file)
		err = cmd.Run()
		ifErrorExit(err)

		tar_file := tmp_dir + "/kubernetes-test-" + operatingSystem + "-amd64.tar.gz" 
		cmd_string := "tar xvzf " + tar_file + " --directory " + installDirectory + " --strip-components=3 kubernetes/test/bin/ginkgo kubernetes/test/bin/e2e.test &> /dev/null"
		cmd = exec.Command("bash", "-c", cmd_string)
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

// installBinaries copy binaries from the net to the binaries directory.
func installBinaries(binDirectory, k8sVersion, operatingSystem, baseDirPath string, ) {
	wd, err := os.Getwd()
	ifErrorExit(err)
	
	if wd != baseDirPath {
		err = os.Chdir(baseDirPath)
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
	QUIET_MODE := "--quiet"
	if ciMode == true {
		QUIET_MODE = ""
	}

	_, err := os.Stat(containerFile)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	err = os.Chdir(kpngDir)
	if err != nil {
		log.Fatal(err)
	}

	if QUIET_MODE == "" {
		cmd := exec.Command(containerEngine, "build", "-t", KpngImageTagName, "-f", containerFile, ".")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		cmd := exec.Command(containerEngine, "build", QUIET_MODE, "-t", KpngImageTagName, "-f", containerFile, ".")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}

	err = os.Chdir(kpngDir + "/hack")
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

	err = os.Chdir(kpngDir + "/hack")
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

	const KIND_CONFIG_TEMPLATE = `
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

	cmd_string := kind + " get clusters | grep -q " + clusterName
	cmd := exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err == nil {
		cmd_string = kind + " delete cluster --name " + clusterName
		cmd = exec.Command("bash", "-c", cmd_string)
		err = cmd.Run()
		if err != nil {
			log.Fatalf("Cannot delete cluster %s\n", clusterName, err)
		}
		fmt.Printf("Previous cluster %s deleted.\n", clusterName)
	}

	// Default Log level for all components in test clusters
	kind_cluster_log_level := os.Getenv("KIND_CLUSTER_LOG_LEVEL")
	if strings.TrimSpace(kind_cluster_log_level) == "" {
		kind_cluster_log_level = "4"
	}
	kind_log_level := "-v3"
	if ciMode == true {
		kind_log_level = "-v7"
	}
/*
	// Potentially enable --logging-format
	scheduler_extra_args := "\"v\": \"" + kind_cluster_log_level + "\""
	controllerManager_extra_args := "\"v\": \"" + kind_cluster_log_level + "\""
	apiServer_extra_args := "\"v\": \"" + kind_cluster_log_level + "\""
*/
	var CLUSTER_CIDR, SERVICE_CLUSTER_IP_RANGE string
	
	switch ipFamily {
	case "ipv4":
		CLUSTER_CIDR = ClusterCidrV4
        SERVICE_CLUSTER_IP_RANGE = ServiceClusterIpRangeV4
	}

	fmt.Printf("Preparing to setup %s cluster ...\n", clusterName)
	// Create cluster
	// Create the config file
	tmpl, err := template.New("kind_config_template").Parse(KIND_CONFIG_TEMPLATE)	
	if err != nil {
		log.Fatalf("Unable to create template %v", err)
	}
	kind_config_template_data := KindConfigData {
		IpFamily:			 	ipFamily,
		ClusterCidr:			CLUSTER_CIDR,
		ServiceClusterIpRange:	SERVICE_CLUSTER_IP_RANGE,
	}

	yamlDestPath := artifactsDirectory + "/kind-config.yaml"
	f, err := os.Create(yamlDestPath)
	if err != nil {
		log.Fatal(err)
	}
	err = tmpl.Execute(f, kind_config_template_data)
	if err != nil {
		log.Fatal(err)
	}
	
	cmd_string_set := []string {
		kind + " create cluster ",
		"--name " + clusterName,
		" --image " + KindestNodeImage + ":" + E2eK8sVersion,
		" --retain",
		" --wait=20m ",
		kind_log_level,
		" --config=" + artifactsDirectory + "/kind-config.yaml",
	}
	cmd_string = ""
	for _, s := range cmd_string_set {
		cmd_string += s
	}
	
	cmd = exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Can not create kind cluster %s\n", clusterName, err)
	} 

	// Patch kube-proxy to set the verbosity level
	cmd_string_set = []string {
		kubectl + " patch -n kube-system daemonset/kube-proxy ",
		"--type='json' ",
		"-p='[{\"op\": \"add\", \"path\": \"/spec/template/spec/containers/0/command/-\", \"value\": \"--v=" + kind_cluster_log_level + "\" }]'",
	}
	cmd_string = ""
	for _, s := range cmd_string_set {
		cmd_string += s
	}
	cmd = exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Cannot patch kube-proxy.\n", err)
	}
	fmt.Println("Kube-proxy patched! Guys how can I test this? To find out if it was really successful.")

	// Generate the file kubeconfig.conf on the artifacts directory
	cmd_string = kind + " get kubeconfig --internal --name " + clusterName +" > " + artifactsDirectory + "/kubeconfig.conf"
	cmd = exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to create the file kubeconfig.conf.\n", err)
	}

	// Generate the file KubeconfigTests on the artifacts directory
	cmd_string = kind + " get kubeconfig --name " + clusterName +" > " + artifactsDirectory + "/" + KubeconfigTests
	cmd = exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to create the file KubeconfigTests.\n", err)
	}

	fmt.Printf("Cluster %s is created.\n", clusterName)

}

// waitUntilClusterIsReady wait pods with selector k8s-app=kube-dns be ready and operational.
func waitUntilClusterIsReady(clusterName, binDir string, ciMode bool) {

	k8s_context := "kind-" + clusterName
	kubectl := binDir + "/kubectl"

	_, err := os.Stat(binDir)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(kubectl)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	cmd_string := "kubectl --context " + k8s_context + " wait --for=condition=ready pods --namespace=" + Namespace + " --selector k8s-app=kube-dns 1> /dev/null"
	cmd := exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	
	if ciMode == true {
		cmd = exec.Command("kubectl", "--context", k8s_context, "get", "nodes", "-o", "wide")
		err = cmd.Run()
		if err != nil {
			log.Fatal("Unable to show nodes.", err)
		}

		cmd = exec.Command("kubectl", "--context", k8s_context, "get", "pods", "--all-namespaces")
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

	k8s_context := "kind-" + clusterName
	kubectl 	:= binDir + "/kubectl"
	kind		:= binDir + "/kind"
	
	artifactsDirectory := os.Getenv("E2E_ARTIFACTS")
	E2E_BACKEND := os.Getenv("E2E_BACKEND")
	E2E_DEPLOYMENT_MODEL := os.Getenv("E2E_DEPLOYMENT_MODEL")

	_, err := os.Stat(binDir)
	if err !=nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(kubectl)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	// Remove existing kube-proxy
	cmd := exec.Command(kubectl, "--context", k8s_context, "delete", "--namespace", Namespace, "daemonset.apps/kube-proxy")
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
	cmd = exec.Command(kubectl, "--context", k8s_context, "create", "serviceaccount", "--namespace", Namespace, ServiceAccountName)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating serviceaccount %s.\n", ServiceAccountName, err)
	}
	fmt.Println("Created service account", ServiceAccountName)

	cmd = exec.Command(kubectl, "--context", k8s_context, "create", "clusterrolebinding", ClusterRoleBindingName, "--clusterrole", ClusterRoleName, "--serviceaccount", Namespace + ":"+ ServiceAccountName)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating clusterrolebinding %s.\n", ClusterRoleBindingName, err)
	}
	fmt.Println("Created clusterrolebinding", ClusterRoleBindingName)

	cmd = exec.Command(kubectl, "--context", k8s_context, "create", "configmap", ConfigMapName, "--namespace", Namespace, "--from-file", artifactsDirectory + "/kubeconfig.conf")
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating configmap", ConfigMapName)
	}
	fmt.Println("Created configmap", ConfigMapName)

	// Deploy kpng from the template generated
	E2E_SERVER_ARGS := "'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', 'to-api', '--listen=unix:///k8s/proxy.sock'"
    E2E_BACKEND_ARGS := "'local', '--api=" + KpngServerAddress + "', 'to-" + E2E_BACKEND + "', '--v=" + KpngDebugLevel + "'"

	if E2E_DEPLOYMENT_MODEL == "single-process-per-node" {
		E2E_BACKEND_ARGS="'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', 'to-local', 'to-" + E2E_BACKEND + "', '--v=" + KpngDebugLevel + "'"
	} 

	E2E_SERVER_ARGS = "[" + E2E_SERVER_ARGS + "]"
	E2E_BACKEND_ARGS = "[" + E2E_BACKEND_ARGS + "]"

	// Setting vars for generate the kpng deployment based on template
	_ = os.Setenv("kpng_image", KpngImageTagName) 
    _ = os.Setenv("image_pull_policy", "IfNotPresent") 
    _ = os.Setenv("backend", E2E_BACKEND) 
    _ = os.Setenv("config_map_name", ConfigMapName) 
    _ = os.Setenv("service_account_name", ServiceAccountName) 
    _ = os.Setenv("namespace", Namespace) 
    _ = os.Setenv("e2e_backend_args", E2E_BACKEND_ARGS)
    _ = os.Setenv("deployment_model", E2E_DEPLOYMENT_MODEL)
    _ = os.Setenv("e2e_server_args", E2E_SERVER_ARGS)

	// TODO: Change kpngDir to SCRIPT_DIR
	//go run "${SCRIPT_DIR}"/kpng-ds-yaml-gen.go "${SCRIPT_DIR}"/kpng-deployment-ds-template.txt  "${artifactsDirectory}"/kpng-deployment-ds.yaml	
	//ifErrorExit "error generating kpng deployment YAML"
	SCRIPT_DIR := kpngDir + "/hack"

	cmd = exec.Command("go", "run", SCRIPT_DIR + "/kpng-ds-yaml-gen.go", SCRIPT_DIR + "/kpng-deployment-ds-template.txt", artifactsDirectory + "/kpng-deployment-ds.yaml")
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error generating kpng deployment YAML.", err)
	}

	cmd = exec.Command(kubectl, "--context", k8s_context, "create", "-f", artifactsDirectory + "/kpng-deployment-ds.yaml")
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating kpng deployment \n", err)
	}

	cmd = exec.Command(kubectl, "--context", k8s_context, "--namespace", Namespace, "rollout", "status", "daemonset", "kpng", "-w", "--request-timeout", "3m")	
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
	e2e_test := binDir + "/e2e.test"

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
	ginkgo_skip := GinkgoSkipTests
	ginkgo_focus := ""
	if strings.TrimSpace(GinkgoFocus) == "" {
		ginkgo_focus = "\\[Conformance\\]"
	} else {
		ginkgo_focus = GinkgoFocus
	}
	
	var skip_set_name,  skip_set_ref string

	if includeSpecificFailedTests == false {
	// Find ip_type and backend specific skip sets	
		skip_set_name = "GINKGO_SKIP_" + ipFamily + "_" + backend + "_TEST"
		skip_set_ref = skip_set_name
		if len(strings.TrimSpace(skip_set_ref)) > 0 {
			ginkgo_skip = ginkgo_skip + "|" + skip_set_ref
		}	
	}

	// setting this env prevents ginkgo e2e from trying to run provider setup
	_ = os.Setenv("KUBERNETES_CONFORMANCE_TEST", "'y'")
	// setting these is required to make RuntimeClass tests work ... :/
	_ = os.Setenv("KUBE_CONTAINER_RUNTIME", "remote")
	_ = os.Setenv("KUBE_CONTAINER_RUNTIME_ENDPOINT", "unix:///run/containerd/containerd.sock")
	_ = os.Setenv("KUBE_CONTAINER_RUNTIME_NAME", "containerd")

	cmd := exec.Command(ginkgo, "--nodes", strconv.Itoa(GinkgoNumberOfNodes), "--focus", ginkgo_focus, "--skip", ginkgo_skip, e2e_test, 
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
	base_dir := wd //How can I have this variable as a constant??? 
	kpngDir = path.Dir(wd)


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
	cmd_string := "uname | tr '[:upper:]' '[:lower:]'"
	cmd := exec.Command("bash", "-c", cmd_string) // I need to better understand the -c option! And also try to implement it using Cmd.StdinPipe
	cmd.Stdout = &buffer
	err = cmd.Run()
	ifErrorExit(err)
	OS := strings.TrimSpace(buffer.String())


	// Get the path to the Dockerfile
	cmd = exec.Command("dirname", base_dir)
	buffer.Reset()
	cmd.Stdout = &buffer
	err = cmd.Run()
	ifErrorExit(err)
	dockerfile = strings.TrimSpace(buffer.String()) + "/Dockerfile"

	installBinaries(binDir, KubernetesVersion, OS, base_dir)
	prepareContainer(dockerfile, ciMode)

	tmp_suffix := ""
	if cluster_count == 1 {
		if len(suffix) > 0 {
			tmp_suffix = "-" + suffix
		}
		setE2eDir(e2eDir + tmp_suffix)
	}

	createInfrastructureAndRunTests(e2eDir, ipFamily, backend, binDir, suffix, developerMode, ciMode, deploymentModel, exportMetrics, includeSpecificFailedTests)
}