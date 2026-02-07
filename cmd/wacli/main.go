package main

import (
	"os"
	"strings"

	"github.com/steipete/wacli/internal/logging"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"google.golang.org/protobuf/proto"
)

func main() {
	applyDeviceLabel()
	logging.Debug().Strs("args", os.Args[1:]).Msg("wacli starting")
	if err := execute(os.Args[1:]); err != nil {
		logging.Error().Err(err).Msg("command failed")
		os.Exit(1)
	}
	logging.Debug().Msg("wacli finished")
}

func applyDeviceLabel() {
	label := strings.TrimSpace(os.Getenv("WACLI_DEVICE_LABEL"))
	platformRaw := strings.TrimSpace(os.Getenv("WACLI_DEVICE_PLATFORM"))
	if platformRaw != "" {
		platform := parsePlatformType(platformRaw)
		store.DeviceProps.PlatformType = platform.Enum()
	}
	if label == "" {
		return
	}
	store.SetOSInfo(label, [3]uint32{0, 1, 0})
	store.BaseClientPayload.UserAgent.Device = proto.String(label)
	store.BaseClientPayload.UserAgent.Manufacturer = proto.String(label)
}

func parsePlatformType(raw string) waCompanionReg.DeviceProps_PlatformType {
	value := strings.TrimSpace(raw)
	if value == "" {
		return waCompanionReg.DeviceProps_CHROME
	}
	value = strings.ToUpper(value)
	if enumValue, ok := waCompanionReg.DeviceProps_PlatformType_value[value]; ok {
		return waCompanionReg.DeviceProps_PlatformType(enumValue)
	}
	return waCompanionReg.DeviceProps_CHROME
}
