package config

import (
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

type ConfigType struct {
	Azure struct {
		AuthLocation          string `yaml:"authLocation"`
		SubscriptionID        string `yaml:"subscriptionId"`
		ResourceGroupBaseName string `yaml:"resourceGroupBaseName"`
	} `yaml:"azure"`
}

var Config ConfigType

// LoadConfig loads the configuration from the given path
// If the path is empty, it will load the configuration from ./config.yml
func LoadConfig(path *string) {
	if path == nil {
		p := "./config.yml"
		path = &p
	}

	// Load YAML file from path
	yamlFile, err := os.ReadFile(*path)
	if err != nil {
		log.Fatalf(err.Error())
	}

	err = yaml.Unmarshal(yamlFile, &Config)
	if err != nil {
		log.Fatalf(err.Error())
	}
}
