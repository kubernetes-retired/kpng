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
	KIND_VERSION="v0.17.0"
	K8S_VERSION="v1.25.3"
	KPNG_IMAGE_TAG_NAME="kpng:test_270323_0904" 
	CLUSTER_CIDR_V4="10.1.0.0/16"
	SERVICE_CLUSTER_IP_RANGE_V4="10.2.0.0/16"
	KINDEST_NODE_IMAGE="docker.io/kindest/node"
	E2E_K8S_VERSION="v1.25.3"
	KUBECONFIG_TESTS="kubeconfig_tests.conf"
)

var kpng_dir, CONTAINER_ENGINE string

func if_error_exit(error_msg error) {
    // Description:
    // Validate if previous command failed and show an error msg (if provided) 
    //
	// Arguments:
	// $1 - error message if not provided, it will just exit
	///////////////////////////////////////////////////////////////////////////
	if error_msg != nil {
		log.Fatal(error_msg)
	}
} 

func command_exist(cmd_test string) bool {
	///////////////////////////////////////////////////////////////////////////
	// Description:                                                            
    // Checkt if a binary exists                                               
    //                                                                         
    // Arguments:                                                              
    //   arg1: binary name                                                     
	///////////////////////////////////////////////////////////////////////////
	
	cmd_script := "command -v " + cmd_test + " > /dev/null 2>&1"
	cmd := exec.Command("bash", "-c", cmd_script)
	err := cmd.Run()
	
	if err == nil {
		return true
	}
	return false
}

func add_to_path(directory string) {
    // Description:                                                            //
    // Add directory to path                                                   //
    //                                                                         //
    // Arguments:                                                              //
    //   arg1:  directory                                                      //	
	///////////////////////////////////////////////////////////////////////
	_, err := os.Stat(directory)
	if err != nil && os.IsNotExist(err) {
		if_error_exit(err)
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
		if_error_exit(err)
		fmt.Println(os.Getenv("PATH"))
		fmt.Println("The directory is NOW in the $PATH env variable! hooray:)")
	}

}

func set_sysctl(attribute string, value int) {
    // Description:
    // Set a sysctl attribute to value
    //                                         
    // Arguments:
    //   arg1: attribute
    //   arg2: value
	///////////////////////////////////////////////////////////////
	var buffer bytes.Buffer
	cmd := exec.Command("sysct", "-n", attribute )
	cmd.Stdout = &buffer
	//result, err := cmd.Output() //cmd.Output() returns bytes. How to convert bytes to int?
	err := cmd.Run()
	if_error_exit(err)

	result_str := strings.TrimSpace(buffer.String())
	result_int, err := strconv.Atoi(result_str)
	
	fmt.Printf("Checking sysctls value: %d vs result: %d\n", value, result_int)
	if value != result_int {
		fmt.Printf("Setting: \"sysctl -w %s=%d\"\n", attribute, value)
		variable_value := attribute + "=" + strconv.Itoa(value)
		cmd = exec.Command("sudo", "sysctl", "-w", variable_value)
		err = cmd.Run()
		if_error_exit(err)
	}
}

func set_host_network_settings(ip_family string) {
	// Description: 
	// Prepare hosts network settings 
	//
	// Arguments: 
	//	arg1: ip_family
	///////////////////////////////////////////////////////////////////////
	set_sysctl("net.ipv4.ip_forward", 1)
	if ip_family == "ipv6" {
		//TODO 
		fmt.Println("TODO :-)")
	}	
}

func verify_sysctl_setting(attribute string, value int) {
	///////////////////////////////////////////////////////////////////////////
	// Description:                                                            
    // Verify that a sysctl attribute setting has a value                      
    //                                                                         
    // Arguments:                                                              
    //   arg1: attribute                                                       
    //   arg2: value                                                           
	///////////////////////////////////////////////////////////////////////////

	// Get the current value of the attribute and store it in the result variable
	var buffer bytes.Buffer
	cmd := exec.Command("sysctl", "-n", attribute)
	cmd.Stdout = &buffer
	err := cmd.Run()
	if_error_exit(err)
	result, err := strconv.Atoi(strings.TrimSpace(buffer.String()))

	if value != result {
		fmt.Printf("Failure: \"sysctl -n %s\" returned \"%d\", not \"%d\" as expected.\n", attribute, result, value)	
		os.Exit(1)
	}
}

func verify_host_network_settings(ip_family string) {
	///////////////////////////////////////////////////////////////////////////
	// Description:                                                            
	// Verify hosts network settings                                           
	//                                                                         
	// Arguments:                                                              
	//   arg1: ip_family                                                       
	///////////////////////////////////////////////////////////////////////////

	verify_sysctl_setting("net.ipv4.ip_forward", 1)
} 

