package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	marker       string
	stopOnDelete bool
	kubeconfig   string
)

// rootCmd defines the CLI command using Cobra
var rootCmd = &cobra.Command{
	Use:   "pod-watcher",
	Short: "Watch Kubernetes pods and log changes when a marker string is present",
	Long: `pod-watcher monitors all Kubernetes pods across all namespaces, filtering for a specified marker string in the pod's YAML.
It logs every change to any matching pod as a separate YAML document in a stream.

Examples:
  pod-watcher --marker "DEBUG_MODE"
  pod-watcher --marker "DEBUG_MODE" --stop-on-delete
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Execute the watch logic
		if err := runWatcher(cmd.Context()); err != nil {
			log.Fatalf("Error: %v", err)
		}
	},
}

func init() {
	// Define CLI flags
	rootCmd.Flags().StringVarP(&marker, "marker", "m", "", "Marker substring to filter pods (required)")
	rootCmd.Flags().BoolVarP(&stopOnDelete, "stop-on-delete", "s", false, "Stop after first matching pod is deleted")
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (defaults to in-cluster or default config)")
	// Mark required flags
	_ = rootCmd.MarkFlagRequired("marker")
}

func main() {
	// Set up context that cancels on SIGINT/SIGTERM for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// Run the Cobra command
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}
}

// runWatcher connects to Kubernetes and starts watching pods for the marker.
func runWatcher(ctx context.Context) error {
	// Build Kubernetes REST client configuration
	config, err := buildConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("could not load Kubernetes config: %w", err)
	}
	// Create a Kubernetes clientset from the config
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("could not create Kubernetes client: %w", err)
	}
	log.Printf("Starting pod watcher (marker=%q, stopOnDelete=%v)", marker, stopOnDelete)

	// Variables for stop-on-delete mode
	var targetPodKey string // "namespace/name" of the first matching pod
	targetAcquired := false // whether we've locked onto a specific pod
	done := false           // signals when to terminate the watch loop

	// Outer loop: keep watching until done or error requiring restart
	for !done {
		// 1. List pods to get current resourceVersion&#8203;:contentReference[oaicite:9]{index=9}
		list, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("Initial pod list error: %v. Retrying...", err)
			time.Sleep(2 * time.Second)
			continue // retry listing until successful
		}
		resourceVersion := list.ResourceVersion

		// 2. Start watching from the obtained resourceVersion for new changes
		watcher, err := clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
			ResourceVersion: resourceVersion,
		})
		if err != nil {
			log.Printf("Watch start failed (resourceVersion=%s): %v. Retrying...", resourceVersion, err)
			time.Sleep(2 * time.Second)
			continue // retry starting the watch
		}

		// Inner loop: process events from the watch
		for event := range watcher.ResultChan() {
			// Exit if context was canceled (e.g., Ctrl+C)
			if ctx.Err() != nil {
				log.Println("Context canceled, stopping watcher.")
				done = true
				break
			}
			if event.Type == watch.Error {
				// An error occurred in the watch stream (e.g., too old resourceVersion)
				// Log details and break to restart the watch&#8203;:contentReference[oaicite:10]{index=10}
				if status, ok := event.Object.(*metav1.Status); ok {
					log.Printf("Watch error: %s (code %d)", status.Message, status.Code)
				} else {
					log.Printf("Watch error: received unknown error object")
				}
				break // break inner loop to re-establish watch
			}

			// Convert to a Pod or skip
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				// If it's not a Pod, it might be a *metav1.Status
				// or something else. Usually we skip it.
				continue
			}

			// Serialize Pod to YAML
			podYAML, err := yaml.Marshal(pod)
			if err != nil {
				log.Printf("Failed to marshal pod %s/%s to YAML: %v", pod.Namespace, pod.Name, err)
				continue
			}
			yamlStr := string(podYAML)
			// Check for marker substring
			if !strings.Contains(yamlStr, marker) {
				continue // ignore events that don't include the marker
			}

			// If stopOnDelete mode, select the first matching pod as target
			currentKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
			if stopOnDelete {
				if !targetAcquired {
					targetPodKey = currentKey
					targetAcquired = true
					log.Printf("Target pod found: %s (monitoring exclusively)", targetPodKey)
				}
				// Once a target is acquired, ignore other pods
				if currentKey != targetPodKey {
					continue
				}
			}

			// Output the pod's YAML as one document in the stream
			fmt.Printf("---\n%s\n", yamlStr)

			// If this was a deletion of the target pod (stop-on-delete mode), we can finish
			if stopOnDelete && targetAcquired && event.Type == watch.Deleted && currentKey == targetPodKey {
				log.Printf("Target pod %s deleted, exiting watcher.", targetPodKey)
				done = true
				break
			}
		} // end inner for events

		// Clean up watcher resources
		watcher.Stop()
		if done {
			break // exit outer loop if done flag is set
		}
		// Otherwise, loop continues to restart the watch after a short pause
		log.Println("Watch stream ended, restarting watch...")
		time.Sleep(1 * time.Second)
	}
	return nil
}

// buildConfig creates a Kubernetes client config from a file path or in-cluster settings
func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		// Use the provided kubeconfig file
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	// No kubeconfig specified: try default external config, then in-cluster
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, nil)
	restConfig, err := config.ClientConfig()
	if err != nil {
		// If not found in default locations, try in-cluster config
		return rest.InClusterConfig()
	}
	return restConfig, nil
}
