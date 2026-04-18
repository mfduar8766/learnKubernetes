package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
)

func main() {
	username := utils.GetEnv(types.MQTT_USER)
	password := utils.GetEnv(types.MQTT_PASSWORD)

	if username == "" || password == "" {
		panic("MQTT_USER or MQTT_PASSWORD environment variables are not set")
	}

	dirPath := "/mosquitto/auth"
	filename := dirPath + "/password_file"

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			fmt.Printf("Failed to create dir: %v\n", err)
			os.Exit(2)
		}
	}

	binary, err := exec.LookPath("mosquitto_passwd")
	if err != nil {
		fmt.Printf("Failed to find mosquitto binary path: %+v\n", err)
		// Fallback to the most common install location if LookPath fails
		binary = "/usr/bin/mosquitto_passwd"
	}

	cmd := exec.Command(binary, "-b", "-c", filename, username, password)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("mosquitto_passwd failed: %s\n", string(out))
		os.Exit(2)
	}

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString("\n")
		f.Close()
	}

	// FIX: Append a newline to the end of the file so Mosquitto parses it correctly
	err = os.Chown(filename, 1883, 1883)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully generated password file")
}