func setup_kind(install_directory, operating_system string) {
	///////////////////////////////////////////////////////////////////////////
	// Description:                                                            
    // setup kind if not available in the system                               
    //                                                                         
    // Arguments:                                                              
    //   arg1: installation directory, path to where kind will be installed     
	///////////////////////////////////////////////////////////////////////////
	_, err := os.Stat(install_directory)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(install_directory + "/kind")
	if err == nil {
		fmt.Println("The kind tool is already set in your system.")
	} else if err != nil && os.IsNotExist(err) {
		fmt.Println()
		fmt.Println("Downloading kind ...")

		tmp_file, err := os.CreateTemp("/tmp", "kind_setup")
		if_error_exit(err)
		defer os.Remove(tmp_file.Name()) // Clean up. QUESTION: As we will end up moving the temp file, is this necessary? 
	
		url := "https://kind.sigs.k8s.io/dl/" + KIND_VERSION + "/kind-" + operating_system + "-amd64"
		fmt.Printf("Temp filename: %s\n", tmp_file.Name())
		cmd := exec.Command("curl", "-L", url, "-o", tmp_file.Name())	
		err = cmd.Run()
		if_error_exit(err)
		// TODO: Need to find out how to display, the curl ongoing download details, on the terminal
	
		cmd = exec.Command("sudo", "mv", tmp_file.Name(), install_directory + "/kind")
		_ = cmd.Run()
		//if_error_exit(err) 
		cmd = exec.Command("sudo", "chmod", "+rx", install_directory + "/kind")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chown", "root.root", install_directory + "/kind")
		
		fmt.Println("The kind tool is set.")		
	}
}

func setup_kubectl(install_directory, k8s_version, operating_system string) {
    // Description:                                                            
    // setup kubectl if not available in the system                            
    //                                                                         
    // Arguments:                                                              
    //   arg1: installation directory, path to where kubectl will be installed 
    //   arg2: Kubernetes version                                              
    //   arg3: OS, name of the operating system                                
	///////////////////////////////////////////////////////////////////////////

	// Check if the installation directory exist
	_, err := os.Stat(install_directory)
	if_error_exit(err)
	// If kubectl is not installed, install it.
	_, err = os.Stat(install_directory + "/kubectl")
	if err == nil {
		fmt.Println("Kubectl is already installed in the System.")
	} else if err != nil && os.IsNotExist(err) {
		// Show message "Downloading kubectl ..."
		fmt.Println("Downloading kubectl ...") 
		// Create tem file
		tmp_file, err := os.CreateTemp(".", "kubectl_setup")
		if_error_exit(err)
		// Download kubectl
		url := "https://dl.k8s.io/" + k8s_version + "/bin/" + operating_system + "/amd64/kubectl"
		cmd := exec.Command("curl", "-L", url, "-o", tmp_file.Name())
		err = cmd.Run()
		if_error_exit(err)
		//mv, chmod, chown 
		cmd = exec.Command("sudo", "mv", tmp_file.Name(), install_directory + "/kubectl")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chmod", "+rx", install_directory + "/kubectl")
		_ = cmd.Run()
		cmd = exec.Command("sudo", "chown", "root.root", install_directory + "/kubectl")
		_ = cmd.Run()

		fmt.Println("The Kubectl tool is set.")
	}

	
}

func setup_ginkgo(install_directory, k8s_version, operating_system string) {
    ///////////////////////////////////////////////////////////////////////////
	// Description:
    // setup ginkgo and e2e.test
    //
    // # Arguments:
    //   arg1: binary directory, path to where ginko will be installed
    //   arg2: Kubernetes version
    //  arg3: OS, name of the operating system
	/////////////////////////////////////////////////////////////////////////// 

	//Create temp directory
	tmp_dir, err := os.MkdirTemp(".", "ginkgo_setup_")  //I think this should only happen in case ginkgo and e2e.test are not installed. Fix later
	if_error_exit(err)
	defer os.RemoveAll(tmp_dir) //Clean up

	_, ginkgo_exist := os.Stat(install_directory + "/ginkgo")
	_, e2e_test_exist := os.Stat(install_directory + "/e2e.test")

	if os.IsNotExist(ginkgo_exist) || os.IsNotExist(e2e_test_exist) {
		fmt.Println("Downloading ginkgo and e2e.test ...")
		url := "https://dl.k8s.io/" + k8s_version + "/kubernetes-test-" + operating_system + "-amd64.tar.gz"
		out_file := tmp_dir + "/kubernetes-test-" + operating_system + "-amd64.tar.gz"

		cmd := exec.Command("curl", "-L", url, "-o", out_file)
		err = cmd.Run()
		if_error_exit(err)

		tar_file := tmp_dir + "/kubernetes-test-" + operating_system + "-amd64.tar.gz" 
		cmd_string := "tar xvzf " + tar_file + " --directory " + install_directory + " --strip-components=3 kubernetes/test/bin/ginkgo kubernetes/test/bin/e2e.test &> /dev/null"
		cmd = exec.Command("bash", "-c", cmd_string)
		err = cmd.Run()
		if_error_exit(err)

		fmt.Println("The tools ginko and e2e.test have been set up.")

	} else {
		fmt.Println("The tools ginko and e2e.test have already been set up.")
	}


}

