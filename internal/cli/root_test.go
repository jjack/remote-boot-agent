package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestNewCLI(t *testing.T) {
	cli := NewCLI()
	if cli == nil {
		t.Fatal("expected pointer to CLI, got nil")
	}
	if cli.RootCmd == nil {
		t.Fatal("expected RootCmd to be initialized")
	}
	if cli.RootCmd.Use != "grubstation" {
		t.Errorf("expected use 'grubstation', got %s", cli.RootCmd.Use)
	}
}

func TestCLI_Execute(t *testing.T) {
	cli := NewCLI()

	cli.RootCmd.SetArgs([]string{"help"})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	err := cli.Execute()
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
}

func TestCLI_PersistentPreRun_ConfigParseFail(t *testing.T) {
	f, err := os.CreateTemp("", "bad-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, _ = f.Write([]byte("invalid\n yaml\n  content"))
	_ = f.Close()

	cli := NewCLI()
	cli.RootCmd.SetArgs([]string{"options", "list", "--config", f.Name()})
	err = cli.Execute()
	if err == nil {
		t.Fatal("expected error on malformed config file")
	}
}

func TestCLI_PersistentPreRun_ConfigValidateFail(t *testing.T) {
	f, err := os.CreateTemp("", "empty-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, _ = f.Write([]byte("{}")) // Empty config will parse successfully, but fail domain validation
	_ = f.Close()

	cli := NewCLI()
	cli.RootCmd.SetArgs([]string{"options", "list", "--config", f.Name()})
	err = cli.Execute()
	if err == nil {
		t.Fatal("expected error on invalid config file")
	}
}

func TestCLI_PersistentPreRun_Setup(t *testing.T) {
	cli := NewCLI()

	cmd, _, err := cli.RootCmd.Find([]string{"setup"})
	if err != nil {
		t.Fatal(err)
	}

	if cmd.PersistentPreRunE == nil {
		t.Fatal("expected setup command to override PersistentPreRunE")
	}

	err = cmd.PersistentPreRunE(cmd, []string{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestBootCmd(t *testing.T) {
	deps := &CommandDeps{}
	cmd := NewBootCmd(deps)
	if cmd.Use != "boot" {
		t.Errorf("expected Use 'boot', got %q", cmd.Use)
	}
}
