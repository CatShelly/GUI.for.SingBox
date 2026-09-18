package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadPassword reads user settings at startup and persists a password on first use.
func LoadPassword(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", false, err
	}
	var settings map[string]any
	if err := yaml.Unmarshal(b, &settings); err != nil {
		return "", false, fmt.Errorf("invalid user.yaml: %w", err)
	}
	if value, exists := settings["webuiPassword"]; exists && value != nil {
		password, ok := value.(string)
		if !ok {
			return "", false, fmt.Errorf("user.yaml webuiPassword must be a quoted string")
		}
		if strings.TrimSpace(password) != "" {
			return password, false, nil
		}
	}
	var document yaml.Node
	if err := yaml.Unmarshal(b, &document); err != nil {
		return "", false, err
	}
	if len(document.Content) == 0 || document.Content[0].Tag == "!!null" {
		document = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return "", false, fmt.Errorf("user.yaml must contain a mapping")
	}
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return "", false, err
	}
	password := hex.EncodeToString(random)
	value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: password, Style: yaml.DoubleQuotedStyle}
	found := false
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "webuiPassword" {
			root.Content[i+1] = value
			found = true
			break
		}
	}
	if !found {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "webuiPassword"}, value)
	}
	b, err = yaml.Marshal(&document)
	if err != nil {
		return "", false, err
	}
	if err := atomicWrite(path, b); err != nil {
		return "", false, err
	}
	return password, true, nil
}
