package utils

import (
	"encoding/json"
	"github.com/melbahja/goph"
	"log"
	"math/rand"
	"performance/pkg/app"
)

// SshCommand executes an SshCommand command on the given VM
func SshCommand(ip string, commands []string) ([]string, error) {
	client, err := goph.NewUnknown(app.Config.Azure.Username, ip, goph.Password(app.Config.Azure.Password))
	if err != nil {
		return nil, err
	}

	var outAll []string

	for _, command := range commands {
		out, err := client.Run(command)
		if err != nil {
			return nil, err
		}

		outAll = append(outAll, string(out))
	}

	defer func(client *goph.Client) {
		err := client.Close()
		if err != nil {
			log.Println(err)
		}
	}(client)

	return outAll, nil
}

// ParseSshOutput parses the string list's first string returned by SshCommand to a struct
func ParseSshOutput[T any](output []string) (*T, error) {
	if len(output) == 0 {
		return nil, nil
	}

	var parsedOutput T
	err := json.Unmarshal([]byte(output[0]), &parsedOutput)
	if err != nil {
		return nil, err
	}

	return &parsedOutput, nil
}

// RandomName generates a random name with the given prefix
func RandomName(prefix string) string {
	randomString := func(length int) string {
		letterBytes := "abcdefghijklmnopqrstuvwxyz"
		b := make([]byte, length)
		for i := range b {
			b[i] = letterBytes[rand.Intn(len(letterBytes))]
		}
		return string(b)
	}

	return prefix + "-" + randomString(5)
}
