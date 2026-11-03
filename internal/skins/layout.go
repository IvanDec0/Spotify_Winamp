package skins

import (
	"fmt"
	"math"
)

const (
	maxLayoutDepth = 12
	maxLayoutNodes = 500
)

type propKind int

const (
	propBool propKind = iota
	propInt
	propNumber
	propEnum
)

type propRule struct {
	kind    propKind
	minimum float64
	maximum float64
	options map[string]bool
}

func boolProp() propRule { return propRule{kind: propBool} }
func numberProp(min, max float64) propRule {
	return propRule{kind: propNumber, minimum: min, maximum: max}
}
func intProp(min, max float64) propRule {
	return propRule{kind: propInt, minimum: min, maximum: max}
}
func enumProp(options ...string) propRule {
	values := make(map[string]bool, len(options))
	for _, option := range options {
		values[option] = true
	}
	return propRule{kind: propEnum, options: values}
}

var containerPropRules = map[string]map[string]propRule{
	"stack": {
		"gap":     intProp(0, 64),
		"padding": intProp(0, 64),
		"align":   enumProp("start", "center", "end", "stretch"),
	},
	"row": {
		"gap":     intProp(0, 64),
		"padding": intProp(0, 64),
		"align":   enumProp("start", "center", "end", "stretch"),
		"justify": enumProp("start", "center", "end", "space-between", "space-around"),
		"wrap":    boolProp(),
	},
	"column": {
		"gap":     intProp(0, 64),
		"padding": intProp(0, 64),
		"align":   enumProp("start", "center", "end", "stretch"),
		"justify": enumProp("start", "center", "end", "space-between", "space-around"),
	},
	"grid": {
		"gap":     intProp(0, 64),
		"padding": intProp(0, 64),
		"columns": intProp(1, 12),
		"rows":    intProp(1, 12),
		"align":   enumProp("start", "center", "end", "stretch"),
		"flow":    enumProp("row", "column"),
	},
}

var componentPropRules = map[string]map[string]propRule{
	"titlebar": {
		"showBrand":          boolProp(),
		"showProvider":       boolProp(),
		"showDevice":         boolProp(),
		"showWindowControls": boolProp(),
		"compact":            boolProp(),
	},
	"album-art": {
		"size":       intProp(48, 160),
		"frame":      boolProp(),
		"reflection": boolProp(),
	},
	"track-info": {
		"showAlbum":  boolProp(),
		"showStatus": boolProp(),
		"align":      enumProp("left", "center"),
	},
	"progress": {
		"showTime":      boolProp(),
		"showRemaining": boolProp(),
	},
	"transport-controls": {
		"size":        enumProp("compact", "regular", "large"),
		"showShuffle": boolProp(),
		"showRepeat":  boolProp(),
	},
	"volume": {
		"orientation": enumProp("horizontal", "vertical"),
		"showIcon":    boolProp(),
	},
	"shuffle": {
		"showLabel": boolProp(),
	},
	"repeat": {
		"showLabel": boolProp(),
	},
	"queue": {
		"maxItems":   intProp(1, 100),
		"showHeader": boolProp(),
		"showArt":    boolProp(),
		"showTime":   boolProp(),
	},
	"visualizer": {
		// The mode enum is checked against the manifest in validateComponent.
		"mode":      enumProp("spectrum", "radial", "oscilloscope", "grid"),
		"layer":     enumProp("background", "inline"),
		"intensity": numberProp(0, 1),
	},
	"device-picker": {
		"compact": boolProp(),
	},
	"settings-button": {
		"showLabel": boolProp(),
	},
}

var componentCapabilities = map[string][]string{
	"titlebar":           {"player.read"},
	"album-art":          {"player.read"},
	"track-info":         {"player.read"},
	"progress":           {"player.read"},
	"transport-controls": {"player.read", "player.control"},
	"volume":             {"player.read", "player.control"},
	"shuffle":            {"player.read", "player.control"},
	"repeat":             {"player.read", "player.control"},
	"queue":              {"queue.read"},
	"visualizer":         {"player.read"},
	"device-picker":      {"devices.read", "devices.control"},
	"settings-button":    nil,
}

