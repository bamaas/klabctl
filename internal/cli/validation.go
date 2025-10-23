package cli

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/bamaas/klabctl/internal/config"
	"github.com/bamaas/klabctl/internal/templates"
	"gopkg.in/yaml.v3"
)

// discoverComponentSchemas finds all component schemas in the embedded templates
func discoverComponentSchemas() (map[string]*config.ComponentSchema, error) {
	schemas := make(map[string]*config.ComponentSchema)

	// Walk the apps directory
	err := fs.WalkDir(templates.EmbeddedTemplates, "apps", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Look for schema.yaml files
		if !d.IsDir() && d.Name() == "schema.yaml" {
			// Read and parse schema
			schemaData, err := fs.ReadFile(templates.EmbeddedTemplates, path)
			if err != nil {
				return fmt.Errorf("failed to read schema %s: %w", path, err)
			}

			var schema config.ComponentSchema
			if err := yaml.Unmarshal(schemaData, &schema); err != nil {
				return fmt.Errorf("failed to parse schema %s: %w", path, err)
			}

			// Validate schema itself
			if schema.ComponentName == "" {
				return fmt.Errorf("schema %s missing componentName field", path)
			}

			schemas[schema.ComponentName] = &schema
		}

		return nil
	})

	return schemas, err
}

// validateSiteAgainstSchemas validates site.yaml against all component schemas
func validateSiteAgainstSchemas(site *config.Site) error {
	// Discover all component schemas
	schemas, err := discoverComponentSchemas()
	if err != nil {
		return fmt.Errorf("failed to discover schemas: %w", err)
	}

	if len(schemas) == 0 {
		fmt.Println("⚠ Warning: No component schemas found, skipping validation")
		return nil
	}

	fmt.Printf("Found %d component schema(s)\n", len(schemas))

	var allErrors []string

	// Validate each enabled component
	for componentName, component := range site.Spec.Apps.Catalog {
		if !component.Enabled {
			continue
		}

		// Check if schema exists for this component
		schema, hasSchema := schemas[componentName]
		if !hasSchema {
			// No schema = no validation (backward compatible)
			continue
		}

		// Validate component
		if errs := validateComponent(componentName, component, schema); len(errs) > 0 {
			allErrors = append(allErrors, errs...)
		} else {
			fmt.Printf("  ✓ %s: validated\n", componentName)
		}
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("validation errors:\n\n%s", strings.Join(allErrors, "\n"))
	}

	fmt.Printf("\n✓ All enabled components validated successfully\n\n")
	return nil
}

// validateComponent validates a single component against its schema
func validateComponent(
	componentName string,
	component config.Component,
	schema *config.ComponentSchema,
) []string {
	var errors []string

	// Validate values
	for fieldName, fieldSchema := range schema.Values {
		fieldPath := fmt.Sprintf("%s.values.%s", componentName, fieldName)

		// Handle nested fields (e.g., "cloudflare.apiToken")
		fieldValue, exists := getNestedValue(component.Values, fieldName)

		// Check required
		if fieldSchema.Required && !exists {
			errors = append(errors, fmt.Sprintf(
				"[%s] Required field missing: %s\n  Description: %s\n  Example: %s",
				componentName, fieldPath, fieldSchema.Description, fieldSchema.Example))
			continue
		}

		// Skip validation if field doesn't exist and isn't required
		if !exists {
			continue
		}

		// Validate type and format
		if errs := validateFieldValue(fieldPath, fieldValue, fieldSchema); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	return errors
}

// validateFieldValue validates a field value against its schema
func validateFieldValue(fieldPath string, value interface{}, schema config.ValueSchema) []string {
	var errors []string

	// Type checking
	switch schema.Type {
	case "string":
		strVal, ok := value.(string)
		if !ok {
			errors = append(errors, fmt.Sprintf(
				"[%s] Type mismatch: expected string, got %T", fieldPath, value))
			return errors
		}

		// Format validation
		switch schema.Format {
		case "ipv4":
			if !isValidIPv4(strVal) {
				errors = append(errors, fmt.Sprintf(
					"[%s] Invalid IPv4 address: %s", fieldPath, strVal))
			}
		case "hostname":
			if !isValidHostname(strVal) {
				errors = append(errors, fmt.Sprintf(
					"[%s] Invalid hostname: %s", fieldPath, strVal))
			}
		case "email":
			if !isValidEmail(strVal) {
				errors = append(errors, fmt.Sprintf(
					"[%s] Invalid email: %s", fieldPath, strVal))
			}
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			errors = append(errors, fmt.Sprintf(
				"[%s] Type mismatch: expected boolean, got %T", fieldPath, value))
		}

	case "number":
		switch value.(type) {
		case int, int64, float64:
			// Valid number types
		default:
			errors = append(errors, fmt.Sprintf(
				"[%s] Type mismatch: expected number, got %T", fieldPath, value))
		}

	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			errors = append(errors, fmt.Sprintf(
				"[%s] Type mismatch: expected object, got %T", fieldPath, value))
		}
	}

	return errors
}

// Helper validation functions

func isValidIPv4(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}
	return true
}

func isValidHostname(hostname string) bool {
	// Simple hostname validation
	return len(hostname) > 0 && len(hostname) <= 253
}

func isValidEmail(email string) bool {
	// Simple email validation
	return strings.Contains(email, "@") && len(email) > 3
}

func getNestedValue(values map[string]interface{}, path string) (interface{}, bool) {
	// Handle nested paths like "cloudflare.apiToken"
	parts := strings.Split(path, ".")

	current := values
	for i, part := range parts {
		val, exists := current[part]
		if !exists {
			return nil, false
		}

		// Last part - return the value
		if i == len(parts)-1 {
			return val, true
		}

		// Not last part - must be a map
		nextMap, ok := val.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = nextMap
	}

	return nil, false
}
