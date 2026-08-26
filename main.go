package main

import (
	"fmt"
	"os"

	"github.com/siliconwitchery/superstack-cli/internal/account"
	"github.com/siliconwitchery/superstack-cli/internal/device"
	"github.com/siliconwitchery/superstack-cli/internal/dispatch"
	"github.com/siliconwitchery/superstack-cli/internal/fleet"
	"github.com/siliconwitchery/superstack-cli/internal/fleetkey"
	"github.com/siliconwitchery/superstack-cli/internal/login"
	"github.com/siliconwitchery/superstack-cli/internal/member"
)

const version = "0.0.4"

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
			{Name: "fleet delete", Arguments: "<fleet_id>", Summary: "Delete a fleet and unpair its devices", Run: fleet.Delete},
		},
	},
	{
		Title: "Devices",
		Commands: []dispatch.Command{
			{Name: "device list", Arguments: "[fleet_id] [--json]", Summary: "List devices and when they were last seen", Run: device.List},
			{Name: "device pair", Arguments: "<imei> <fleet_id>", Summary: "Pair a device into a fleet", Run: device.Pair},
			{Name: "device rename", Arguments: "<imei> <new_name>", Summary: "Rename a device", Run: device.Rename},
			{Name: "device unpair", Arguments: "<imei>", Summary: "Remove a device from its fleet", Run: device.Unpair},
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
			{Name: "member add", Arguments: "<email> <fleet_id>", Summary: "Add a member to a fleet", Run: member.Add},
			{Name: "member list", Arguments: "<fleet_id> [--json]", Summary: "List a fleet's owner and members", Run: member.List},
			{Name: "member remove", Arguments: "<email> <fleet_id>", Summary: "Remove a member from a fleet", Run: member.Remove},
		},
	},
	{
		Title: "Keys",
		Commands: []dispatch.Command{
			{Name: "key create", Arguments: "<fleet_id> <label>", Summary: "Create a key for sending data to a fleet", Run: fleetkey.Create},
			{Name: "key list", Arguments: "[fleet_id] [--json]", Summary: "List the keys that can reach your fleets", Run: fleetkey.List},
			{Name: "key revoke", Arguments: "<key_id>", Summary: "Stop a key from reaching its fleet", Run: fleetkey.Revoke},
		},
	},
	{
		Title: "Account",
		Commands: []dispatch.Command{
			{Name: "account balance", Arguments: "[fleet_id] [--json]", Summary: "Show the credit left on your fleets", Run: account.Balance},
			{Name: "account topup", Arguments: "<fleet_id>", Summary: "Add credit to a fleet", Run: account.TopUp},
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
