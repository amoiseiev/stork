package keactrl

import (
	"encoding/json"
	"reflect"
	"sort"

	"github.com/pkg/errors"
	storkutil "isc.org/stork/util"
)

const (
	ResponseSuccess            = 0
	ResponseError              = 1
	ResponseCommandUnsupported = 2
	ResponseEmpty              = 3
)

// Interface to a Kea command that can be marshalled and sent.
type SerializableCommand interface {
	GetCommand() string
	GetDaemonsList() []string
	Marshal() string
}

// Represents a command sent to Kea including command name, daemons list
// (service list in Kea terms) and arguments.
type Command struct {
	Command   string      `json:"command"`
	Daemons   []string    `json:"service,omitempty" mapstructure:"service"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// Common fields in each received Kea response.
type ResponseHeader struct {
	Result int    `json:"result"`
	Text   string `json:"text"`
	Daemon string `json:"-"`
}

// Represents unmarshaled response from Kea daemon.
type Response struct {
	ResponseHeader
	Arguments *map[string]interface{} `json:"arguments,omitempty"`
}

// A list of responses from multiple Kea daemons by the Kea Control Agent.
type ResponseList []Response

// Represents unmarshaled response from Kea daemon with hash value computed
// from the arguments.
type HashedResponse struct {
	ResponseHeader
	Arguments     *map[string]interface{} `json:"arguments,omitempty"`
	ArgumentsHash string                  `json:"-"`
}

// A list of responses including hash value computed from the arguments.
type HashedResponseList []HashedResponse

// An interface exposing properties of the response allowing for
// error checking.
type ExaminableResponse interface {
	GetResult() int
	GetText() string
	GetDaemon() string
	GetArguments() *map[string]interface{}
}

// In some cases we need to compute a hash from the arguments received
// in a response. The arguments are passed as a string to a hashing
// function. Capturing the arguments as string requires hooking up to
// the JSON unmarshaller with a custom unmarshalling function. The
// hasherValue and hasher types serve this purpose.
type hasherValue string

type hasher struct {
	Value *hasherValue `json:"arguments,omitempty"`
}

// Custom unmarshaller hashing arguments string with FNV128 hashing function.
func (v *hasherValue) UnmarshalJSON(b []byte) error {
	*v = hasherValue(storkutil.Fnv128(string(b)))
	return nil
}

// Creates new Kea command from specified command name, daemons list and arguments.
// The arguments are required to be a map or struct.
func NewCommand(command string, daemons []string, arguments interface{}) *Command {
	if len(command) == 0 {
		return nil
	}
	if arguments != nil {
		argsType := reflect.TypeOf(arguments)
		switch argsType.Kind() {
		case reflect.Map, reflect.Struct:
			break
		case reflect.Ptr:
			if argsType.Elem().Kind() != reflect.Struct {
				return nil
			}
		default:
			return nil
		}
	}
	sort.Strings(daemons)
	cmd := &Command{
		Command:   command,
		Daemons:   daemons,
		Arguments: arguments,
	}
	return cmd
}

func NewCommandFromJSON(jsonCommand string) (*Command, error) {
	cmd := Command{}
	err := json.Unmarshal([]byte(jsonCommand), &cmd)
	if err != nil {
		err = errors.Wrapf(err, "failed to parse Kea command: %s", jsonCommand)
		return nil, err
	}
	return &cmd, nil
}

// Returns JSON representation of the Kea command, which can be sent to
// the Kea servers over GRPC.
func (c Command) Marshal() string {
	bytes, _ := json.Marshal(c)
	return string(bytes)
}

// Returns command name.
func (c Command) GetCommand() string {
	return c.Command
}

// Returns daemon names specified within the command.
func (c Command) GetDaemonsList() []string {
	return c.Daemons
}

// Converts parsed response from the Kea Control Agent to JSON. The "parsed"
// argument should be a slice of Response, HashedResponse or similar structures.
func MarshalResponseList(parsed interface{}) ([]byte, error) {
	// All computed fields are ignored internally by the json package.
	return json.Marshal(parsed)
}

// Parses response received from the Kea Control Agent. The "parsed" argument
// should be a slice of Response, HashedResponse or similar structures.
func UnmarshalResponseList(request SerializableCommand, response []byte, parsed interface{}) error {
	err := json.Unmarshal(response, parsed)
	if err != nil {
		err = errors.Wrapf(err, "failed to parse responses from Kea: %s", response)
		return err
	}

	// Try to match the responses with the services in the request and tag them with
	// the service names.
	parsedList := reflect.ValueOf(parsed).Elem()

	daemonNames := request.GetDaemonsList()
	if (len(daemonNames) > 0) && (parsedList.Len() > 0) {
		for i, daemon := range daemonNames {
			if i >= parsedList.Len() {
				break
			}
			parsedElem := parsedList.Index(i)
			field := parsedElem.FieldByName("Daemon")
			if field.IsValid() {
				field.SetString(daemon)
			}
		}
	}

	// Start computing hashes from the arguments received in the response.
	// We may consider optimizing it to hash while unmarshalling the response. This,
	// however, would require having a dedicated structure for arguments and custom
	// unmarshaller to be implemented for it. While this makes sense, it gives
	// significantly less flexibility on the caller side to use different structures
	// into which the responses are unmarshalled. Hopefully, several milliseconds more
	// for hashing the response doesn't matter for user experience, especially since
	// it is conducted in the background.
	hashers := []hasher{}
	for i := 0; i < parsedList.Len(); i++ {
		// First, we have to check if the response contains ArgumentsHash field.
		// Existence of this field is an indication that a caller wants us to
		// compute a hash.
		parsedElem := parsedList.Index(i)
		field := parsedElem.FieldByName("ArgumentsHash")
		if !field.IsValid() {
			// Response struct does not contain the ArgumentsHash, so there is
			// nothing to do.
			break
		}
		// If we haven't yet computed the hashes, let's do it now. We use
		// custom unmarshaller which will read the arguments parameter from
		// the response and compute hashes for each daemon from which a
		// response has been received.
		if len(hashers) == 0 {
			err = json.Unmarshal(response, &hashers)
			if err != nil {
				err = errors.Wrapf(err, "failed to compute hashes for Kea responses: %s", response)
				return err
			}
		}
		// This should not happen but let's be safe.
		if i >= len(hashers) {
			break
		}
		// Let's copy the hash value to the response if the hash exists. It may
		// be nil when no arguments were received in the response.
		if hashers[i].Value != nil {
			field.SetString(string(*hashers[i].Value))
		}
	}

	return nil
}

// Returns status code.
func (r Response) GetResult() int {
	return r.Result
}

// Returns status text.
func (r Response) GetText() string {
	return r.Text
}

// Returns name of the daemon that returned the response.
func (r Response) GetDaemon() string {
	return r.Daemon
}

// Returns response arguments.
func (r Response) GetArguments() *map[string]interface{} {
	return r.Arguments
}

// Returns status code.
func (r HashedResponse) GetResult() int {
	return r.Result
}

// Returns status text.
func (r HashedResponse) GetText() string {
	return r.Text
}

// Returns name of the daemon that returned the response.
func (r HashedResponse) GetDaemon() string {
	return r.Daemon
}

// Returns response arguments.
func (r HashedResponse) GetArguments() *map[string]interface{} {
	return r.Arguments
}

// Check response status code and returns appropriate error or nil if the
// response was successful.
func GetResponseError(response ExaminableResponse) (err error) {
	if response.GetResult() == ResponseError || response.GetResult() == ResponseCommandUnsupported {
		statusName := "error"
		if response.GetResult() == ResponseCommandUnsupported {
			statusName = "unsupported command"
		}
		var daemon string
		if len(response.GetDaemon()) > 0 {
			daemon = "Kea " + response.GetDaemon() + " daemon"
		} else {
			daemon = "Kea"
		}
		err = errors.Errorf("%s status (%d) returned by %s with text: '%s'",
			statusName, response.GetResult(), daemon, response.GetText())
	}
	return
}
