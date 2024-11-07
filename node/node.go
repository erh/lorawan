package node

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

// Model represents a lorawan node model.
var Model = resource.NewModel("viam", "lorawan", "node")

type Config struct {
	JoinType    string `json:"join_type,omitempty"`
	DecoderPath string `json:"decoder_path"`
	DevEUI      string `json:"dev_eui,omitempty"`
	AppKey      string `json:"app_key,omitempty"`
	AppSKey     string `json:"app_s_key,omitempty"`
	NwkSKey     string `json:"network_s_key,omitempty"`
	DevAddr     string `json:"dev_addr,omitempty"`
	GatewayName string `json:"gateway,omitempty"`
}

func init() {
	resource.RegisterComponent(
		sensor.API,
		Model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newNode,
		})
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.DecoderPath == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("decoder path is required"))
	}
	switch conf.JoinType {
	case "ABP":
		return conf.validateABPAttributes(path)
	case "OTAA", "":
		return conf.validateOTAAAttributes(path)
	default:
		return nil, resource.NewConfigValidationError(path,
			errors.New("join type is OTAA or ABP"))
	}
}

func (conf *Config) validateOTAAAttributes(path string) ([]string, error) {
	if conf.DevEUI == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("dev EUI is required for OTAA join type"))
	}
	if len(conf.DevEUI) != 16 {
		return nil, resource.NewConfigValidationError(path,
			errors.New("dev EUI must be 8 bytes"))
	}
	if conf.AppKey == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("app key is required for OTAA join type"))
	}
	if len(conf.AppKey) != 32 {
		return nil, resource.NewConfigValidationError(path,
			errors.New("app key must be 16 bytes"))
	}
	return nil, nil
}

func (conf *Config) validateABPAttributes(path string) ([]string, error) {
	if conf.AppSKey == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("app session key is required for ABP join type"))
	}
	if len(conf.AppSKey) != 32 {
		return nil, resource.NewConfigValidationError(path,
			errors.New("app session key must be 16 bytes"))
	}
	if conf.NwkSKey == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("network session key is required for ABP join type"))
	}
	if len(conf.NwkSKey) != 32 {
		return nil, resource.NewConfigValidationError(path,
			errors.New("network session key must be 16 bytes"))
	}
	if conf.DevAddr == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("device address is required for ABP join type"))
	}
	if len(conf.DevAddr) != 8 {
		return nil, resource.NewConfigValidationError(path,
			errors.New("device address must be 4 bytes"))
	}

	return nil, nil

}

type Node struct {
	resource.Named
	resource.AlwaysRebuild
	logger logging.Logger

	workers *utils.StoppableWorkers
	mu      sync.Mutex

	DecoderPath string

	nwkSKey []byte
	AppSKey []byte
	AppKey  []byte

	Addr   []byte
	DevEui []byte

	gateway sensor.Sensor
}

func newNode(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (sensor.Sensor, error) {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	n := &Node{
		Named:       conf.ResourceName().AsNamed(),
		logger:      logger,
		DecoderPath: cfg.DecoderPath,
	}

	gateway, err := sensor.FromDependencies(deps, cfg.GatewayName)
	if err != nil {
		return nil, fmt.Errorf("no gateway named (%s)", cfg.GatewayName)
	}

	n.gateway = gateway

	switch cfg.JoinType {
	case "OTAA", "":
		appKey, err := hex.DecodeString(cfg.AppKey)
		if err != nil {
			return nil, err
		}
		n.AppKey = appKey

		devEui, err := hex.DecodeString(cfg.DevEUI)
		if err != nil {
			return nil, err
		}
		n.DevEui = devEui
	case "ABP":
		devAddr, err := hex.DecodeString(cfg.DevAddr)
		if err != nil {
			return nil, err
		}

		n.Addr = devAddr

		appSKey, err := hex.DecodeString(cfg.AppSKey)
		if err != nil {
			return nil, err
		}

		n.AppSKey = appSKey
	}

	cmd := make(map[string]interface{})

	cmd["register_device"] = n

	fmt.Println("doing do command")

	_, err = gateway.DoCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func (n *Node) Close(ctx context.Context) error {
	return nil
}

func (n *Node) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	allReadings, err := n.gateway.Readings(ctx, nil)
	if err != nil {
		return map[string]interface{}{}, err
	}
	// return allReadings[n.Name().Name].(map[string]interface{}), nil
	return allReadings, nil
}
