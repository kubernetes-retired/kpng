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


}

func main() {
	fmt.Println("Verify Host Networking Settings")
	set_sysctl("net.ipv4.ip_forward", 1)

}