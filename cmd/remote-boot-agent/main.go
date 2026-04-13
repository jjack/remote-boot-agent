package main

import (
"bytes"
"encoding/json"
"fmt"
"log"
"net/http"
"os"
"strings"

"github.com/jjack/remote-boot-agent/internal/bootloader"
"github.com/jjack/remote-boot-agent/internal/bootloader/grub"
"github.com/jjack/remote-boot-agent/internal/config"
"github.com/jjack/remote-boot-agent/internal/initsystem"
"github.com/jjack/remote-boot-agent/internal/initsystem/systemd"
"github.com/spf13/cobra"
)

func setDefaults(cfg *config.Config, blReg *bootloader.Registry, initReg *initsystem.Registry) {
if cfg.Host.Bootloader == "" {
cfg.Host.Bootloader = blReg.Detect()
}
if cfg.Host.InitSystem == "" {
cfg.Host.InitSystem = initReg.Detect()
}
}

func buildCommands(blReg *bootloader.Registry, initReg *initsystem.Registry) *cobra.Command {
var rootCmd = &cobra.Command{
Use:   "remote-boot-agent",
Short: "remote-boot-agent reads boot configurations and posts them to Home Assistant",
}
config.InitFlags(rootCmd.PersistentFlags())

var getSelectedOSCmd = &cobra.Command{
Use:   "get-selected-os",
Short: "Output the currently selected OS from Home Assistant",
Run: func(cmd *cobra.Command, args []string) {
cfg, _ := config.Load(cmd.Flags())
setDefaults(cfg, blReg, initReg)

endpoint := fmt.Sprintf("%s/api/remote_boot_manager/%s", strings.TrimRight(cfg.HomeAssistant.BaseURL, "/"), cfg.Host.MACAddress)
resp, err := http.Get(endpoint)
if err != nil {
log.Fatalf("Error communicating with Home Assistant: %v", err)
}
defer resp.Body.Close()

if resp.StatusCode >= 200 && resp.StatusCode < 300 {
var buf bytes.Buffer
buf.ReadFrom(resp.Body)
fmt.Printf("%s\n", buf.String())
}
},
}

var getAvailableOSesCmd = &cobra.Command{
Use:   "get-available-oses",
Short: "Output the list of available OSes from the bootloader",
Run: func(cmd *cobra.Command, args []string) {
cfg, _ := config.Load(cmd.Flags())
setDefaults(cfg, blReg, initReg)

bl, _ := blReg.Get(cfg.Host.Bootloader)
opts, _ := bl.Parse(cfg)

for _, osName := range opts.AvailableOSes {
fmt.Printf("%s\n", osName)
}
},
}

var pushAvailableOSesCmd = &cobra.Command{
Use:   "push-available-oses",
Short: "Push the list of available OSes to Home Assistant",
Run: func(cmd *cobra.Command, args []string) {
cfg, _ := config.Load(cmd.Flags())
setDefaults(cfg, blReg, initReg)

bl, _ := blReg.Get(cfg.Host.Bootloader)
opts, _ := bl.Parse(cfg)

webhookURL := fmt.Sprintf("%s/api/webhook/remote_boot_manager_ingest", strings.TrimRight(cfg.HomeAssistant.BaseURL, "/"))

type HAPayload struct {
MACAddress string   `json:"mac_address"`
Hostname   string   `json:"hostname"`
Bootloader string   `json:"bootloader"`
OSList     []string `json:"os_list"`
}
payload := HAPayload{
MACAddress: cfg.Host.MACAddress,
Hostname:   cfg.Host.Hostname,
Bootloader: cfg.Host.Bootloader,
OSList:     opts.AvailableOSes,
}

jsonData, _ := json.Marshal(payload)
http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
},
}

rootCmd.AddCommand(getSelectedOSCmd)
rootCmd.AddCommand(getAvailableOSesCmd)
rootCmd.AddCommand(pushAvailableOSesCmd)

return rootCmd
}

func main() {
blReg := bootloader.NewRegistry(grub.New())
initReg := initsystem.NewRegistry(systemd.New())

rootCmd := buildCommands(blReg, initReg)

if err := rootCmd.Execute(); err != nil {
fmt.Println(err)
os.Exit(1)
}
}