// validateLayoutSemantics enforces the parts of the skin contract that JSON
// Schema cannot express: node budget and depth, unique registered components,
// typed props, capability declarations and a usable root.
func validateLayoutSemantics(layout *Layout, manifest *Manifest) error {
	if layout.SchemaVersion != 1 {
		return validationError("unsupported layout schema version")
	}
	if !isContainer(layout.Root.Type) {
		return validationError("layout root must be a stack, row, column or grid container")
	}
	if len(layout.Root.Children) == 0 {
		return validationError("layout root must contain at least one child")
	}

	seenComponents := make(map[string]bool)
	seenNames := make(map[string]string)
	nodes := 0

	var walk func(node LayoutNode, depth int) error
	walk = func(node LayoutNode, depth int) error {
		nodes++
		if nodes > maxLayoutNodes {
			return validationError("layout exceeds the 500 node limit")
		}
		if depth > maxLayoutDepth {
			return validationError("layout exceeds the maximum depth of 12")
		}

		if node.Name != "" {
			if previous, exists := seenNames[node.Name]; exists {
				return validationError(fmt.Sprintf("layout name %q is used more than once (%s)", node.Name, previous))
			}
			seenNames[node.Name] = node.Type
		}

		if isContainer(node.Type) {
			if node.Component != "" {
				return validationError("containers cannot declare a component")
			}
			if len(node.Children) == 0 {
				return validationError(fmt.Sprintf("%s node %q must contain at least one child", node.Type, node.Name))
			}
			rules, ok := containerPropRules[node.Type]
			if !ok {
				return validationError(fmt.Sprintf("unknown container type %q", node.Type))
			}
			if err := validateProps(node.Props, rules, node.Type); err != nil {
				return err
			}
			for _, child := range node.Children {
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
			return nil
		}

		if node.Type != "component" {
			return validationError(fmt.Sprintf("unknown layout node type %q", node.Type))
		}
		if len(node.Children) != 0 {
			return validationError("component nodes cannot contain children")
		}
		if node.Component == "" {
			return validationError("component node is missing its component name")
		}
		if seenComponents[node.Component] {
			return validationError(fmt.Sprintf("component %q appears more than once", node.Component))
		}
		seenComponents[node.Component] = true
		return validateComponent(node, manifest)
	}

	if err := walk(layout.Root, 1); err != nil {
		return err
	}
	if !seenComponents["titlebar"] {
		return validationError("layout must include the titlebar component so the window can be moved and closed")
	}
	return nil
}

func validateComponent(node LayoutNode, manifest *Manifest) error {
	rules, ok := componentPropRules[node.Component]
	if !ok {
		return validationError(fmt.Sprintf("unknown component %q", node.Component))
	}
	if err := validateProps(node.Props, rules, node.Component); err != nil {
		return err
	}

	for _, capability := range componentCapabilities[node.Component] {
		if !contains(manifest.Capabilities, capability) {
			return validationError(fmt.Sprintf("component %q requires the %q capability", node.Component, capability))
		}
	}

	if node.Component == "visualizer" && node.Props != nil {
		if raw, exists := node.Props["mode"]; exists {
			mode, ok := raw.(string)
			if !ok {
				return validationError("visualizer prop \"mode\" must be a string")
			}
			if !contains(manifest.Visualizer.Modes, mode) {
				return validationError(fmt.Sprintf("visualizer mode %q is not declared in the manifest", mode))
			}
		}
	}
	return nil
}

func validateProps(props map[string]any, rules map[string]propRule, scope string) error {
	for name, raw := range props {
		rule, ok := rules[name]
		if !ok {
			return validationError(fmt.Sprintf("unknown %s prop %q", scope, name))
		}
		switch rule.kind {
		case propBool:
			if _, ok := raw.(bool); !ok {
				return validationError(fmt.Sprintf("%s prop %q must be a boolean", scope, name))
			}
		case propEnum:
			value, ok := raw.(string)
			if !ok {
				return validationError(fmt.Sprintf("%s prop %q must be a string", scope, name))
			}
			if !rule.options[value] {
				return validationError(fmt.Sprintf("%s prop %q has an unsupported value %q", scope, name, value))
			}
		case propInt:
			value, ok := raw.(float64)
			if !ok || math.Trunc(value) != value {
				return validationError(fmt.Sprintf("%s prop %q must be an integer", scope, name))
			}
			if value < rule.minimum || value > rule.maximum {
				return validationError(fmt.Sprintf("%s prop %q must be between %g and %g", scope, name, rule.minimum, rule.maximum))
			}
		case propNumber:
			value, ok := raw.(float64)
			if !ok {
				return validationError(fmt.Sprintf("%s prop %q must be a number", scope, name))
			}
			if value < rule.minimum || value > rule.maximum {
				return validationError(fmt.Sprintf("%s prop %q must be between %g and %g", scope, name, rule.minimum, rule.maximum))
			}
		default:
			return validationError("internal skin prop rule error")
		}
	}
	return nil
}

func isContainer(nodeType string) bool {
	switch nodeType {
	case "stack", "row", "column", "grid":
		return true
	default:
		return false
	}
}