func setup_bpf2go(install_directory string) {
	///////////////////////////////////////////////////////////////////////////
    // Description:                                                            
    // Install bpf2go binary                                                   
    //                                                                         
    // Arguments:                                                              
    //   arg1: installation directory, path to where bpf2go will be installed  
	///////////////////////////////////////////////////////////////////////////

	if ! command_exist("bpf2go") {
		if ! command_exist("go") {
			fmt.Println("Dependency not met: 'bpf2go' not installed and cannot install with 'go'")
			os.Exit(1)
		} 
		
		fmt.Println("'bpf2go' not found, installing with 'go'")
		_, err := os.Stat(install_directory)
		if err != nil && os.IsNotExist(err) {
			log.Fatal(err)
		}
		// set GOBIN to bin_directory to ensure that binary is in search path	
		_ = os.Setenv("GOBIN", install_directory)
		// remove GOPATH just to be sure
		_ = os.Setenv("GOPATH", "")

		cmd := exec.Command("go", "install", "github.com/cilium/ebpf/cmd/bpf2go@v0.9.2")
		err = cmd.Run()
		if_error_exit(err)
	
		var buffer bytes.Buffer
		cmd = exec.Command("which", "bpf2go")
		cmd.Stdout = &buffer
		err = cmd.Run()
		if_error_exit(err)
		fmt.Printf("The tool bpf2go is installed at: %s\n", buffer.String())	
	} 
} 



func install_binaries(bin_directory, k8s_version, operating_system, base_dir_path string, ) {
    
    // Description:
    // Copy binaries from the net to binaries directory
    //
    // Arguments:
    //   arg1: binary directory, path to where ginko will be installed
    //   arg2: Kubernetes version
    //  arg3: OS, name of the operating system
	/////////////////////////////////////////////////////////////////////////////////
	wd, err := os.Getwd()
	if_error_exit(err)
	
	if wd != base_dir_path {
		err = os.Chdir(base_dir_path)
		if_error_exit(err)
	}
	err = os.MkdirAll(bin_directory, 0755) 

	add_to_path(bin_directory) 

	setup_kind(bin_directory, operating_system)
	setup_kubectl(bin_directory, k8s_version, operating_system)
	setup_ginkgo(bin_directory, k8s_version, operating_system)
	setup_bpf2go(bin_directory)
}

func detect_container_engine() {
	///////////////////////////////////////////////////////////////////////////
    // Description:                                                           
    // Detect Container Engine, by default it is docker but developers might   
    // use real alternatives like podman. The project welcome both.            
    //                                                                         
    // Arguments:                                                              
    //   None
	///////////////////////////////////////////////////////////////////////////                                                                  	
	
	CONTAINER_ENGINE = "docker"
	cmd := exec.Command(CONTAINER_ENGINE) 
	err := cmd.Run()
	if err != nil {
		// If docker is not available, let's check if podman exists
		CONTAINER_ENGINE = "podman"
		cmd = exec.Command(CONTAINER_ENGINE, "--help")
		err = cmd.Run() 
		if err != nil {
			fmt.Println("The e2e tests currently support docker and podman as the container engine. Please install either of them")
			os.Exit(1)
		}
	}
	fmt.Println("Detected Container Engine:", CONTAINER_ENGINE)
}

