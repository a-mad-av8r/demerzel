package accountscli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const defaultControlURL = "http://127.0.0.1:3001"

var errHelpRequested = errors.New("help requested")

type commonFlags struct {
	controlURL *string
	dataDir    *string
	authFile   *string
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if err := run(args, stdin, stdout); err != nil {
		if errors.Is(err, errHelpRequested) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "demerzel accounts: %s\n", err)
		return 1
	}
	return 0
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printAccountsHelp(stdout)
		return nil
	}

	command := args[0]
	commandArgs := args[1:]
	if command == "label" {
		return runLabel(commandArgs, stdout)
	}

	flags, common := newCommonFlagSet(command)
	switch command {
	case "list":
		return runList(commandArgs, flags, common, stdout)
	case "add":
		return runAdd(commandArgs, flags, common, stdin, stdout)
	case "disable", "restore", "remove":
		return runMutation(command, commandArgs, flags, common, stdout)
	default:
		return errors.New("unknown accounts command; run 'demerzel accounts help'")
	}
}

func newCommonFlagSet(name string) (*flag.FlagSet, commonFlags) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	urlDefault := strings.TrimSpace(os.Getenv("DEMERZEL_CONTROL_URL"))
	if urlDefault == "" {
		urlDefault = defaultControlURL
	}
	dataDirDefault := os.Getenv("DATA_DIR")
	return flags, commonFlags{
		controlURL: flags.String("url", urlDefault, "loopback control API URL"),
		dataDir:    flags.String("data-dir", dataDirDefault, "durable application data directory"),
		authFile:   flags.String("auth-key-file", "", "owner-only management key file"),
	}
}

func parseFlags(flags *flag.FlagSet, args []string, help io.Writer, usage string) error {
	flags.Usage = func() { _, _ = io.WriteString(help, usage) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.Usage()
			return errHelpRequested
		}
		return errors.New("invalid command options")
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional argument")
	}
	return nil
}

func commonClient(flags commonFlags) (*apiClient, error) {
	baseURL, err := validateControlURL(*flags.controlURL)
	if err != nil {
		return nil, err
	}
	authKey, err := loadAuthKey(*flags.dataDir, *flags.authFile)
	if err != nil {
		return nil, err
	}
	return newAPIClient(baseURL, authKey), nil
}

func runList(args []string, flags *flag.FlagSet, common commonFlags, stdout io.Writer) error {
	groupValue := flags.Uint64("group", 0, "limit output to one group ID")
	if err := parseFlags(flags, args, stdout, listHelp); err != nil {
		return err
	}
	groupSet := false
	flags.Visit(func(value *flag.Flag) {
		if value.Name == "group" {
			groupSet = true
		}
	})
	if groupSet {
		if _, err := parseRequiredID(*groupValue, "group"); err != nil {
			return err
		}
	}
	client, err := commonClient(common)
	if err != nil {
		return err
	}
	groups, err := client.listGroups()
	if err != nil {
		return err
	}
	if *groupValue > 0 {
		found := false
		for _, group := range groups {
			if uint64(group.ID) == *groupValue {
				found = true
				break
			}
		}
		if !found {
			return errors.New("group ID was not found")
		}
	}

	accounts := make([]listedAccount, 0)
	for _, group := range groups {
		if *groupValue > 0 && uint64(group.ID) != *groupValue {
			continue
		}
		items, err := client.listGroupCredentials(group.ID)
		if err != nil {
			return err
		}
		for _, item := range items {
			accounts = append(accounts, listedAccount{Group: group, Credential: item})
		}
	}
	return writeAccountTable(stdout, accounts)
}

func runAdd(args []string, flags *flag.FlagSet, common commonFlags, stdin io.Reader, stdout io.Writer) error {
	groupValue := flags.Uint64("group", 0, "target group ID")
	useStdin := flags.Bool("stdin", false, "read one API key from stdin")
	keyFile := flags.String("key-file", "", "read one API key from a restrictive file")
	if err := parseFlags(flags, args, stdout, addHelp); err != nil {
		return err
	}
	groupID, err := parseRequiredID(*groupValue, "group")
	if err != nil {
		return err
	}
	if *useStdin == (*keyFile != "") {
		return errors.New("choose exactly one of --stdin or --key-file")
	}
	var secret []byte
	if *useStdin {
		secret, err = readSingleCredential(stdin)
	} else {
		secret, err = readCredentialFile(*keyFile)
	}
	if err != nil {
		return err
	}
	defer clear(secret)

	client, err := commonClient(common)
	if err != nil {
		return err
	}
	idempotencyKey, err := newIdempotencyKey()
	if err != nil {
		return errors.New("could not create a credential import request")
	}
	result, err := client.importCredential(groupID, string(secret), idempotencyKey)
	if err != nil {
		return err
	}
	if result.CredentialsAdded+result.CredentialsDuplicated != 1 {
		return errors.New("control API returned an unexpected import result")
	}
	if result.CredentialsAdded == 1 {
		_, err = fmt.Fprintf(stdout, "Added one account to group %d.\n", groupID)
	} else {
		_, err = fmt.Fprintf(stdout, "Account already exists in group %d.\n", groupID)
	}
	return err
}

