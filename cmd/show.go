package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/BetterCallB4RB4/heimdall/pkg/providers/azure"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current cloud and Kubernetes context",
	Long:  "Show the AWS profile, Azure subscription, kubeconfig, and Kubernetes context active in the current shell.",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Box("Current Heimdall Context", currentContextFields())
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func currentContextFields() [][2]string {
	azureSubscription := "not selected"
	if os.Getenv("AZURE_CONFIG_DIR") != "" {
		subscription, err := azure.GetActiveSubscription()
		if err != nil {
			azureSubscription = "unavailable"
		} else {
			azureSubscription = fmt.Sprintf("%s (%s)", subscription.Name, subscription.ID)
		}
	}

	kubeconfig := valueOrStatus(os.Getenv("KUBECONFIG"), "not selected")
	kubernetesContext := "not selected"
	if os.Getenv("KUBECONFIG") != "" {
		kubernetesContext = currentKubernetesContext()
	}

	return [][2]string{
		{"AWS profile", valueOrStatus(os.Getenv("AWS_PROFILE"), "not selected")},
		{"Azure config dir", valueOrStatus(os.Getenv("AZURE_CONFIG_DIR"), "not selected")},
		{"Azure subscription", azureSubscription},
		{"Kubeconfig", kubeconfig},
		{"Kubernetes context", kubernetesContext},
	}
}

func currentKubernetesContext() string {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	if err != nil {
		return "unavailable"
	}
	if context := strings.TrimSpace(string(out)); context != "" {
		return context
	}
	return "unavailable"
}

func valueOrStatus(value, status string) string {
	if value == "" {
		return status
	}
	return value
}
