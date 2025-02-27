package keaconfig

import (
	"fmt"
	"strings"

	errors "github.com/pkg/errors"
	storkutil "isc.org/stork/util"
)

// DHCP option space (one of dhcp4 or dhcp6).
type DHCPOptionSpace = string

// Top level DHCP option spaces.
const (
	DHCPv4OptionSpace DHCPOptionSpace = "dhcp4"
	DHCPv6OptionSpace DHCPOptionSpace = "dhcp6"
)

// An interface to a DHCP option. Database model representing DHCP options
// implements this interface.
type DHCPOption interface {
	// Returns a boolean flag indicating if the option should be
	// always returned, regardless whether it is requested or not.
	IsAlwaysSend() bool
	// Returns option code.
	GetCode() uint16
	// Returns encapsulated option space name.
	GetEncapsulate() string
	// Returns option fields.
	GetFields() []DHCPOptionField
	// Returns option name.
	GetName() string
	// Returns option space.
	GetSpace() string
	// Returns the universe (i.e., IPv4 or IPv6).
	GetUniverse() storkutil.IPType
}

// Represents a DHCP option in the format used by Kea (i.e., an item of the
// option-data list).
type SingleOptionData struct {
	AlwaysSend bool   `mapstructure:"always-send" json:"always-send,omitempty"`
	Code       uint16 `mapstructure:"code" json:"code,omitempty"`
	CSVFormat  bool   `mapstructure:"csv-format" json:"csv-format"`
	Data       string `mapstructure:"data" json:"data,omitempty"`
	Name       string `mapstructure:"name" json:"name,omitempty"`
	Space      string `mapstructure:"space" json:"space,omitempty"`
}

// Creates a SingleOptionData instance from the DHCP option model used
// by Stork (e.g., from an option held in the Stork database). If the
// option has a definition, it uses the Kea's csv-format setting and
// specifies the option value as a comma separated list. Otherwise, it
// converts the option fields to a hex form and sets the csv-format to
// false. The lookup interface must not be nil.
func CreateSingleOptionData(daemonID int64, lookup DHCPOptionDefinitionLookup, option DHCPOption) (*SingleOptionData, error) {
	// Create Kea representation of the option. Set csv-format to
	// true for all options for which the definitions are known.
	data := &SingleOptionData{
		AlwaysSend: option.IsAlwaysSend(),
		Code:       option.GetCode(),
		CSVFormat:  lookup.DefinitionExists(daemonID, option),
		Name:       option.GetName(),
		Space:      option.GetSpace(),
	}
	// Convert option fields depending on the csv-format setting.
	converted := []string{}
	for _, field := range option.GetFields() {
		var (
			value string
			err   error
		)
		switch field.GetFieldType() {
		case HexBytesField:
			value, err = convertHexBytesField(field)
		case StringField:
			value, err = convertStringField(field, data.CSVFormat)
		case BoolField:
			value, err = convertBoolField(field, data.CSVFormat)
		case Uint8Field, Uint16Field, Uint32Field:
			value, err = convertUintField(field, data.CSVFormat)
		case IPv4AddressField:
			value, err = convertIPv4AddressField(field, data.CSVFormat)
		case IPv6AddressField:
			value, err = convertIPv6AddressField(field, data.CSVFormat)
		case IPv6PrefixField:
			value, err = convertIPv6PrefixField(field, data.CSVFormat)
		case PsidField:
			value, err = convertPsidField(field, data.CSVFormat)
		case FqdnField:
			value, err = convertFqdnField(field, data.CSVFormat)
		default:
			err = errors.Errorf("unsupported option field type %s", field.GetFieldType())
		}
		if err != nil {
			return nil, err
		}
		// The value can be a string with the option field value or a string
		// of hexadecimal digits representing the value.
		converted = append(converted, value)
	}
	if data.CSVFormat {
		// Use comma separated values.
		data.Data = strings.Join(converted, ",")
	} else {
		// The option is specified as a string of hexadecimal digits. Let's
		// just concatenate all option fields into a single string.
		data.Data = strings.Join(converted, "")
	}
	return data, nil
}

// Represents a DHCP option and implements DHCPOption interface. It is returned
// by the CreateDHCPOption function.
type dhcpOption struct {
	AlwaysSend  bool
	Code        uint16
	Encapsulate string
	Fields      []DHCPOptionField
	Name        string
	Space       string
	Universe    storkutil.IPType
}

// Creates an instance of a DHCP option in Stork from the option representation
// in Kea.
func CreateDHCPOption(optionData SingleOptionData, universe storkutil.IPType, lookup DHCPOptionDefinitionLookup) (DHCPOption, error) {
	option := dhcpOption{
		AlwaysSend: optionData.AlwaysSend,
		Code:       optionData.Code,
		Name:       optionData.Name,
		Space:      optionData.Space,
		Universe:   universe,
	}
	data := strings.TrimSpace(optionData.Data)

	// Option encapsulation.
	def := lookup.Find(0, option)
	if def != nil {
		// If the option definition is known, let's take the encapsulated option
		// space name from it.
		option.Encapsulate = def.GetEncapsulate()
	} else {
		// Generate the encapsulated option space name because option
		// definition does not exist in Stork for this option.
		switch option.Space {
		case DHCPv4OptionSpace, DHCPv6OptionSpace:
			option.Encapsulate = fmt.Sprintf("option-%d", option.Code)
		default:
			option.Encapsulate = fmt.Sprintf("%s.%d", option.Space, option.Code)
		}
	}

	// There is nothing to do if the option is empty.
	if len(data) == 0 || (def != nil && def.GetType() == EmptyOption) {
		return option, nil
	}

	// Option data specified as comma separated values.
	if optionData.CSVFormat {
		values := strings.Split(data, ",")
		for i, raw := range values {
			v := strings.TrimSpace(raw)
			var field DHCPOptionField
			if def != nil {
				fieldType, ok := GetDHCPOptionDefinitionFieldType(def, i)
				if !ok {
					break
				}
				// We know option definition so we expect that our option
				// adheres to the specific format. Try to parse the option
				// field with checking whether or not its value has that
				// format.
				var err error
				if field, err = parseDHCPOptionField(fieldType, v); err != nil {
					return nil, err
				}
			} else {
				// We don't know the option definition so we will need to
				// try to infer option field data format.
				field = inferDHCPOptionField(v)
			}
			option.Fields = append(option.Fields, field)
		}
		return option, nil
	}

	// If the csv-format is false the option payload is specified using a string
	// of hexadecimal digits. Sanitize colons and whitespaces.
	data = strings.ReplaceAll(strings.ReplaceAll(data, " ", ""), ":", "")
	field := dhcpOptionField{
		FieldType: HexBytesField,
		Values:    []any{data},
	}
	option.Fields = append(option.Fields, field)

	return option, nil
}
