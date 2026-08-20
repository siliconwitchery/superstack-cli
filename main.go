package main

import (
	"fmt"
	"os"

	"github.com/siliconwitchery/superstack-cli/internal/commands"
)

const version = "0.0.3"

var sections = []commands.Section{
	{
		Title: "Getting started",
		Commands: []commands.Command{
			{Name: "login", Arguments: "<github|gitlab>", Summary: "Log in with the selected provider", Run: commands.Login},
			{Name: "logout", Summary: "Log out of your account", Run: commands.Logout},
		},
	},
	{
		Title: "Fleets",
		Commands: []commands.Command{
			{Name: "fleet create", Arguments: "<name>", Summary: "Create a fleet", Run: commands.FleetCreate},
			{Name: "fleet list", Arguments: "[--json]", Summary: "List the fleets you can reach", Run: commands.FleetList},
			{Name: "fleet rename", Arguments: "<fleet_id> <new_name>", Summary: "Rename a fleet", Run: commands.FleetRename},
			{Name: "fleet transfer", Arguments: "<fleet_id> <email>", Summary: "Hand a fleet to a new owner", Run: commands.FleetTransfer},
			{Name: "fleet delete", Arguments: "<fleet_id>", Summary: "Delete a fleet and release its devices", Run: commands.FleetDelete},
		},
	},
	{
		Title: "Devices",
		Commands: []commands.Command{
			{Name: "device claim", Arguments: "<imei> <fleet_id> [name]", Summary: "Claim a device into a fleet, then press its pairing button", Run: commands.DeviceClaim},
			{Name: "device list", Arguments: "[fleet_id] [--json]", Summary: "List devices, their state, and when they were last seen", Run: commands.DeviceList},
			{Name: "device rename", Arguments: "<imei> <new_name>", Summary: "Rename a device", Run: commands.DeviceRename},
			{Name: "device release", Arguments: "<imei>", Summary: "Release a device from its fleet and erase everything on it", Run: commands.DeviceRelease},
			{Name: "device start", Arguments: "<imei>", Summary: "Start the code on a device"},
			{Name: "device stop", Arguments: "<imei>", Summary: "Stop the code on a device"},
			{Name: "device restart", Arguments: "<imei>", Summary: "Restart the code on a device"},
		},
	},
	{
		Title: "Files",
		Commands: []commands.Command{
			{Name: "upload", Arguments: "<imei|fleet_id> <file> ...", Summary: "Upload files or directories to a device or fleet"},
			{Name: "download", Arguments: "<imei|fleet_id> <path>", Summary: "Download a device or fleet's files into <path>"},
			{Name: "dev", Arguments: "<imei|fleet_id> <file> ... [--log-file <file>]", Summary: "Upload on every change, and tail"},
		},
	},
	{
		Title: "Logs",
		Commands: []commands.Command{
			{Name: "tail", Arguments: "<imei|fleet_id> [-n num] [--log-file <file>]", Summary: "Stream a device or fleet's log as it arrives"},
		},
	},
	{
		Title: "People",
		Commands: []commands.Command{
			{Name: "member add", Arguments: "<email> <fleet_id>", Summary: "Give someone access to a fleet", Run: commands.MemberAdd},
			{Name: "member list", Arguments: "<fleet_id> [--json]", Summary: "List the people who can reach a fleet", Run: commands.MemberList},
			{Name: "member remove", Arguments: "<email> <fleet_id>", Summary: "Take away someone's access", Run: commands.MemberRemove},
		},
	},
	{
		Title: "Keys",
		Commands: []commands.Command{
			{Name: "key create", Arguments: "<fleet_id> <label>", Summary: "Create a key for sending data to a fleet", Run: commands.KeyCreate},
			{Name: "key list", Arguments: "[fleet_id] [--json]", Summary: "List the keys that can reach your fleets", Run: commands.KeyList},
			{Name: "key revoke", Arguments: "<key_id>", Summary: "Stop a key from reaching its fleet", Run: commands.KeyRevoke},
		},
	},
	{
		Title: "Account",
		Commands: []commands.Command{
			{Name: "account balance", Arguments: "[fleet_id] [--json]", Summary: "Show the credit left on your fleets", Run: commands.AccountBalance},
			{Name: "account topup", Arguments: "<fleet_id>", Summary: "Add credit to a fleet", Run: commands.AccountTopup},
			{Name: "account delete", Summary: "Delete your account entirely", Run: commands.AccountDelete},
		},
	},
	{
		Title: "Superstack",
		Commands: []commands.Command{
			{Name: "version", Summary: "Show the version"},
			{Name: "help", Arguments: "[command]", Summary: "Show this help, or help for one command"},
		},
	},
}

func main() {
	err := commands.Dispatch(sections, version, os.Args[1:], os.Stdin, os.Stdout)

	if err != nil {
		fmt.Fprintf(os.Stderr, "superstack: %s\n", err)
		os.Exit(1)
	}
}
