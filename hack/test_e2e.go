package main

import (
	"fmt"
	"os"
	"os/exec"
	"log"
	"bytes"
	"strconv"
	"strings"
) 

const (
	KIND_VERSION="v0.17.0"
	K8S_VERSION="v1.25.3"

)

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
    // Description:                                                            #
    // Add directory to path                                                   #
    //                                                                         #
    // Arguments:                                                              #
    //   arg1:  directory                                                      #	
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





func main() {
	fmt.Println("Hello :)")
	var e2e_dir string = ""
	var bin_dir string = ""	

	wd, err := os.Getwd()
	if_error_exit(err)
	base_dir := wd //How can I have this variable as a constant??? 


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

	install_binaries(bin_dir, K8S_VERSION, OS, base_dir)

}