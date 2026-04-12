package main

import (
	"bufio"
	"os"
	"strings"
)

func readConfig(filename string) ([]DomainConfig, error) {
	fs, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	var cfg []DomainConfig
	r := bufio.NewReader(fs)
	for {
		raw, _, err := r.ReadLine()
		if err != nil {
			break
		}
		line := strings.TrimSpace(string(raw))
		ss := strings.Split(line, " ")
		dc := DomainConfig{
			Action: ss[0],
			Domain: ss[1],
		}
		if len(ss) >= 3 {
			dc.Extra = strings.Join(ss[2:], " ")
		}
		cfg = append(cfg, dc)
	}

	return cfg, nil
}

type DomainConfig struct {
	Action string
	Domain string
	Extra  string
}
