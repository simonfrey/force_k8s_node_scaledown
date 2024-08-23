package main

import (
	"context"
	"fmt"
	"github.com/alecthomas/kong"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var cli struct {
	IgnoreNamespaces []string      `kong:"default='gke-managed-cim,gmp-system,kube-system',env='IGNORE_NAMESPACES',help='If a pod is running in any of these namespaces, the pod will be ignored and the node deleted'"`
	MinNodeAge       time.Duration `kong:"default='5m',required,env='MIN_NODE_AGE',help='If nodes are younger than this, they will be ignored and not deleted'"`
	Sleep            time.Duration `kong:"default='20s',required,env='SLEEP',help='Time to sleep between checks'"`
	AllowedIdleTime  time.Duration `kong:"default='3m',required,env='ALLOWED_IDLE_TIME',help='Time a node is allowed to be idle before it is deleted'"`
	HealthcheckPort  string        `kong:"default='9200',env='HEALTHCHECK_PORT',help='Port to listen on for health checks'"`
}

func main() {
	kong.Parse(&cli)

	// Create a map of namespaces to ignore
	ignoreNamespaceMap := make(map[string]bool)
	for _, namespace := range cli.IgnoreNamespaces {
		ignoreNamespaceMap[namespace] = true
	}

	//
	// Setup Kubernetes API client

	// Load the kubeconfig file to configure access to the cluster
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}
	// Create a clientset to interact with the Kubernetes API
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes clientset: %v", err)
	}

	//
	// Kubernetes health endpoint server
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
		_ = http.ListenAndServe(":"+cli.HealthcheckPort, nil)
	}()

	//
	// Trap kubernetes shutdown signal
	run := atomic.Bool{}
	run.Store(true)
	go func() {
		sigint := make(chan os.Signal, 1)

		// interrupt signal sent from terminal
		signal.Notify(sigint, os.Interrupt)
		// sigterm signal sent from kubernetes
		signal.Notify(sigint, syscall.SIGTERM)

		<-sigint

		// Shutdown loop
		run.Store(false)
	}()

	idleNodes := make(map[string]time.Time)
	for run.Load() {
		// Get a list of all nodes in the cluster
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatalf("Error listing nodes: %v", err)
		}

		fmt.Printf("\n\n----------\n%s\nListing Nodes and blocking Pods Running on them:\n", time.Now())

		// Iterate over the nodes
		for _, node := range nodes.Items {
			fmt.Printf("\nNode Name: %s\n", node.Name)
			nodeAge := time.Since(node.CreationTimestamp.Time)
			if nodeAge < cli.MinNodeAge {
				fmt.Printf("  Node is less than %v old. Skipping\n", cli.MinNodeAge)
				continue
			}

			// Get a list of all pods in all namespaces
			pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				log.Fatalf("Error listing pods: %v", err)
			}

			// Iterate over the pods to find which are running on the current node
			runningPodCount := 0
			for _, pod := range pods.Items {
				if ignoreNamespaceMap[strings.TrimSpace(pod.Namespace)] {
					// Pod is from namespace that should be ignored. (e.g. kube-system)
					continue
				}
				if pod.Spec.NodeName != node.Name {
					// Pod is on wrong node (or better: The API reports it wrong)
					continue
				}
				if pod.Status.Phase != "Running" {
					// Pod is not runnig, hence does not count towards a "required" node
					continue
				}
				runningPodCount++
				fmt.Printf("  Pod Name: %s, Namespace: %s. Phase: %s\n", pod.Name, pod.Namespace, pod.Status.Phase)
			}

			if runningPodCount > 0 {
				// Remove from idle list
				delete(idleNodes, node.Name)
				fmt.Printf("  Total Pods Running on Node: %d. Node is still required\n", runningPodCount)
				continue
			}

			// No pods are running on the node. Check if the node is idle
			lastActiveTime, ok := idleNodes[node.Name]
			if !ok {
				// Node is not in the idle list. Add it
				idleNodes[node.Name] = time.Now()
				fmt.Printf("  Node is in allowed-idle state. Added to idle list\n")
				continue
			}
			if time.Now().Add(-cli.AllowedIdleTime).Before(lastActiveTime) {
				fmt.Printf("  Node is in allowed-idle state. Last active %s ago.\n", time.Now().Sub(lastActiveTime))
				continue
			}

			fmt.Printf("  No Pods Running on Node. Node is not required anymore. Allowed-idle time is up.\n  Remove it.\n")
			// Remove node
			err = clientset.CoreV1().Nodes().Delete(context.TODO(), node.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Fatalf("Error deleting node: %v", err)
			}
			fmt.Printf("  Node %s removed\n", node.Name)
		}

		time.Sleep(cli.Sleep)
	}
}
