package app

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
		// Username is the username for all the VMs created in Azure
		Username string `yaml:"username"`
		// Password is the password for all the VMs created in Azure
		Password string `yaml:"password"`
		// PublicKeys is a list of public keys that will be added to the VMs
		PublicKeys []string `yaml:"publicKeys"`
	} `yaml:"azure"`

	KubeVirt struct {
		Version string `yaml:"version"`
		Workers int    `yaml:"workers"`
		CDI     struct {
			Version string `yaml:"version"`
		} `yaml:"cdi"`
	} `yaml:"kubevirt"`

	OpenNebula struct {
		Workers int `yaml:"workers"`
		Image   struct {
			Name string `yaml:"name"`
			URL  string `yaml:"url"`
		} `yaml:"image"`
		Template struct {
			Name string `yaml:"name"`
		} `yaml:"template"`
	} `yaml:"opennebula"`
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
