package main

import (
	"fmt"
	"os"
)

func readConfig(filename string) ([]DomainConfig, error) {
	fs, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	var cfg []DomainConfig
	var status string
	var domain string
	for {
		if _, err := fmt.Fscanln(fs, &status, &domain); err != nil {
			break
		}
		cfg = append(cfg, DomainConfig{
			Status: status,
			Domain: domain,
		})
	}

	return cfg, nil
}

type DomainConfig struct {
	Status string
	Domain string
}
