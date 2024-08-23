package main

import (
	"context"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ignoreNamespaces := []string{"gke-managed-cim", "gmp-system", "kube-system"}
	minNodeAge := 5 * time.Minute
	sleep := 20 * time.Second

	ignoreNamespaceMap := make(map[string]bool)
	for _, namespace := range ignoreNamespaces {
		ignoreNamespaceMap[namespace] = true
	}

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

	for {
		// Get a list of all nodes in the cluster
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatalf("Error listing nodes: %v", err)
		}

		fmt.Println("### Listing Nodes and Pods Running on Them:")

		// Iterate over the nodes
		for _, node := range nodes.Items {
			fmt.Printf("\nNode Name: %s\n", node.Name)
			nodeAge := time.Since(node.CreationTimestamp.Time)
			if nodeAge < minNodeAge {
				fmt.Printf("  Node is less than %v old. Skipping\n", minNodeAge)
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
				if ignoreNamespaceMap[pod.Namespace] {
					continue
				}
				if pod.Spec.NodeName != node.Name {
					continue
				}
				if pod.Status.Phase != "Running" {
					continue
				}
				runningPodCount++
				fmt.Printf("  Pod Name: %s, Namespace: %s. Phase: %s\n", pod.Name, pod.Namespace, pod.Status.Phase)
			}

			if runningPodCount > 0 {
				fmt.Printf("  Total Pods Running on Node: %d\n", runningPodCount)
				continue
			}
			fmt.Printf("  No Pods Running on Node\n")
			fmt.Printf("Removing Node: %s\n", node.Name)
			// Drain and remove node
			err = clientset.CoreV1().Nodes().Delete(context.TODO(), node.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Fatalf("Error deleting node: %v", err)
			}
		}

		time.Sleep(sleep)
	}
}