func container_build(CONTAINER_FILE string, ci_mode bool) {
	///////////////////////////////////////////////////////////////////////////
    // Description:                                                            
    // build a container image for KPNG                                        
    //                                                                         
    // Arguments:                                                              
    //   arg1: Path for E2E installation directory, or the empty string         
    //   arg2: ci_mode        
	///////////////////////////////////////////////////////////////////////////
	
	// QUESTION to Jay: Is it necessary to have this variables in capital letters?
	
	// Running locally it's not necessary to show all info
	QUIET_MODE := "--quiet"
	if ci_mode == true {
		QUIET_MODE = ""
	}

	_, err := os.Stat(CONTAINER_FILE)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	err = os.Chdir(kpng_dir)
	if err != nil {
		log.Fatal(err)
	}

	if QUIET_MODE == "" {
		cmd := exec.Command(CONTAINER_ENGINE, "build", "-t", KPNG_IMAGE_TAG_NAME, "-f", CONTAINER_FILE, ".")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		cmd := exec.Command(CONTAINER_ENGINE, "build", QUIET_MODE, "-t", KPNG_IMAGE_TAG_NAME, "-f", CONTAINER_FILE, ".")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}

	err = os.Chdir(kpng_dir + "/hack")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Image build and tag %s is set.\n", KPNG_IMAGE_TAG_NAME)
}

func set_e2e_dir(e2e_dir string) {
	/////////////////////////////////////////////////////////////////////////////
    // Description:                                                            //
    // Set E2E directory                                                       //
    //                                                                         //
    // Arguments:                                                              //
    //   arg1: Path for E2E installation directory                             //
    /////////////////////////////////////////////////////////////////////////////
	
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	err = os.Chdir(kpng_dir + "/hack")
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(e2e_dir)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(e2e_dir + "/artifacts") 
	if err == nil {
		fmt.Println("The directory \"artifacts\" already exist!")
	} else if err != nil && os.IsNotExist(err) {
		err = os.Mkdir(e2e_dir + "/artifacts", 0755)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = os.Chdir(wd)
	if err != nil {
		log.Fatal(err)
	}
}

func prepare_container(dockerfile string, ci_mode bool) {
	///////////////////////////////////////////////////////////////////////////
	// Description:
    // Prepare container  
    //                                                      
    // Arguments:
    //   arg1: Path of dockerfile
    //   arg2: ci_mode
	///////////////////////////////////////////////////////////////////////////

	detect_container_engine()
	container_build(dockerfile, ci_mode) 

}

func create_cluster(cluster_name, bin_dir, ip_family, artifacts_directory string, ci_mode bool) {
    /////////////////////////////////////////////////////////////////////////////
    // Description:                                                            //
    // Create kind cluster                                                     //
    //                                                                         //
    /////////////////////////////////////////////////////////////////////////////
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
		kind = bin_dir + "/kind"
		kubectl = bin_dir + "/kubectl"
	)


	_, err := os.Stat(bin_dir)
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

	cmd_string := kind + " get clusters | grep -q " + cluster_name
	cmd := exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err == nil {
		cmd_string = kind + " delete cluster --name " + cluster_name
		cmd = exec.Command("bash", "-c", cmd_string)
		err = cmd.Run()
		if err != nil {
			log.Fatalf("Cannot delete cluster %s\n", cluster_name, err)
		}
		fmt.Printf("Previous cluster %s deleted.\n", cluster_name)
	}

	// Default Log level for all components in test clusters
	kind_cluster_log_level := os.Getenv("KIND_CLUSTER_LOG_LEVEL")
	if strings.TrimSpace(kind_cluster_log_level) == "" {
		kind_cluster_log_level = "4"
	}
	kind_log_level := "-v3"
	if ci_mode == true {
		kind_log_level = "-v7"
	}
/*
	// Potentially enable --logging-format
	scheduler_extra_args := "\"v\": \"" + kind_cluster_log_level + "\""
	controllerManager_extra_args := "\"v\": \"" + kind_cluster_log_level + "\""
	apiServer_extra_args := "\"v\": \"" + kind_cluster_log_level + "\""
*/
	var CLUSTER_CIDR, SERVICE_CLUSTER_IP_RANGE string
	
	switch ip_family {
	case "ipv4":
		CLUSTER_CIDR = CLUSTER_CIDR_V4
        SERVICE_CLUSTER_IP_RANGE = SERVICE_CLUSTER_IP_RANGE_V4
	}

	fmt.Printf("Preparing to setup %s cluster ...\n", cluster_name)
	// Create cluster
	// Create the config file
	tmpl, err := template.New("kind_config_template").Parse(KIND_CONFIG_TEMPLATE)	
	if err != nil {
		log.Fatalf("Unable to create template %v", err)
	}
	kind_config_template_data := KindConfigData {
		IpFamily:			 	ip_family,
		ClusterCidr:			CLUSTER_CIDR,
		ServiceClusterIpRange:	SERVICE_CLUSTER_IP_RANGE,
	}

	yamlDestPath := artifacts_directory + "/kind-config.yaml"
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
		"--name " + cluster_name,
		" --image " + KINDEST_NODE_IMAGE + ":" + E2E_K8S_VERSION,
		" --retain",
		" --wait=1m ",
		kind_log_level,
		" --config=" + artifacts_directory + "/kind-config.yaml",
	}
	cmd_string = ""
	for _, s := range cmd_string_set {
		cmd_string += s
	}
	
	cmd = exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Can not create kind cluster %s\n", cluster_name, err)
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
	cmd_string = kind + " get kubeconfig --internal --name " + cluster_name +" > " + artifacts_directory + "/kubeconfig.conf"
	cmd = exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to create the file kubeconfig.conf.\n", err)
	}

	// Generate the file KUBECONFIG_TESTS on the artifacts directory
	cmd_string = kind + " get kubeconfig --name " + cluster_name +" > " + artifacts_directory + "/" + KUBECONFIG_TESTS
	cmd = exec.Command("bash", "-c", cmd_string)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to create the file KUBECONFIG_TESTS.\n", err)
	}

	fmt.Printf("Cluster %s is created.\n", cluster_name)

}

