package webui

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// updateNPSSOToken reads the YAML config file, updates the psn.npsso_token
// field, and writes it back, preserving structure and comments.
func updateNPSSOToken(configPath, token string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// doc is a Document node; its first child is the top-level mapping.
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected YAML structure")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("expected top-level mapping")
	}

	// Find the "psn" key in the top-level mapping.
	psnNode := findMapValue(root, "psn")
	if psnNode == nil {
		return fmt.Errorf("'psn' section not found in config")
	}
	if psnNode.Kind != yaml.MappingNode {
		return fmt.Errorf("'psn' is not a mapping")
	}

	// Find the "npsso_token" key inside the psn mapping.
	tokenNode := findMapValue(psnNode, "npsso_token")
	if tokenNode == nil {
		return fmt.Errorf("'npsso_token' not found in psn section")
	}

	tokenNode.Value = token

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Preserve original file permissions.
	info, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("stat config: %w", err)
	}

	if err := os.WriteFile(configPath, out, info.Mode()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// findMapValue returns the value node for the given key in a mapping node.
func findMapValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
