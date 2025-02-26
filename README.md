# Pod Watcher

A simple Go CLI tool for watching Kubernetes pods across all namespaces, filtering for a specified marker string in their YAML definition, and logging each change (Added/Modified/Deleted) in a YAML stream. Built using the Cobra CLI framework and client-go.
Features

* Watches all pods in all namespaces via the Kubernetes API.
* Filters pods by a marker substring anywhere in their YAML serialization.
* Outputs each revision of matching pods as a separate YAML document (separated by ---).
* Supports two modes:
  * Continuous mode (default): Run indefinitely, logging all events for all matching pods.
  * Stop-on-delete mode: Once it finds the first matching pod, it watches only that pod, and exits after the pod is deleted.
* Automatically recovers from watch interruptions (e.g., ResourceVersionTooOld) by re-listing and re-watching.
* Respects cancellation (e.g., Ctrl+C) for a graceful shutdown.

# Prerequisites

Go 1.18+ (or a version compatible with the Kubernetes client library you’re using).
Access to a Kubernetes cluster (either via a valid kubeconfig file or in-cluster credentials).

# Installation

1. Clone this repository:
    ```
    git clone https://github.com/stephenc/pod-watcher.git
    cd pod-watcher
    ```

2. Build the binary:
    ```
    go build -o pod-watcher main.go
    ```

3. (Optional) Move the compiled binary to your $PATH:
    ```
    sudo mv pod-watcher /usr/local/bin/
    ```

Now you can run pod-watcher from the cloned directory or from $PATH.


# Usage

```
Usage:
pod-watcher [flags]

Flags:
-h, --help               help for pod-watcher
-m, --marker string      Marker substring to filter pods (required)
-s, --stop-on-delete     Stop after the first matching pod is deleted
--kubeconfig string  Path to kubeconfig file (defaults to in-cluster or standard config)
```

# Examples
1.  Continuous Mode

    Watch all pods in all namespaces, looking for the substring "DEBUG_MODE" anywhere in the pod’s YAML. Logs all events (Add/Update/Delete) for any matching pod(s). Will run until you interrupt it (Ctrl+C) or the watch connection fails permanently.
  
    ```
    pod-watcher --marker "DEBUG_MODE"
    ```

2.  Stop-on-Delete Mode

    Watch for a pod containing "MARKER_STRING" and, once found, focus only on that single pod. When that pod is finally deleted, the watcher will exit.
 
    ```   
    pod-watcher --marker "MARKER_STRING" --stop-on-delete
    ```
    
# Output Format

Every time a matching pod is created, updated, or deleted, the tool outputs a YAML document to stdout. Each document is prefixed with ---, making it easy to separate and process revisions:

```yaml
---
apiVersion: v1
kind: Pod
metadata:
name: example-pod
namespace: default
# ...
spec:
# ...
status:
# ...
```

When using continuous mode, if multiple pods match the marker, their YAML revisions will interleave in the order the watcher receives events.
Kubernetes Configuration

* Default behavior: The tool attempts to use the typical client-go lookup flow for a kubeconfig (checks the KUBECONFIG environment variable, then ~/.kube/config, etc.). If that fails, it attempts to use in-cluster credentials (suitable when running inside Kubernetes).

* Explicit kubeconfig: Use the --kubeconfig flag to specify a particular config file:

```
pod-watcher --marker "DEBUG_MODE" --kubeconfig /path/to/kubeconfig
```

# Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request for bug fixes, improvements, or additional features.

1. Fork the repository and clone your fork.
2. Create a new branch for your change: `git checkout -b my-feature`.
3. Make changes, add tests (if applicable).
4. Run tests with `go test ./...`.
5. Commit your changes and push them to your fork.
6. Open a pull request on this repository.

# License

This project is licensed under the MIT License. See the LICENSE file for details.