func create_imfrastructure_and_run_tests(e2e_dir, ip_family, backend, bin_dir, suffix string, developer_mode, ci_mode bool, deployment_model string, export_metrics, include_specific_failed_tests bool) {
    ///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
    // Description:                                                            
    // create_infrastructure_and_run_tests                                     
    //                                                                         
	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	artifacts_directory := e2e_dir + "/artifacts"
	cluster_name := "kpng-e2e-" + ip_family + "-" + backend + suffix

	_ = os.Setenv("E2E_DIR", e2e_dir)
    _ = os.Setenv("E2E_ARTIFACTS", artifacts_directory)
    _ = os.Setenv("E2E_CLUSTER_NAME", cluster_name)
    _ = os.Setenv("E2E_IP_FAMILY", ip_family)
    _ = os.Setenv("E2E_BACKEND", backend)
    _ = os.Setenv("E2E_DEPLOYMENT_MODEL", deployment_model)
    _ = os.Setenv("E2E_EXPORT_METRICS", strconv.FormatBool(export_metrics))
	
	_, err := os.Stat(artifacts_directory)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(bin_dir)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}

	fmt.Println("Cluster name: ", cluster_name)

	create_cluster(cluster_name, bin_dir, ip_family, artifacts_directory, ci_mode)



}

func main() {
	fmt.Println("Hello :)")

	var ci_mode bool = true
	var dockerfile string
	var e2e_dir string = ""
	var bin_dir string = ""	
	var (
		ip_family		 				= "ipv4"
		backend							= "iptables"
		suffix							= ""
		developer_mode					= true
		deployment_model				= "single-process-per-node"
		export_metrics					= false 
		include_specific_failed_tests	= true
		cluster_count					= 1
	)
	

	wd, err := os.Getwd()
	if_error_exit(err)
	base_dir := wd //How can I have this variable as a constant??? 
	kpng_dir = path.Dir(wd)


	if e2e_dir == "" {
		pwd, err := os.Getwd()
		if_error_exit(err)
		e2e_dir = pwd + "/temp_go/e2e"
	} 
	if bin_dir == "" {
		bin_dir = e2e_dir + "/bin"
	}

	// Get the OS
	var buffer bytes.Buffer 
	cmd_string := "uname | tr '[:upper:]' '[:lower:]'"
	cmd := exec.Command("bash", "-c", cmd_string) // I need to better understand the -c option! And also try to implement it using Cmd.StdinPipe
	cmd.Stdout = &buffer
	err = cmd.Run()
	if_error_exit(err)
	OS := strings.TrimSpace(buffer.String())


	// Get the path to the Dockerfile
	cmd = exec.Command("dirname", base_dir)
	buffer.Reset()
	cmd.Stdout = &buffer
	err = cmd.Run()
	if_error_exit(err)
	dockerfile = strings.TrimSpace(buffer.String()) + "/Dockerfile"

	install_binaries(bin_dir, K8S_VERSION, OS, base_dir)
	prepare_container(dockerfile, ci_mode)

	tmp_suffix := ""
	if cluster_count == 1 {
		if len(suffix) > 0 {
			tmp_suffix = "-" + suffix
		}
		set_e2e_dir(e2e_dir + tmp_suffix)
	}

	create_imfrastructure_and_run_tests(e2e_dir, ip_family, backend, bin_dir, suffix, developer_mode, ci_mode, deployment_model, export_metrics, include_specific_failed_tests)
}