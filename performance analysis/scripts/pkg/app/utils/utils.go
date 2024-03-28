package utils

import (
	"github.com/melbahja/goph"
	"log"
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
