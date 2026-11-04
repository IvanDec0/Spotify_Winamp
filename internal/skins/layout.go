package skins

import (
	"fmt"
	"math"
	"path"
	"strings"
	"unicode/utf8"
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
	propString
)

type propRule struct {
	kind      propKind
	minimum   float64
	maximum   float64
	minLength int
	maxLength int
	options   map[string]bool
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
func stringProp(min, max int) propRule {
	return propRule{kind: propString, minLength: min, maxLength: max}
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
		"brand":              stringProp(1, 24),
		"showBrand":          boolProp(),
		"showMark":           boolProp(),
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

		switch node.Type {
		case "component":
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
		case "box", "text", "image", "button":
			if node.Component != "" {
				return validationError("decorative nodes cannot declare a component")
			}
			if len(node.Children) != 0 {
				return validationError("decorative nodes cannot contain children")
			}
			return validatePrimitive(node, manifest)
		default:
			return validationError(fmt.Sprintf("unknown layout node type %q", node.Type))
		}
	}

	if err := walk(layout.Root, 1); err != nil {
		return err
	}
	if !seenComponents["titlebar"] {
		return validationError("layout must include the titlebar component so the window can be moved and closed")
	}
	return nil
}

var primitivePropRules = map[string]map[string]propRule{
	"box": {},
	"text": {
		"text": stringProp(1, 120),
	},
	"image": {
		"src": stringProp(1, 200),
		"alt": stringProp(0, 120),
		"fit": enumProp("contain", "cover"),
	},
	"button": {
		"action": enumProp(
			"open-library",
			"open-settings",
			"open-devices",
			"toggle-queue",
			"play-pause",
			"next",
			"previous",
			"shuffle",
			"repeat",
			"cycle-visualizer",
			"cycle-theme",
			"minimise",
			"close",
		),
		"text":  stringProp(0, 24),
		"label": stringProp(0, 80),
	},
}

var primitiveButtonCapabilities = map[string][]string{
	"open-devices":     {"devices.read"},
	"toggle-queue":     {"queue.read"},
	"play-pause":       {"player.control"},
	"next":             {"player.control"},
	"previous":         {"player.control"},
	"shuffle":          {"player.control"},
	"repeat":           {"player.control"},
	"cycle-visualizer": nil,
	"cycle-theme":      nil,
	"open-library":     nil,
	"open-settings":    nil,
	"minimise":         nil,
	"close":            nil,
}

func validatePrimitive(node LayoutNode, manifest *Manifest) error {
	rules := primitivePropRules[node.Type]
	if err := validateProps(node.Props, rules, node.Type); err != nil {
		return err
	}

	switch node.Type {
	case "box":
		return nil
	case "text":
		if _, ok := node.Props["text"]; !ok {
			return validationError("text nodes must declare the text prop")
		}
	case "image":
		raw, ok := node.Props["src"]
		if !ok {
			return validationError("image nodes must declare the src prop")
		}
		if err := validateImageSource(raw.(string)); err != nil {
			return err
		}
	case "button":
		raw, ok := node.Props["action"]
		if !ok {
			return validationError("button nodes must declare the action prop")
		}
		action := raw.(string)
		for _, capability := range primitiveButtonCapabilities[action] {
			if !contains(manifest.Capabilities, capability) {
				return validationError(fmt.Sprintf("button action %q requires the %q capability", action, capability))
			}
		}
		text, _ := node.Props["text"].(string)
		label, _ := node.Props["label"].(string)
		if text == "" && label == "" {
			return validationError("button nodes must declare text or label")
		}
	}
	return nil
}

func validateImageSource(src string) error {
	if src == "" || strings.Contains(src, "\\") || strings.Contains(src, "://") || strings.HasPrefix(src, "/") {
		return validationError("image src must be a relative skin asset")
	}
	cleaned := path.Clean(src)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return validationError("image src must stay inside the skin directory")
	}
	extension := strings.ToLower(path.Ext(cleaned))
	if !isAssetExtension(extension) {
		return validationError(fmt.Sprintf("image src has an unsupported asset type: %s", src))
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
		case propString:
			value, ok := raw.(string)
			if !ok {
				return validationError(fmt.Sprintf("%s prop %q must be a string", scope, name))
			}
			length := utf8.RuneCountInString(value)
			if length < rule.minLength || length > rule.maxLength {
				return validationError(fmt.Sprintf("%s prop %q must be between %d and %d characters", scope, name, rule.minLength, rule.maxLength))
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
