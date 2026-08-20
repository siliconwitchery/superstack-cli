package main

import (
	"fmt"
	"os"

	"github.com/siliconwitchery/superstack-cli/internal/account"
	"github.com/siliconwitchery/superstack-cli/internal/device"
	"github.com/siliconwitchery/superstack-cli/internal/dispatch"
	"github.com/siliconwitchery/superstack-cli/internal/fleet"
	"github.com/siliconwitchery/superstack-cli/internal/key"
	"github.com/siliconwitchery/superstack-cli/internal/login"
	"github.com/siliconwitchery/superstack-cli/internal/member"
)

const version = "0.0.3"

var sections = []dispatch.Section{
	{
		Title: "Getting started",
		Commands: []dispatch.Command{
			{Name: "login", Arguments: "<github|gitlab>", Summary: "Log in with the selected provider", Run: login.Login},
			{Name: "logout", Summary: "Log out of your account", Run: login.Logout},
		},
	},
	{
		Title: "Fleets",
		Commands: []dispatch.Command{
			{Name: "fleet create", Arguments: "<name>", Summary: "Create a fleet", Run: fleet.Create},
			{Name: "fleet list", Arguments: "[--json]", Summary: "List the fleets you can reach", Run: fleet.List},
			{Name: "fleet rename", Arguments: "<fleet_id> <new_name>", Summary: "Rename a fleet", Run: fleet.Rename},
			{Name: "fleet transfer", Arguments: "<fleet_id> <email>", Summary: "Hand a fleet to a new owner", Run: fleet.Transfer},
			{Name: "fleet delete", Arguments: "<fleet_id>", Summary: "Delete a fleet and release its devices", Run: fleet.Delete},
		},
	},
	{
		Title: "Devices",
		Commands: []dispatch.Command{
			{Name: "device claim", Arguments: "<imei> <fleet_id> [name]", Summary: "Claim a device into a fleet, then press its pairing button", Run: device.Claim},
			{Name: "device list", Arguments: "[fleet_id] [--json]", Summary: "List devices, their state, and when they were last seen", Run: device.List},
			{Name: "device rename", Arguments: "<imei> <new_name>", Summary: "Rename a device", Run: device.Rename},
			{Name: "device release", Arguments: "<imei>", Summary: "Release a device from its fleet and erase everything on it", Run: device.Release},
			{Name: "device start", Arguments: "<imei>", Summary: "Start the code on a device"},
			{Name: "device stop", Arguments: "<imei>", Summary: "Stop the code on a device"},
			{Name: "device restart", Arguments: "<imei>", Summary: "Restart the code on a device"},
		},
	},
	{
		Title: "Files",
		Commands: []dispatch.Command{
			{Name: "upload", Arguments: "<imei|fleet_id> <file> ...", Summary: "Upload files or directories to a device or fleet"},
			{Name: "download", Arguments: "<imei|fleet_id> <path>", Summary: "Download a device or fleet's files into <path>"},
			{Name: "dev", Arguments: "<imei|fleet_id> <file> ... [--log-file <file>]", Summary: "Upload on every change, and tail"},
		},
	},
	{
		Title: "Logs",
		Commands: []dispatch.Command{
			{Name: "tail", Arguments: "<imei|fleet_id> [-n num] [--log-file <file>]", Summary: "Stream a device or fleet's log as it arrives"},
		},
	},
	{
		Title: "People",
		Commands: []dispatch.Command{
			{Name: "member add", Arguments: "<email> <fleet_id>", Summary: "Give someone access to a fleet", Run: member.Add},
			{Name: "member list", Arguments: "<fleet_id> [--json]", Summary: "List the people who can reach a fleet", Run: member.List},
			{Name: "member remove", Arguments: "<email> <fleet_id>", Summary: "Take away someone's access", Run: member.Remove},
		},
	},
	{
		Title: "Keys",
		Commands: []dispatch.Command{
			{Name: "key create", Arguments: "<fleet_id> <label>", Summary: "Create a key for sending data to a fleet", Run: key.Create},
			{Name: "key list", Arguments: "[fleet_id] [--json]", Summary: "List the keys that can reach your fleets", Run: key.List},
			{Name: "key revoke", Arguments: "<key_id>", Summary: "Stop a key from reaching its fleet", Run: key.Revoke},
		},
	},
	{
		Title: "Account",
		Commands: []dispatch.Command{
			{Name: "account balance", Arguments: "[fleet_id] [--json]", Summary: "Show the credit left on your fleets", Run: account.Balance},
			{Name: "account topup", Arguments: "<fleet_id>", Summary: "Add credit to a fleet", Run: account.Topup},
			{Name: "account delete", Summary: "Delete your account entirely", Run: account.Delete},
		},
	},
	{
		Title: "Superstack",
		Commands: []dispatch.Command{
			{Name: "version", Summary: "Show the version"},
			{Name: "help", Arguments: "[command]", Summary: "Show this help, or help for one command"},
		},
	},
}

func main() {
	err := dispatch.Dispatch(sections, version, os.Args[1:], os.Stdin, os.Stdout)

	if err != nil {
		fmt.Fprintf(os.Stderr, "superstack: %s\n", err)
		os.Exit(1)
	}
}
