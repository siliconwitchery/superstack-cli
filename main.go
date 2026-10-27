package main

import (
	"fmt"
	"os"

	"github.com/siliconwitchery/superstack-cli/internal/account"
	"github.com/siliconwitchery/superstack-cli/internal/dev"
	"github.com/siliconwitchery/superstack-cli/internal/device"
	"github.com/siliconwitchery/superstack-cli/internal/dispatch"
	"github.com/siliconwitchery/superstack-cli/internal/files"
	"github.com/siliconwitchery/superstack-cli/internal/fleet"
	"github.com/siliconwitchery/superstack-cli/internal/fleetkey"
	"github.com/siliconwitchery/superstack-cli/internal/login"
	"github.com/siliconwitchery/superstack-cli/internal/member"
	"github.com/siliconwitchery/superstack-cli/internal/tail"
)

const version = "0.0.4"

var sections = []dispatch.Section{
	{
		Title: "Getting started",
		Commands: []dispatch.Command{
			{Name: "login", Arguments: "<github|gitlab>", Summary: "Log in with the selected provider", Run: login.Login,
				Details: "Shows a one-time code, then opens the provider's page in your browser\n" +
					"to enter it. The login is saved on this computer until you log out."},
			{Name: "logout", Summary: "Log out of your account", Run: login.Logout,
				Details: "Ends the login on the server and removes it from this computer."},
		},
	},
	{
		Title: "Fleets",
		Commands: []dispatch.Command{
			{Name: "fleet create", Arguments: "<name>", Summary: "Create a fleet", Run: fleet.Create},
			{Name: "fleet list", Arguments: "[--json]", Summary: "List the fleets you can reach", Run: fleet.List},
			{Name: "fleet rename", Arguments: "<fleet_id> <new_name>", Summary: "Rename a fleet", Run: fleet.Rename},
			{Name: "fleet transfer", Arguments: "<fleet_id> <email>", Summary: "Hand a fleet to a new owner", Run: fleet.Transfer,
				Details: "The new owner must have logged in at least once. Asks first, because\n" +
					"you lose access to the fleet, its devices and its credit."},
			{Name: "fleet delete", Arguments: "<fleet_id>", Summary: "Delete a fleet and unpair its devices", Run: fleet.Delete,
				Details: "Asks first. Each device wipes its code and waits to be paired again,\n" +
					"and any credit left on the fleet is lost."},
		},
	},
	{
		Title: "Devices",
		Commands: []dispatch.Command{
			{Name: "device pair", Arguments: "<imei> <fleet_id> [name]", Summary: "Pair a device with a fleet", Run: device.Pair,
				Details: "The IMEI is the 15-digit number printed on the device. Press the\n" +
					"pairing button on the device within 60 seconds of running this."},
			{Name: "device list", Arguments: "[fleet_id] [--json]", Summary: "List devices and when they were last seen", Run: device.List},
			{Name: "device rename", Arguments: "<imei> <new_name>", Summary: "Rename a device", Run: device.Rename},
			{Name: "device unpair", Arguments: "<imei>", Summary: "Remove a device from its fleet", Run: device.Unpair,
				Details: "Asks first. The device wipes its code and waits to be paired again."},
			{Name: "device start", Arguments: "<imei>", Summary: "Start the code on a device", Run: device.Start,
				Details: "The device starts its code at its next check-in."},
			{Name: "device stop", Arguments: "<imei>", Summary: "Stop the code on a device", Run: device.Stop,
				Details: "The device stops its code at its next check-in."},
			{Name: "device restart", Arguments: "<imei>", Summary: "Restart the code on a device", Run: device.Restart,
				Details: "The device restarts its code at its next check-in."},
		},
	},
	{
		Title: "Files",
		Commands: []dispatch.Command{
			{Name: "upload", Arguments: "<imei|fleet_id> <file> ...", Summary: "Upload files or directories to a device or fleet", Run: files.Upload,
				Details: "Replaces all the code on the device, or on every device now in the\n" +
					"fleet. The files must include main.lua. A file goes to the top level of\n" +
					"the device under its own name. A directory's contents go to the top\n" +
					"level, keeping the folders inside it, except names that start with a\n" +
					"dot. Each device picks up the code at its next check-in."},
			{Name: "download", Arguments: "<imei> <path>", Summary: "Download the code last uploaded to a device into <path>", Run: files.Download,
				Details: "Asks before replacing a file that already exists in <path>."},
			{Name: "dev", Arguments: "<imei|fleet_id> <file> ...", Summary: "Upload on every change, and follow the log from the first upload", Run: dev.Dev,
				Details: "Uploads the files as upload does, then follows the log of the device or\n" +
					"fleet from now on. Each time the files change, it uploads them again.\n" +
					"It runs until you press Ctrl-C."},
		},
	},
	{
		Title: "Logs",
		Commands: []dispatch.Command{
			{Name: "tail", Arguments: "<fleet_id> [imei ...] [-n num [-o num]]", Summary: "Follow a fleet's log as it arrives, or print a window of its past logs with -n", Run: tail.Tail,
				Details: "Shows the fleet's newest 100 logs, then follows new ones until you\n" +
					"press Ctrl-C. Name IMEIs after the fleet id to show only those devices.\n" +
					"-n num prints the newest num logs, from 1 to 1000, and stops. -o num\n" +
					"skips that many of the newest logs first, so -n 100 -o 100 prints the\n" +
					"100 before them. Each line shows when the log arrived, the device's\n" +
					"IMEI, the kind of log and the device's name, then the text."},
		},
	},
	{
		Title: "People",
		Commands: []dispatch.Command{
			{Name: "member add", Arguments: "<email> <fleet_id>", Summary: "Add a member to a fleet", Run: member.Add,
				Details: "The person must have logged in at least once. A member can do anything\n" +
					"with the fleet's devices, but cannot change the fleet itself."},
			{Name: "member list", Arguments: "<fleet_id> [--json]", Summary: "List a fleet's owner and members", Run: member.List},
			{Name: "member remove", Arguments: "<email> <fleet_id>", Summary: "Remove a member from a fleet", Run: member.Remove,
				Details: "Asks first. The owner cannot be removed."},
		},
	},
	{
		Title: "Keys",
		Commands: []dispatch.Command{
			{Name: "key create", Arguments: "<fleet_id> <label>", Summary: "Create a key for sending data to a fleet", Run: fleetkey.Create,
				Details: "Shows the key once, so copy it then. Anyone holding it can send data to\n" +
					"the fleet."},
			{Name: "key list", Arguments: "[fleet_id] [--json]", Summary: "List the keys that can reach your fleets", Run: fleetkey.List},
			{Name: "key revoke", Arguments: "<key_id>", Summary: "Stop a key from reaching its fleet", Run: fleetkey.Revoke,
				Details: "Asks first. Anything still using the key can no longer reach the fleet."},
		},
	},
	{
		Title: "Account",
		Commands: []dispatch.Command{
			{Name: "account balance", Arguments: "[fleet_id] [--json]", Summary: "Show the credit left on your fleets", Run: account.Balance},
			{Name: "account topup", Arguments: "<fleet_id>", Summary: "Add credit to a fleet", Run: account.TopUp,
				Details: "Opens a page in your browser to choose an amount and pay. The credit\n" +
					"shows in account balance once the payment completes."},
			{Name: "account delete", Summary: "Delete your account entirely", Run: account.Delete,
				Details: "Asks first, and cannot be undone. Hand over or delete every fleet you\n" +
					"own before deleting your account."},
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
