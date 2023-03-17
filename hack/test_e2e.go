package main

import (
	"fmt"
	"os/exec"
	"log"
	"bytes"
	"strconv"
	"strings"
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

func add_to_path(directory string) {

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
	if ip_family = "ipv6" {
		//TODO 
		fmt.Println("TODO :-)")
	}	
}

func install_binaries(bin_directory string, k8s_version string, os string) {
    
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
	
	if wd != basename_dir {
		err = os.Chdir(basename_dir)
		if_error_exit
	}
	os.MkdirAll() ...

}





func main() {
	fmt.Println("Ola :-)")
	var e2e_dir string = ""
	var bin_dir string = ""	

	wd, err := os.Getwd
	if_error_exit(err)
	const basename_dir = wd


	if e2e_dir == "" {
		pwd, err := os.Getwd()
		if_error_exit(err)
		e2e_dir = pwd + "/temp_go/e2e"
	} 
	if bin_dir == "" {
		bin_dir = e2e_dir + "/bin"
	}






}