func runLabel(args []string, stdout io.Writer) error {
	flags, common := newCommonFlagSet("label")
	groupValue := flags.Uint64("group", 0, "target group ID")
	credentialValue := flags.Uint64("credential", 0, "credential ID")
	label := flags.String("label", "", "new label; an empty value clears it")
	if err := parseFlags(flags, args, stdout, labelHelp); err != nil {
		return err
	}
	labelSet := false
	flags.Visit(func(value *flag.Flag) {
		if value.Name == "label" {
			labelSet = true
		}
	})
	if !labelSet {
		return errors.New("--label is required; pass an empty value to clear the label")
	}
	groupID, err := parseRequiredID(*groupValue, "group")
	if err != nil {
		return err
	}
	credentialID, err := parseRequiredID(*credentialValue, "credential")
	if err != nil {
		return err
	}
	client, err := commonClient(common)
	if err != nil {
		return err
	}
	updated, err := client.updateLabel(groupID, credentialID, *label)
	if err != nil {
		return err
	}
	if updated.CredentialID != credentialID || updated.Label != *label {
		return errors.New("control API did not confirm the requested label")
	}
	if *label == "" {
		_, err = fmt.Fprintf(stdout, "Cleared label for credential %d in group %d.\n", credentialID, groupID)
	} else {
		_, err = fmt.Fprintf(stdout, "Updated label for credential %d in group %d.\n", credentialID, groupID)
	}
	return err
}

func runMutation(command string, args []string, flags *flag.FlagSet, common commonFlags, stdout io.Writer) error {
	groupValue := flags.Uint64("group", 0, "target group ID")
	credentialValue := flags.Uint64("credential", 0, "credential ID")
	if err := parseFlags(flags, args, stdout, mutationHelp(command)); err != nil {
		return err
	}
	groupID, err := parseRequiredID(*groupValue, "group")
	if err != nil {
		return err
	}
	credentialID, err := parseRequiredID(*credentialValue, "credential")
	if err != nil {
		return err
	}
	client, err := commonClient(common)
	if err != nil {
		return err
	}
	switch command {
	case "disable":
		err = client.setEnabled(groupID, credentialID, false)
	case "restore":
		err = client.setEnabled(groupID, credentialID, true)
	case "remove":
		err = client.removeCredential(groupID, credentialID)
	}
	if err != nil {
		return err
	}
	verb := map[string]string{"disable": "Disabled", "restore": "Restored", "remove": "Removed"}[command]
	_, err = fmt.Fprintf(stdout, "%s credential %d in group %d.\n", verb, credentialID, groupID)
	return err
}

func parseRequiredID(value uint64, name string) (uint, error) {
	if value == 0 {
		return 0, fmt.Errorf("--%s must be a positive ID", name)
	}
	if value > uint64(^uint(0)) {
		return 0, fmt.Errorf("--%s is outside the supported ID range", name)
	}
	return uint(value), nil
}

func printAccountsHelp(output io.Writer) {
	_, _ = io.WriteString(output, accountsHelp)
}

const accountsHelp = `Usage:
  demerzel accounts list [--group ID]
  demerzel accounts add --group ID (--stdin | --key-file PATH)
  demerzel accounts label --group ID --credential ID --label TEXT
  demerzel accounts disable --group ID --credential ID
  demerzel accounts restore --group ID --credential ID
  demerzel accounts remove --group ID --credential ID

Connection options for each command:
  --url URL             Loopback control API (default http://127.0.0.1:3001)
  --auth-key-file PATH  Owner-only AUTH_KEY file (default DATA_DIR/auth.key)
  --data-dir PATH       Durable application data root
  --help                Show command help

API keys are read from stdin or a restrictive file, never from arguments.
`

const listHelp = `Usage: demerzel accounts list [--group ID] [connection options]\n` + accountsConnectionHelp
const addHelp = `Usage: demerzel accounts add --group ID (--stdin | --key-file PATH) [connection options]\n` + accountsConnectionHelp
const labelHelp = `Usage: demerzel accounts label --group ID --credential ID --label TEXT [connection options]\n` + accountsConnectionHelp

func mutationHelp(command string) string {
	return fmt.Sprintf("Usage: demerzel accounts %s --group ID --credential ID [connection options]\n%s", command, accountsConnectionHelp)
}

const accountsConnectionHelp = `\nConnection options:\n  --url URL             Loopback control API\n  --auth-key-file PATH  Owner-only AUTH_KEY file\n  --data-dir PATH       Durable application data root\n`
`})} cfm? We need ensure compile struct flags created issue: common flags added first. For runList, we create group flag after newCommon. Fine. But runLabel does own newCommon. Top-level `run` currently for label before creation. Good. `run` default command picks `newCommonFlagSet(command)` for list/add/mutation. In runAdd etc good. Helpers `parseFlags` command help if no args no error. `flag.ErrHelp` prints usage but parseFlags returns nil; run continues command with default group 0 causing positive missing ID after help. Need add flag help detection before run? parseFlags could return special `errShowedHelp` and caller checks; or use `flagSet.Bool(